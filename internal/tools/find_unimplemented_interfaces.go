package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"go-mcp-server/internal/project"
)

type FindUnimplementedInterfacesTool struct {
	base BaseGoTool
}

func NewFindUnimplementedInterfacesTool(manager *project.Manager) *FindUnimplementedInterfacesTool {
	return &FindUnimplementedInterfacesTool{base: NewBaseGoTool(manager)}
}

func (t *FindUnimplementedInterfacesTool) Name() string { return "find_unimplemented_interfaces" }

func (t *FindUnimplementedInterfacesTool) Description() string {
	return "Finds interfaces defined in the project that no concrete type implements. " +
		"Uses go/types semantic analysis. Empty interfaces (0 methods) are excluded."
}

func (t *FindUnimplementedInterfacesTool) InputSchema() map[string]any { return nameOnlySchema() }

func (t *FindUnimplementedInterfacesTool) Call(_ context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	_, proj, err := findAndAnalyzeSemantic(t.base, args)
	if err != nil {
		return OKError(err.Error()), nil
	}

	ifaces := proj.FindUnimplementedInterfaces()
	if len(ifaces) == 0 {
		return OKMarkdown("# Unimplemented Interfaces\n\n_All project interfaces have at least one implementation._\n"), nil
	}

	var rows [][]string
	for _, iface := range ifaces {
		rows = append(rows, []string{
			"`" + iface.Name + "`",
			fmt.Sprintf("%d", iface.Methods),
			iface.Position,
		})
	}
	var md strings.Builder
	fmt.Fprintf(&md, "# Unimplemented Interfaces (%d)\n\n", len(ifaces))
	md.WriteString(table([]string{"Interface", "Methods", "Location"}, rows))
	return OKMarkdown(md.String()), nil
}
