package project

import (
	"errors"
	"testing"
	"time"

	"go-mcp-server/internal/model"
)

func TestLoadLocalFixture(t *testing.T) {
	mgr := NewManager()
	entry, err := mgr.Load("testdata/tinymod", "tiny", time.Minute)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if entry.Name != "tinymod" {
		t.Errorf("Name = %q, want %q (from module path)", entry.Name, "tinymod")
	}
	if entry.EffectiveName() != "tiny" {
		t.Errorf("EffectiveName = %q, want %q", entry.EffectiveName(), "tiny")
	}
	if entry.Kind != model.SingleModule {
		t.Errorf("Kind = %v, want SingleModule", entry.Kind)
	}
	if entry.ModulePath != "example.com/tinymod" {
		t.Errorf("ModulePath = %q, want %q", entry.ModulePath, "example.com/tinymod")
	}
	if entry.GoVersion != "1.21" {
		t.Errorf("GoVersion = %q, want %q", entry.GoVersion, "1.21")
	}

	// Analysis is cached after Load.
	proj, err := mgr.Analysis(entry)
	if err != nil {
		t.Fatalf("Analysis: %v", err)
	}
	if len(proj.Files) != 1 || proj.Files[0].RelPath != "main.go" {
		t.Errorf("Files = %v, want [main.go]", proj.Files)
	}
}

func TestLoadMissingPath(t *testing.T) {
	mgr := NewManager()
	_, err := mgr.Load("/nonexistent/path/12345", "", time.Minute)
	var le *model.LoadError
	if !errors.As(err, &le) {
		t.Fatalf("err = %v, want *LoadError", err)
	}
	if le.Code != model.ErrSourceNotFound {
		t.Errorf("Code = %q, want %q", le.Code, model.ErrSourceNotFound)
	}
}

func TestLoadMissingGoMod(t *testing.T) {
	mgr := NewManager()
	dir := t.TempDir() // existe pero no tiene go.mod
	_, err := mgr.Load(dir, "", time.Minute)
	var le *model.LoadError
	if !errors.As(err, &le) {
		t.Fatalf("err = %v, want *LoadError", err)
	}
	if le.Code != model.ErrGoModMissing {
		t.Errorf("Code = %q, want %q", le.Code, model.ErrGoModMissing)
	}
}

func TestAnalysisLazyForAddedEntries(t *testing.T) {
	mgr := NewManager()
	// Direct Add (no Load) — analysis is parsed on demand.
	mgr.Add(&model.ProjectEntry{
		Name:      "tinymod",
		Path:      "testdata/tinymod",
		Kind:      model.SingleModule,
		ExpiresAt: time.Now().Add(time.Minute),
	})
	entry := mgr.Find("tinymod")
	proj, err := mgr.Analysis(entry)
	if err != nil {
		t.Fatalf("Analysis lazy: %v", err)
	}
	if proj.ModulePath != "example.com/tinymod" {
		t.Errorf("ModulePath = %q, want %q", proj.ModulePath, "example.com/tinymod")
	}
	// Segunda llamada: cache caliente, mismo puntero.
	proj2, _ := mgr.Analysis(entry)
	if proj != proj2 {
		t.Errorf("Analysis no cachea: %p != %p", proj, proj2)
	}
}
