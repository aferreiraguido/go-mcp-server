// list_loaded_projects.go — the first tool. Lists loaded projects.
//
// It is the "hello world" of the pattern: tests `Manager`, `BaseGoTool`,
// the `Tool` interface, and deterministic Markdown serialization.
// Direct equivalent of `ListLoadedProjectsTool` in rust/java/cobol.
package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"go-mcp-server/internal/project"
)

// ListLoadedProjectsTool implementa Tool.
type ListLoadedProjectsTool struct {
	base BaseGoTool
}

// NewListLoadedProjectsTool builds the tool.
func NewListLoadedProjectsTool(manager *project.Manager) *ListLoadedProjectsTool {
	return &ListLoadedProjectsTool{base: NewBaseGoTool(manager)}
}

// Name returns the stable tool name.
func (t *ListLoadedProjectsTool) Name() string { return "list_loaded_projects" }

// Description returns the LLM-readable description.
func (t *ListLoadedProjectsTool) Description() string {
	return "Lists all currently loaded Go projects. Output is Markdown, sorted " +
		"by effective name (alias if set, else directory name). Entries expire " +
		"after 10 minutes by default."
}

// InputSchema returns the input JSON Schema (no arguments).
func (t *ListLoadedProjectsTool) InputSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"properties":           map[string]any{},
		"additionalProperties": false,
	}
}

// Call executes the listing. Deterministic Markdown output:
// no timestamps, sorted by EffectiveName.
func (t *ListLoadedProjectsTool) Call(_ context.Context, _ map[string]any) (*mcp.CallToolResult, error) {
	projects := t.base.Manager.List()

	if len(projects) == 0 {
		return OKMarkdown("# Loaded Projects\n\n" +
			"_No projects loaded._\n\n" +
			"Use `load_go_project` to load a module or workspace.\n"), nil
	}

	var md strings.Builder
	fmt.Fprintf(&md, "# Loaded Projects (%d)\n\n", len(projects))
	md.WriteString("| Name | Alias | Kind | Path |\n")
	md.WriteString("|------|-------|------|------|\n")
	for _, p := range projects {
		alias := p.Alias
		if alias == "" {
			alias = "-"
		}
		fmt.Fprintf(&md, "| `%s` | `%s` | %s | `%s` |\n",
			p.Name, alias, p.Kind, p.Path)
	}

	hasMembers := false
	for _, p := range projects {
		if len(p.Modules) > 0 {
			hasMembers = true
			break
		}
	}
	if hasMembers {
		md.WriteString("\n## Workspace Members\n\n")
		for _, p := range projects {
			if len(p.Modules) == 0 {
				continue
			}
			fmt.Fprintf(&md, "- `%s`: %s\n", p.Name, strings.Join(p.Modules, ", "))
		}
	}
	return OKMarkdown(md.String()), nil
}
