// unload_go_project.go — unloads a project from memory.
// Counterpart of `UnloadRustProjectTool` (rust-mcp-server).
package tools

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"go-mcp-server/internal/project"
)

// UnloadGoProjectTool implementa Tool.
type UnloadGoProjectTool struct {
	base BaseGoTool
}

// NewUnloadGoProjectTool builds the tool.
func NewUnloadGoProjectTool(manager *project.Manager) *UnloadGoProjectTool {
	return &UnloadGoProjectTool{base: NewBaseGoTool(manager)}
}

// Name returns the stable tool name.
func (t *UnloadGoProjectTool) Name() string { return "unload_go_project" }

// Description returns the LLM-readable description.
func (t *UnloadGoProjectTool) Description() string {
	return "Removes a loaded Go project from memory (by name or alias). " +
		"If it was a temp dir (Git clone or extracted archive), it is deleted from disk."
}

// InputSchema: name required.
func (t *UnloadGoProjectTool) InputSchema() map[string]any {
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

// Call removes the entry from the Manager.
func (t *UnloadGoProjectTool) Call(_ context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	name, err := Arg(args, "name")
	if err != nil {
		return OKError(err.Error()), nil
	}
	removed := t.base.Manager.Remove(name)
	if removed == nil {
		return OKError(fmt.Sprintf(
			"project '%s' not found (or expired). Nothing to unload.", name)), nil
	}
	return OKMarkdown(fmt.Sprintf("# Project Unloaded\n\n`%s` has been removed from memory.\n",
		removed.EffectiveName())), nil
}
