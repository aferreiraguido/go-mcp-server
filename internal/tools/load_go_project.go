// load_go_project.go — loads a Go project into memory.
// Counterpart of `LoadRustProjectTool` (rust-mcp-server).
package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"go-mcp-server/internal/project"
)

// LoadGoProjectTool implementa Tool.
type LoadGoProjectTool struct {
	base BaseGoTool
}

// NewLoadGoProjectTool builds the tool.
func NewLoadGoProjectTool(manager *project.Manager) *LoadGoProjectTool {
	return &LoadGoProjectTool{base: NewBaseGoTool(manager)}
}

// Name returns the stable tool name.
func (t *LoadGoProjectTool) Name() string { return "load_go_project" }

// Description returns the LLM-readable description.
func (t *LoadGoProjectTool) Description() string {
	return "Loads a Go project into memory. Source can be a local path, a Git URL " +
		"(https://github.com/foo/bar or git@github.com:foo/bar), or an archive URL " +
		"(.zip, .tar.gz). The project is cached for 10 minutes (configurable via expiry_minutes)."
}

// InputSchema: source required; alias and expiry_minutes optional.
func (t *LoadGoProjectTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"source": map[string]any{
				"type":        "string",
				"description": "Local path, Git URL, or archive URL (.zip/.tar.gz)",
			},
			"alias": map[string]any{
				"type":        "string",
				"description": "Optional alias to reference the project",
			},
			"expiry_minutes": map[string]any{
				"type":        "integer",
				"description": "Cache expiration in minutes (default 10)",
			},
		},
		"required":             []string{"source"},
		"additionalProperties": false,
	}
}

// Call resolves the source, parses the project, and registers the entry.
func (t *LoadGoProjectTool) Call(_ context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	source, err := Arg(args, "source")
	if err != nil {
		return OKError(err.Error()), nil
	}
	alias := OptArg(args, "alias")
	expiry := time.Duration(IntArg(args, "expiry_minutes", 10)) * time.Minute

	entry, err := t.base.Manager.Load(source, alias, expiry)
	if err != nil {
		return OKError(err.Error()), nil
	}

	var rows [][]string
	rows = append(rows,
		[]string{"Name", "`" + entry.Name + "`"},
		[]string{"Alias", "`" + dash(entry.Alias) + "`"},
		[]string{"Kind", entry.Kind.String()},
		[]string{"Path", "`" + entry.Path + "`"},
		[]string{"Module", "`" + dash(entry.ModulePath) + "`"},
		[]string{"Go", dash(entry.GoVersion)},
		[]string{"Expires", fmt.Sprintf("%d minutes", int(expiry.Minutes()))},
	)

	var md strings.Builder
	md.WriteString("# Project Loaded\n\n")
	md.WriteString(table([]string{"Field", "Value"}, rows))
	if len(entry.Modules) > 0 {
		md.WriteString("\n## Workspace Members\n\n")
		for _, m := range entry.Modules {
			md.WriteString("- `" + m + "`\n")
		}
	}
	md.WriteString("\nUse `list_functions`, `list_structs`, `program_summary`... with name `" +
		entry.EffectiveName() + "` to analyze it.\n")
	return OKMarkdown(md.String()), nil
}
