// Package model contains the pure domain types: no logic, no I/O.
//
// Counterpart of `model/project_entry.rs` (rust-mcp-server) and
// `com.softtek.mcp.model.ProjectEntry` (java-mcp-server).
package model

import "time"

// ProjectKind clasifica el proyecto cargado.
type ProjectKind int

const (
	// SingleModule is a single Go module (one go.mod, no go.work).
	SingleModule ProjectKind = iota
	// Workspace is a Go workspace (presence of go.work).
	Workspace
)

// String returns the stable label used in tool Markdown.
// DO NOT change without regenerating golden snapshots.
func (k ProjectKind) String() string {
	if k == Workspace {
		return "workspace"
	}
	return "module"
}

// ProjectEntry is the record of a Go project loaded in memory.
//
// Analogous to `ProjectEntry` (rust) / `ProjectEntry` (java). In iter 2
// it will carry the parsed AST (go/ast) when available; in iter 1
// only module metadata.
type ProjectEntry struct {
	// Name is the name derived from the directory (last segment of the path).
	Name string

	// Alias is the optional alias ("" = no alias).
	Alias string

	// Path is the path to the root directory of the module/workspace.
	Path string

	// Kind classifies the project.
	Kind ProjectKind

	// ExpiresAt indicates when this entry expires (10 min by default).
	ExpiresAt time.Time

	// LoadedAt indicates when it was loaded.
	LoadedAt time.Time

	// Modules are the member modules (if workspace, `use` directives
	// from go.work). Empty for single module.
	Modules []string

	// ModulePath is the module path declared in go.mod
	// (`module` directive, e.g. "example.com/foo").
	ModulePath string

	// GoVersion is the Go version declared in go.mod (`go` directive).
	GoVersion string

	// TempDir indicates Path lives in a temporary directory created by the
	// loader (Git clones, extracted archives). Manager.Remove cleans it up.
	TempDir bool
}

// EffectiveName returns the alias if it exists, otherwise the derived name.
func (e *ProjectEntry) EffectiveName() string {
	if e.Alias != "" {
		return e.Alias
	}
	return e.Name
}

// IsExpired reports whether the entry has expired at `now`.
func (e *ProjectEntry) IsExpired(now time.Time) bool {
	return !now.Before(e.ExpiresAt)
}
