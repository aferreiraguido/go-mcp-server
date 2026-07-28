package project

import (
	"testing"
	"time"

	"go-mcp-server/internal/model"
)

func entry(name, alias string, kind model.ProjectKind, ttl time.Duration) *model.ProjectEntry {
	now := time.Now()
	return &model.ProjectEntry{
		Name:      name,
		Alias:     alias,
		Path:      "/fixtures/" + name,
		Kind:      kind,
		ExpiresAt: now.Add(ttl),
		LoadedAt:  now,
	}
}

func TestAddThenFind(t *testing.T) {
	mgr := NewManager()
	mgr.Add(entry("sample", "s", model.SingleModule, time.Minute))

	if got := mgr.Find("s"); got == nil || got.Name != "sample" {
		t.Fatalf("Find by alias = %v, want entry sample", got)
	}
	if got := mgr.Find("sample"); got == nil {
		t.Fatalf("Find by name = nil, want entry")
	}
	if got := mgr.Remove("s"); got == nil {
		t.Fatalf("Remove by alias = nil, want entry")
	}
	if got := mgr.Find("sample"); got != nil {
		t.Fatalf("Find after remove = %v, want nil", got)
	}
}

func TestListIsSortedAndStable(t *testing.T) {
	mgr := NewManager()
	mgr.Add(entry("zzz", "zeta", model.SingleModule, time.Minute))
	mgr.Add(entry("aaa", "alpha", model.SingleModule, time.Minute))

	list := mgr.List()
	if len(list) != 2 {
		t.Fatalf("len(List()) = %d, want 2", len(list))
	}
	if list[0].EffectiveName() != "alpha" || list[1].EffectiveName() != "zeta" {
		t.Fatalf("List() order = [%s %s], want [alpha zeta]",
			list[0].EffectiveName(), list[1].EffectiveName())
	}
}

func TestExpiredEntriesAreSwept(t *testing.T) {
	mgr := NewManager()
	mgr.Add(entry("alive", "", model.SingleModule, time.Minute))
	mgr.Add(entry("dead", "", model.SingleModule, -time.Minute))

	if got := mgr.Count(); got != 1 {
		t.Fatalf("Count() = %d, want 1 (expired swept)", got)
	}
	if got := mgr.Find("dead"); got != nil {
		t.Fatalf("Find(expired) = %v, want nil", got)
	}
	if list := mgr.List(); len(list) != 1 || list[0].Name != "alive" {
		t.Fatalf("List() = %v, want [alive]", list)
	}
}

func TestEffectiveNameAndKind(t *testing.T) {
	e := entry("sample", "alias", model.Workspace, time.Minute)
	if e.EffectiveName() != "alias" {
		t.Fatalf("EffectiveName() = %q, want %q", e.EffectiveName(), "alias")
	}
	e.Alias = ""
	if e.EffectiveName() != "sample" {
		t.Fatalf("EffectiveName() = %q, want %q", e.EffectiveName(), "sample")
	}
	if model.Workspace.String() != "workspace" || model.SingleModule.String() != "module" {
		t.Fatalf("ProjectKind.String() broken: %q %q", model.Workspace, model.SingleModule)
	}
}
