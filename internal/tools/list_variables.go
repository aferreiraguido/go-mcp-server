// list_variables.go — package-level variables in the project.
package tools

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"go-mcp-server/internal/project"
)

// ListVariablesTool implementa Tool.
type ListVariablesTool struct {
	base BaseGoTool
}

// NewListVariablesTool builds the tool.
func NewListVariablesTool(manager *project.Manager) *ListVariablesTool {
	return &ListVariablesTool{base: NewBaseGoTool(manager)}
}

// Name returns the stable tool name.
func (t *ListVariablesTool) Name() string { return "list_variables" }

// Description returns the LLM-readable description.
func (t *ListVariablesTool) Description() string {
	return "Lists all package-level variables in a loaded project: name, type (or initializer " +
		"if inferred), exported flag, and location. Mutable package state is a common bug source — " +
		"this list is a good audit starting point."
}

// InputSchema: name required.
func (t *ListVariablesTool) InputSchema() map[string]any {
	return nameOnlySchema()
}

// Call lists the variables.
func (t *ListVariablesTool) Call(_ context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	_, proj, err := findAndAnalyze(t.base, args)
	if err != nil {
		return OKError(err.Error()), nil
	}
	vars := proj.Variables()
	if len(vars) == 0 {
		return OKMarkdown("# Variables\n\n_No package-level variables found._\n"), nil
	}
	var rows [][]string
	for _, v := range vars {
		rows = append(rows, []string{
			"`" + v.Name + "`", "`" + truncate(dash(v.Type), 40) + "`",
			yesNo(v.Exported), loc(v.File, v.Line),
		})
	}
	md := fmt.Sprintf("# Variables (%d)\n\n", len(vars)) +
		table([]string{"Name", "Type", "Exported", "Location"}, rows)
	return OKMarkdown(md), nil
}
