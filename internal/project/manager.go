// Package project implements the ProjectManager: thread-safe cache of
// loaded projects, with per-entry expiration.
//
// Counterpart of `project_manager.rs` (rust-mcp-server),
// `com.softtek.mcp.ProjectManager` (java) and `CobolFileManager` (cobol).
// Same idea: RWMutex + map[name]ProjectEntry with lazy sweep on each access.
package project

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"go-mcp-server/internal/analysis"
	"go-mcp-server/internal/model"
	"go-mcp-server/internal/sem"
)

// DefaultExpiry is the default expiration: 10 minutes (same as
// rust/java/cobol/python).
const DefaultExpiry = 10 * time.Minute

// Manager is thread-safe. The sweep of expired entries is lazy:
// applied on each write/read operation (no goroutine).
type Manager struct {
	mu      sync.RWMutex
	entries map[string]*model.ProjectEntry
	loader  *Loader
	// analysisCache stores the *analysis.Project by EffectiveName.
	// Lives and dies with the entry (same lazy sweep).
	analysisCache map[string]*analysis.Project
	// semCache stores the *sem.Project (go/packages + go/types) by
	// EffectiveName. Lazy loading — only populated when a semantic
	// tool (iter 3) requests it.
	semCache map[string]*sem.Project
}

// NewManager creates an empty Manager.
func NewManager() *Manager {
	return &Manager{
		entries:       make(map[string]*model.ProjectEntry),
		loader:        &Loader{},
		analysisCache: make(map[string]*analysis.Project),
		semCache:      make(map[string]*sem.Project),
	}
}

// Load resolves the source (local path / Git URL / archive URL), parses the
// project (go.mod + ASTs) and registers the entry. Returns the created
// entry or a *model.LoadError with stable code.
func (m *Manager) Load(source, alias string, expiry time.Duration) (*model.ProjectEntry, error) {
	resolved, err := m.loader.Resolve(source)
	if err != nil {
		return nil, err
	}

	proj, err := analysis.LoadProject(resolved.Path)
	if err != nil {
		if resolved.TempDir {
			os.RemoveAll(resolved.Path)
		}
		return nil, err
	}

	if expiry <= 0 {
		expiry = DefaultExpiry
	}
	now := time.Now()
	entry := &model.ProjectEntry{
		Name:       deriveName(resolved.Path, proj.ModulePath),
		Alias:      alias,
		Path:       resolved.Path,
		Kind:       model.SingleModule,
		ExpiresAt:  now.Add(expiry),
		LoadedAt:   now,
		ModulePath: proj.ModulePath,
		GoVersion:  proj.GoVersion,
		TempDir:    resolved.TempDir,
	}
	if proj.Work != nil {
		entry.Kind = model.Workspace
		for _, u := range proj.Work.Use {
			entry.Modules = append(entry.Modules, strings.TrimPrefix(u.Path, "./"))
		}
		sort.Strings(entry.Modules)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.sweepLocked(now)
	m.entries[entry.EffectiveName()] = entry
	m.analysisCache[entry.EffectiveName()] = proj
	return entry, nil
}

// Analysis returns the cached *analysis.Project for an entry.
// In iter 2 every project in the Manager came from Load (hot cache);
// entries added with Add (tests) are parsed here on demand.
func (m *Manager) Analysis(e *model.ProjectEntry) (*analysis.Project, error) {
	key := e.EffectiveName()

	m.mu.RLock()
	cached, ok := m.analysisCache[key]
	m.mu.RUnlock()
	if ok {
		return cached, nil
	}

	proj, err := analysis.LoadProject(e.Path)
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.analysisCache[key] = proj
	return proj, nil
}

// SemanticAnalysis returns the *sem.Project (go/packages + go/types)
// for an entry. It is EXPENSIVE cold (type-checks the entire dep graph) —
// that is why it is cached here and only used by semantic tools (iter 3).
// The result is NOT parsed in Load (like analysisCache); loading is
// lazy: first call → Load, subsequent → cache.
func (m *Manager) SemanticAnalysis(e *model.ProjectEntry) (*sem.Project, error) {
	key := e.EffectiveName()

	m.mu.RLock()
	cached, ok := m.semCache[key]
	m.mu.RUnlock()
	if ok {
		return cached, nil
	}

	proj, err := sem.Load(e.Path)
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.semCache[key] = proj
	return proj, nil
}

// Add inserts or replaces an entry, indexed by its EffectiveName.
// Sweeps expired entries before inserting.
//
// In iter 1 entry construction is the caller's responsibility
// (tests, or the future ProjectLoader from iter 2 with
// path/Git/archive resolution).
func (m *Manager) Add(entry *model.ProjectEntry) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sweepLocked(time.Now())
	m.entries[entry.EffectiveName()] = entry
}

