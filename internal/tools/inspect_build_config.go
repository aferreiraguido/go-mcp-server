// inspect_build_config.go — raw content of go.mod (and go.work if present).
// Counterpart of `InspectBuildConfigTool` (rust-mcp-server, which returns Cargo.toml).
package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"go-mcp-server/internal/project"
)

// InspectBuildConfigTool implementa Tool.
type InspectBuildConfigTool struct {
	base BaseGoTool
}

// NewInspectBuildConfigTool builds the tool.
func NewInspectBuildConfigTool(manager *project.Manager) *InspectBuildConfigTool {
	return &InspectBuildConfigTool{base: NewBaseGoTool(manager)}
}

// Name returns the stable tool name.
func (t *InspectBuildConfigTool) Name() string { return "inspect_build_config" }

// Description returns the LLM-readable description.
func (t *InspectBuildConfigTool) Description() string {
	return "Returns the raw go.mod content of a loaded project (and go.work if present). " +
		"Use this to see the full build configuration."
}

// InputSchema: name required.
func (t *InspectBuildConfigTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{
				"type":        "string",
				"description": "Project name or alias",
			},
		},
		"required":             []string{"name"},
		"additionalProperties": false,
	}
}

// Call reads go.mod/go.work from disk (may not be in the analysis cache
// as raw text).
func (t *InspectBuildConfigTool) Call(_ context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	name, err := Arg(args, "name")
	if err != nil {
		return OKError(err.Error()), nil
	}
	entry, err := t.base.FindEntry(name)
	if err != nil {
		return OKError(err.Error()), nil
	}

	var md strings.Builder
	md.WriteString("# Build Configuration\n\n")

	gomod, err := os.ReadFile(filepath.Join(entry.Path, "go.mod"))
	if err != nil {
		return OKError("cannot read go.mod: " + err.Error()), nil
	}
	md.WriteString("## go.mod\n\n")
	md.WriteString(codeBlock("", strings.TrimRight(string(gomod), "\n")))
	md.WriteString("\n")

	if gowork, err := os.ReadFile(filepath.Join(entry.Path, "go.work")); err == nil {
		md.WriteString("## go.work\n\n")
		md.WriteString(codeBlock("", strings.TrimRight(string(gowork), "\n")))
		md.WriteString("\n")
	}
	return OKMarkdown(md.String()), nil
}
