// list_functions.go — project functions and methods.
// Counterpart of `ListFunctionsTool` (rust-mcp-server), with visibility filter.
package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"go-mcp-server/internal/project"
)

// ListFunctionsTool implementa Tool.
type ListFunctionsTool struct {
	base BaseGoTool
}

// NewListFunctionsTool builds the tool.
func NewListFunctionsTool(manager *project.Manager) *ListFunctionsTool {
	return &ListFunctionsTool{base: NewBaseGoTool(manager)}
}

// Name returns the stable tool name.
func (t *ListFunctionsTool) Name() string { return "list_functions" }

// Description returns the LLM-readable description.
func (t *ListFunctionsTool) Description() string {
	return "Lists all functions and methods in a loaded project. For each one: signature, " +
		"receiver (if method), exported flag, and location. Optional visibility filter: " +
		"all (default) | exported | unexported."
}

// InputSchema: name required; visibility optional.
func (t *ListFunctionsTool) InputSchema() map[string]any {
	s := nameOnlySchema()
	s["properties"].(map[string]any)["visibility"] = map[string]any{
		"type":        "string",
		"description": "Filter: all (default) | exported | unexported",
		"enum":        []string{"all", "exported", "unexported"},
	}
	return s
}

// Call lists the functions.
func (t *ListFunctionsTool) Call(_ context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	_, proj, err := findAndAnalyze(t.base, args)
	if err != nil {
		return OKError(err.Error()), nil
	}
	visibility := OptArg(args, "visibility")

	var rows [][]string
	total := 0
	for _, f := range proj.Functions() {
		if visibility == "exported" && !f.Exported {
			continue
		}
		if visibility == "unexported" && f.Exported {
			continue
		}
		total++
		sig := strings.TrimPrefix(f.Signature, "func "+f.Name)
		rows = append(rows, []string{
			"`" + f.Name + "`", "`" + sig + "`", "`" + dash(f.Receiver) + "`",
			yesNo(f.Exported), loc(f.File, f.Line),
		})
	}
	if total == 0 {
		return OKMarkdown("# Functions\n\n_No functions found._\n"), nil
	}
	md := fmt.Sprintf("# Functions (%d)\n\n", total) +
		table([]string{"Name", "Signature", "Receiver", "Exported", "Location"}, rows)
	return OKMarkdown(md), nil
}
