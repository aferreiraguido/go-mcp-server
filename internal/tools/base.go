// base.go — BaseGoTool: shared helpers for all tools.
//
// Counterpart of `tools/base_rust_tool.rs` (rust) and `BaseCobolTool`/`BaseJavaTool`.
// It is not an interface — it is a struct with utilities accessible via
// `t.base.Arg(...)` from each tool.
package tools

import (
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"go-mcp-server/internal/analysis"
	"go-mcp-server/internal/model"
	"go-mcp-server/internal/project"
	"go-mcp-server/internal/sem"
)

// BaseGoTool groups common dependencies and helpers for tools.
type BaseGoTool struct {
	Manager *project.Manager
}

// NewBaseGoTool builds the shared base.
func NewBaseGoTool(manager *project.Manager) BaseGoTool {
	return BaseGoTool{Manager: manager}
}

// Arg extracts a required string argument. Error if missing or not a string.
func Arg(args map[string]any, key string) (string, error) {
	v, ok := args[key]
	if !ok {
		return "", fmt.Errorf("missing required argument: '%s'", key)
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("argument '%s' must be a string", key)
	}
	return s, nil
}

// OptArg extracts an optional string argument ("" if missing).
func OptArg(args map[string]any, key string) string {
	if s, ok := args[key].(string); ok {
		return s
	}
	return ""
}

// BoolArg extracts a bool argument (false if missing).
func BoolArg(args map[string]any, key string) bool {
	if b, ok := args[key].(bool); ok {
		return b
	}
	return false
}

// FindEntry looks up a loaded project. MCP error if not found or expired.
func (b BaseGoTool) FindEntry(name string) (*model.ProjectEntry, error) {
	e := b.Manager.Find(name)
	if e == nil {
		return nil, fmt.Errorf(
			"project '%s' not found (or expired). Load it first with load_go_project", name)
	}
	return e, nil
}

// FindAndAnalyze resolves the project by name and returns its cached
// analysis (go/ast + go.mod parsing). It is the entry point for almost
// all iter 2+ tools.
func (b BaseGoTool) FindAndAnalyze(name string) (*model.ProjectEntry, *analysis.Project, error) {
	e, err := b.FindEntry(name)
	if err != nil {
		return nil, nil, err
	}
	proj, err := b.Manager.Analysis(e)
	if err != nil {
		return nil, nil, err
	}
	return e, proj, nil
}

// IntArg extracts an optional integer argument with a default.
// JSON deserializes numbers as float64.
func IntArg(args map[string]any, key string, def int) int {
	if f, ok := args[key].(float64); ok {
		return int(f)
	}
	return def
}

// FindAndAnalyzeSemantic resolves the project by name and returns its
// SEMANTIC analysis (go/packages + go/types). It is the entry point
// for semantic tools (iter 3). The analysis is expensive cold —
// the Manager caches it per entry.
func (b BaseGoTool) FindAndAnalyzeSemantic(name string) (*model.ProjectEntry, *sem.Project, error) {
	e, err := b.FindEntry(name)
	if err != nil {
		return nil, nil, err
	}
	proj, err := b.Manager.SemanticAnalysis(e)
	if err != nil {
		return nil, nil, err
	}
	return e, proj, nil
}

// nameOnlySchema is the most common input schema: only `name` required.
// Tools with more args start from it and add properties.
func nameOnlySchema() map[string]any {
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

// findAndAnalyzeSemantic is a one-line helper for semantic tools
// with a name-only schema: extracts `name`, locates the entry, and returns its
// semantic analysis (go/packages + go/types).
func findAndAnalyzeSemantic(b BaseGoTool, args map[string]any) (*model.ProjectEntry, *sem.Project, error) {
	name, err := Arg(args, "name")
	if err != nil {
		return nil, nil, err
	}
	return b.FindAndAnalyzeSemantic(name)
}

// findAndAnalyze is a one-line helper for tools with a name-only schema:
// extracts `name`, locates the entry, and returns its analysis.
func findAndAnalyze(b BaseGoTool, args map[string]any) (*model.ProjectEntry, *analysis.Project, error) {
	name, err := Arg(args, "name")
	if err != nil {
		return nil, nil, err
	}
	return b.FindAndAnalyze(name)
}

// OKMarkdown wraps Markdown as a successful CallToolResult.
func OKMarkdown(markdown string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: markdown}},
	}
}

// OKError wraps an LLM-readable error as a CallToolResult with
// IsError=true (semantically "the tool returned an input error", not a
// crash). Keeps the API shape predictable.
func OKError(message string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{&mcp.TextContent{
			Text: fmt.Sprintf("# Error\n\n```\n%s\n```\n", message),
		}},
	}
}
