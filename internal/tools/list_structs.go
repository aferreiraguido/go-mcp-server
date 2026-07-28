// list_structs.go — struct types in the project.
// Counterpart of `ListStructsTool` (rust-mcp-server).
package tools

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"go-mcp-server/internal/project"
)

// ListStructsTool implementa Tool.
type ListStructsTool struct {
	base BaseGoTool
}

// NewListStructsTool builds the tool.
func NewListStructsTool(manager *project.Manager) *ListStructsTool {
	return &ListStructsTool{base: NewBaseGoTool(manager)}
}

// Name returns the stable tool name.
func (t *ListStructsTool) Name() string { return "list_structs" }

// Description returns the LLM-readable description.
func (t *ListStructsTool) Description() string {
	return "Lists all struct types in a loaded project: name, field count, exported flag, " +
		"and location. Use inspect_struct for field-level detail."
}

// InputSchema: name required.
func (t *ListStructsTool) InputSchema() map[string]any {
	return nameOnlySchema()
}

// Call lists the structs.
func (t *ListStructsTool) Call(_ context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	_, proj, err := findAndAnalyze(t.base, args)
	if err != nil {
		return OKError(err.Error()), nil
	}
	structs := proj.Structs()
	if len(structs) == 0 {
		return OKMarkdown("# Structs\n\n_No structs found._\n"), nil
	}
	var rows [][]string
	for _, s := range structs {
		rows = append(rows, []string{
			"`" + s.Name + "`", fmt.Sprintf("%d", len(s.Fields)),
			yesNo(s.Exported), loc(s.File, s.Line),
		})
	}
	md := fmt.Sprintf("# Structs (%d)\n\n", len(structs)) +
		table([]string{"Name", "Fields", "Exported", "Location"}, rows)
	return OKMarkdown(md), nil
}
