// inspect_function.go — deep-dive into a function: signature, doc, full body.
// Counterpart of `InspectFunctionTool` (rust-mcp-server).
package tools

import (
	"context"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"go-mcp-server/internal/project"
)

// InspectFunctionTool implementa Tool.
type InspectFunctionTool struct {
	base BaseGoTool
}

// NewInspectFunctionTool builds the tool.
func NewInspectFunctionTool(manager *project.Manager) *InspectFunctionTool {
	return &InspectFunctionTool{base: NewBaseGoTool(manager)}
}

// Name returns the stable tool name.
func (t *InspectFunctionTool) Name() string { return "inspect_function" }

// Description returns the LLM-readable description.
func (t *InspectFunctionTool) Description() string {
	return "Returns detailed info about a named function or method: signature, receiver, " +
		"exported flag, doc comment, location, and the full body source."
}

// InputSchema: name + function_name required.
func (t *InspectFunctionTool) InputSchema() map[string]any {
	s := nameOnlySchema()
	s["properties"].(map[string]any)["function_name"] = map[string]any{
		"type":        "string",
		"description": "Function or method name to inspect",
	}
	s["required"] = []string{"name", "function_name"}
	return s
}

// Call inspects the function.
func (t *InspectFunctionTool) Call(_ context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	_, proj, err := findAndAnalyze(t.base, args)
	if err != nil {
		return OKError(err.Error()), nil
	}
	fnName, err := Arg(args, "function_name")
	if err != nil {
		return OKError(err.Error()), nil
	}
	fn := proj.FindFunc(fnName)
	if fn == nil {
		return OKError("function '" + fnName + "' not found in project. " +
			"Use list_functions to see available functions."), nil
	}

	var md strings.Builder
	md.WriteString("# Function: `" + fn.Name + "`\n\n")
	md.WriteString(table([]string{"Field", "Value"}, [][]string{
		{"Signature", "`" + fn.Signature + "`"},
		{"Receiver", "`" + dash(fn.Receiver) + "`"},
		{"Exported", yesNo(fn.Exported)},
		{"Location", loc(fn.File, fn.Line)},
	}))
	if fn.Doc != "" {
		md.WriteString("\n## Doc\n\n" + fn.Doc + "\n")
	}
	if fn.Body != "" {
		md.WriteString("\n## Source\n\n")
		md.WriteString(codeBlock("go", strings.TrimRight(fn.Body, "\n")))
	}
	return OKMarkdown(md.String()), nil
}
