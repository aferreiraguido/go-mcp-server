// inspect_interface.go — deep-dive into an interface: methods and embeds.
// Counterpart of `InspectTraitTool` (rust-mcp-server).
package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"go-mcp-server/internal/project"
)

// InspectInterfaceTool implementa Tool.
type InspectInterfaceTool struct {
	base BaseGoTool
}

// NewInspectInterfaceTool builds the tool.
func NewInspectInterfaceTool(manager *project.Manager) *InspectInterfaceTool {
	return &InspectInterfaceTool{base: NewBaseGoTool(manager)}
}

// Name returns the stable tool name.
func (t *InspectInterfaceTool) Name() string { return "inspect_interface" }

// Description returns the LLM-readable description.
func (t *InspectInterfaceTool) Description() string {
	return "Returns detailed info about a named interface: doc comment, required methods " +
		"with signatures, and embedded interfaces."
}

// InputSchema: name + interface_name required.
func (t *InspectInterfaceTool) InputSchema() map[string]any {
	s := nameOnlySchema()
	s["properties"].(map[string]any)["interface_name"] = map[string]any{
		"type":        "string",
		"description": "Interface name to inspect",
	}
	s["required"] = []string{"name", "interface_name"}
	return s
}

// Call inspects the interface.
func (t *InspectInterfaceTool) Call(_ context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	_, proj, err := findAndAnalyze(t.base, args)
	if err != nil {
		return OKError(err.Error()), nil
	}
	ifName, err := Arg(args, "interface_name")
	if err != nil {
		return OKError(err.Error()), nil
	}
	iface := proj.FindInterface(ifName)
	if iface == nil {
		return OKError("interface '" + ifName + "' not found in project. " +
			"Use list_interfaces to see available interfaces."), nil
	}

	var md strings.Builder
	md.WriteString("# Interface: `" + iface.Name + "`\n\n")
	md.WriteString(table([]string{"Field", "Value"}, [][]string{
		{"Exported", yesNo(iface.Exported)},
		{"Location", loc(iface.File, iface.Line)},
	}))
	if iface.Doc != "" {
		md.WriteString("\n## Doc\n\n" + iface.Doc + "\n")
	}

	fmt.Fprintf(&md, "\n## Methods (%d)\n\n", len(iface.Methods))
	if len(iface.Methods) == 0 {
		md.WriteString("_Empty interface (`interface{}` / `any`)._\n")
	} else {
		var rows [][]string
		for _, m := range iface.Methods {
			sig := m.Signature
			if m.Embedded {
				sig = "(embedded)"
			}
			rows = append(rows, []string{
				"`" + m.Name + "`", "`" + sig + "`", yesNo(m.Embedded),
			})
		}
		md.WriteString(table([]string{"Name", "Signature", "Embedded"}, rows))
	}
	return OKMarkdown(md.String()), nil
}