// Find searches by EffectiveName (key), alias or real name.
// Returns nil if it does not exist or has expired.
//
// Note: in rust-mcp-server the fallback only checks the alias (redundant with
// the key, which IS the EffectiveName); here the fallback checks the real name,
// which is what a user expects from "name or alias".
func (m *Manager) Find(nameOrAlias string) *model.ProjectEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sweepLocked(time.Now())
	if e, ok := m.entries[nameOrAlias]; ok {
		return e
	}
	for _, e := range m.entries {
		if e.Name == nameOrAlias {
			return e
		}
	}
	return nil
}

// Remove removes by EffectiveName, alias or real name.
// If the entry was a temp dir (clone/archive), cleans it from disk.
// Returns the removed entry or nil.
func (m *Manager) Remove(nameOrAlias string) *model.ProjectEntry {
	m.mu.Lock()
	e, key := m.removeLocked(nameOrAlias)
	m.mu.Unlock()
	if e != nil && e.TempDir {
		os.RemoveAll(e.Path)
	}
	_ = key
	return e
}

// removeLocked performs the deletion with the lock already held.
func (m *Manager) removeLocked(nameOrAlias string) (*model.ProjectEntry, string) {
	if e, ok := m.entries[nameOrAlias]; ok {
		delete(m.entries, nameOrAlias)
		delete(m.analysisCache, nameOrAlias)
		delete(m.semCache, nameOrAlias)
		return e, nameOrAlias
	}
	for k, e := range m.entries {
		if e.Name == nameOrAlias {
			delete(m.entries, k)
			delete(m.analysisCache, k)
			delete(m.semCache, k)
			return e, k
		}
	}
	return nil, ""
}

// List returns live entries sorted by EffectiveName
// (output determinism, same as rust-mcp-server).
func (m *Manager) List() []*model.ProjectEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sweepLocked(time.Now())
	out := make([]*model.ProjectEntry, 0, len(m.entries))
	for _, e := range m.entries {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].EffectiveName() < out[j].EffectiveName()
	})
	return out
}

// Count returns the number of live entries.
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	now := time.Now()
	n := 0
	for _, e := range m.entries {
		if !e.IsExpired(now) {
			n++
		}
	}
	return n
}

// sweepLocked removes expired entries (and their analysis cache).
// The caller must hold the write lock.
func (m *Manager) sweepLocked(now time.Time) {
	for k, e := range m.entries {
		if e.IsExpired(now) {
			delete(m.entries, k)
			delete(m.analysisCache, k)
			delete(m.semCache, k)
		}
	}
}

// deriveName: last segment of the module path if it exists, otherwise of path.
// (For "example.com/samplemodule" → "samplemodule".)
func deriveName(path, modulePath string) string {
	if modulePath != "" {
		if i := strings.LastIndex(modulePath, "/"); i >= 0 && i < len(modulePath)-1 {
			return modulePath[i+1:]
		}
		if modulePath != "" {
			return modulePath
		}
	}
	base := filepath.Base(path)
	if base == "." || base == string(filepath.Separator) {
		return "unnamed"
	}
	return base
}
