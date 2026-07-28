// project_metadata.go — metadata of a loaded project.
// Counterpart of `ProjectMetadataTool` (rust-mcp-server).
//
// It is the ONLY tool that may expose timestamps (LoadedAt/ExpiresAt), and
// only under the explicit flag `include_expiry: true` — see §8 of AGENTS.md
// (snapshot determinism).
package tools

import (
	"context"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"go-mcp-server/internal/project"
)

// ProjectMetadataTool implementa Tool.
type ProjectMetadataTool struct {
	base BaseGoTool
}

// NewProjectMetadataTool builds the tool.
func NewProjectMetadataTool(manager *project.Manager) *ProjectMetadataTool {
	return &ProjectMetadataTool{base: NewBaseGoTool(manager)}
}

// Name returns the stable tool name.
func (t *ProjectMetadataTool) Name() string { return "project_metadata" }

// Description returns the LLM-readable description.
func (t *ProjectMetadataTool) Description() string {
	return "Returns metadata for a loaded project: name, alias, kind, path, module path, " +
		"Go version, member modules (if workspace). Timestamps only with include_expiry=true."
}

// InputSchema: name required; include_expiry optional.
func (t *ProjectMetadataTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{
				"type":        "string",
				"description": "Project name or alias",
			},
			"include_expiry": map[string]any{
				"type":        "boolean",
				"description": "Include loaded_at/expires_at timestamps (default false)",
			},
		},
		"required":             []string{"name"},
		"additionalProperties": false,
	}
}

// Call builds the metadata table.
func (t *ProjectMetadataTool) Call(_ context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	name, err := Arg(args, "name")
	if err != nil {
		return OKError(err.Error()), nil
	}
	entry, err := t.base.FindEntry(name)
	if err != nil {
		return OKError(err.Error()), nil
	}

	rows := [][]string{
		{"Name", "`" + entry.Name + "`"},
		{"Alias", "`" + dash(entry.Alias) + "`"},
		{"Kind", entry.Kind.String()},
		{"Path", "`" + entry.Path + "`"},
		{"Module", "`" + dash(entry.ModulePath) + "`"},
		{"Go", dash(entry.GoVersion)},
	}
	if len(entry.Modules) > 0 {
		rows = append(rows, []string{"Members", "`" + strings.Join(entry.Modules, "`, `") + "`"})
	}
	if BoolArg(args, "include_expiry") {
		rows = append(rows,
			[]string{"Loaded at", entry.LoadedAt.Format(time.RFC3339)},
			[]string{"Expires at", entry.ExpiresAt.Format(time.RFC3339)},
		)
	}

	md := "# Project Metadata\n\n" + table([]string{"Field", "Value"}, rows)
	return OKMarkdown(md), nil
}
