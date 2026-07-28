// list_constants.go — package-level constants (incl. iota groups).
package tools

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"go-mcp-server/internal/project"
)

// ListConstantsTool implementa Tool.
type ListConstantsTool struct {
	base BaseGoTool
}

// NewListConstantsTool builds the tool.
func NewListConstantsTool(manager *project.Manager) *ListConstantsTool {
	return &ListConstantsTool{base: NewBaseGoTool(manager)}
}

// Name returns the stable tool name.
func (t *ListConstantsTool) Name() string { return "list_constants" }

// Description returns the LLM-readable description.
func (t *ListConstantsTool) Description() string {
	return "Lists all package-level constants in a loaded project (each spec of const blocks, " +
		"including iota enums): name, type, value expression, exported flag, and location."
}

// InputSchema: name required.
func (t *ListConstantsTool) InputSchema() map[string]any {
	return nameOnlySchema()
}

// Call lists the constants.
func (t *ListConstantsTool) Call(_ context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	_, proj, err := findAndAnalyze(t.base, args)
	if err != nil {
		return OKError(err.Error()), nil
	}
	consts := proj.Constants()
	if len(consts) == 0 {
		return OKMarkdown("# Constants\n\n_No constants found._\n"), nil
	}
	var rows [][]string
	for _, c := range consts {
		value := c.Value
		if value == "" {
			value = "(iota continuation)"
		}
		rows = append(rows, []string{
			"`" + c.Name + "`", "`" + dash(c.Type) + "`", "`" + truncate(value, 40) + "`",
			yesNo(c.Exported), loc(c.File, c.Line),
		})
	}
	md := fmt.Sprintf("# Constants (%d)\n\n", len(consts)) +
		table([]string{"Name", "Type", "Value", "Exported", "Location"}, rows)
	return OKMarkdown(md), nil
}
