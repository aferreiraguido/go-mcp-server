// inspect_struct.go — deep-dive into a struct: fields, tags, methods.
// Counterpart of `InspectStructTool` (rust-mcp-server).
package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"go-mcp-server/internal/project"
)

// InspectStructTool implementa Tool.
type InspectStructTool struct {
	base BaseGoTool
}

// NewInspectStructTool builds the tool.
func NewInspectStructTool(manager *project.Manager) *InspectStructTool {
	return &InspectStructTool{base: NewBaseGoTool(manager)}
}

// Name returns the stable tool name.
func (t *InspectStructTool) Name() string { return "inspect_struct" }

// Description returns the LLM-readable description.
func (t *InspectStructTool) Description() string {
	return "Returns detailed info about a named struct: doc comment, field table (name, type, " +
		"tag, embedded, exported), and the list of methods with that receiver type."
}

// InputSchema: name + struct_name required.
func (t *InspectStructTool) InputSchema() map[string]any {
	s := nameOnlySchema()
	s["properties"].(map[string]any)["struct_name"] = map[string]any{
		"type":        "string",
		"description": "Struct name to inspect",
	}
	s["required"] = []string{"name", "struct_name"}
	return s
}

// Call inspects the struct.
func (t *InspectStructTool) Call(_ context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	_, proj, err := findAndAnalyze(t.base, args)
	if err != nil {
		return OKError(err.Error()), nil
	}
	stName, err := Arg(args, "struct_name")
	if err != nil {
		return OKError(err.Error()), nil
	}
	st := proj.FindStruct(stName)
	if st == nil {
		return OKError("struct '" + stName + "' not found in project. " +
			"Use list_structs to see available structs."), nil
	}

	var md strings.Builder
	md.WriteString("# Struct: `" + st.Name + "`\n\n")
	md.WriteString(table([]string{"Field", "Value"}, [][]string{
		{"Exported", yesNo(st.Exported)},
		{"Location", loc(st.File, st.Line)},
	}))
	if st.Doc != "" {
		md.WriteString("\n## Doc\n\n" + st.Doc + "\n")
	}

	fmt.Fprintf(&md, "\n## Fields (%d)\n\n", len(st.Fields))
	if len(st.Fields) == 0 {
		md.WriteString("_Empty struct._\n")
	} else {
		var rows [][]string
		for _, f := range st.Fields {
			rows = append(rows, []string{
				"`" + f.Name + "`", "`" + f.Type + "`", "`" + dash(f.Tag) + "`",
				yesNo(f.Embedded), yesNo(f.Exported),
			})
		}
		md.WriteString(table([]string{"Name", "Type", "Tag", "Embedded", "Exported"}, rows))
	}

	methods := proj.MethodsOf(st.Name)
	fmt.Fprintf(&md, "\n## Methods (%d)\n\n", len(methods))
	if len(methods) == 0 {
		md.WriteString("_No methods with this receiver._\n")
	} else {
		var rows [][]string
		for _, m := range methods {
			sig := strings.TrimPrefix(m.Signature, "func "+m.Name)
			rows = append(rows, []string{
				"`" + m.Name + "`", "`" + sig + "`", "`" + m.Receiver + "`", loc(m.File, m.Line),
			})
		}
		md.WriteString(table([]string{"Name", "Signature", "Receiver", "Location"}, rows))
	}
	return OKMarkdown(md.String()), nil
}
