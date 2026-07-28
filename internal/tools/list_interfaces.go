// list_interfaces.go — interface types in the project.
// Counterpart of `ListTraitsTool` (rust-mcp-server): Go interfaces are
// the analogue of Rust traits.
package tools

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"go-mcp-server/internal/project"
)

// ListInterfacesTool implementa Tool.
type ListInterfacesTool struct {
	base BaseGoTool
}

// NewListInterfacesTool builds the tool.
func NewListInterfacesTool(manager *project.Manager) *ListInterfacesTool {
	return &ListInterfacesTool{base: NewBaseGoTool(manager)}
}

// Name returns the stable tool name.
func (t *ListInterfacesTool) Name() string { return "list_interfaces" }

// Description returns the LLM-readable description.
func (t *ListInterfacesTool) Description() string {
	return "Lists all interface types in a loaded project: name, method count (including " +
		"embedded interfaces), exported flag, and location. Use inspect_interface for method detail."
}

// InputSchema: name required.
func (t *ListInterfacesTool) InputSchema() map[string]any {
	return nameOnlySchema()
}

// Call lists the interfaces.
func (t *ListInterfacesTool) Call(_ context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	_, proj, err := findAndAnalyze(t.base, args)
	if err != nil {
		return OKError(err.Error()), nil
	}
	ifaces := proj.Interfaces()
	if len(ifaces) == 0 {
		return OKMarkdown("# Interfaces\n\n_No interfaces found._\n"), nil
	}
	var rows [][]string
	for _, i := range ifaces {
		rows = append(rows, []string{
			"`" + i.Name + "`", fmt.Sprintf("%d", len(i.Methods)),
			yesNo(i.Exported), loc(i.File, i.Line),
		})
	}
	md := fmt.Sprintf("# Interfaces (%d)\n\n", len(ifaces)) +
		table([]string{"Name", "Methods", "Exported", "Location"}, rows)
	return OKMarkdown(md), nil
}
