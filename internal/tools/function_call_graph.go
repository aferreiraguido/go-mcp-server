package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"go-mcp-server/internal/project"
)

type FunctionCallGraphTool struct {
	base BaseGoTool
}

func NewFunctionCallGraphTool(manager *project.Manager) *FunctionCallGraphTool {
	return &FunctionCallGraphTool{base: NewBaseGoTool(manager)}
}

func (t *FunctionCallGraphTool) Name() string { return "function_call_graph" }

func (t *FunctionCallGraphTool) Description() string {
	return "Returns the call graph of a function or method: callers (who calls it) and " +
		"callees (who it calls), using go/types semantic analysis. " +
		"Each entry includes the calling/called function name, location, and source context."
}

func (t *FunctionCallGraphTool) InputSchema() map[string]any {
	s := nameOnlySchema()
	props := s["properties"].(map[string]any)
	props["function_name"] = map[string]any{
		"type":        "string",
		"description": "Function or method name to look up",
	}
	s["required"] = []string{"name", "function_name"}
	return s
}

func (t *FunctionCallGraphTool) Call(_ context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	_, proj, err := findAndAnalyzeSemantic(t.base, args)
	if err != nil {
		return OKError(err.Error()), nil
	}
	funcName, err := Arg(args, "function_name")
	if err != nil {
		return OKError(err.Error()), nil
	}

	cg, ok := proj.CallGraphOf(funcName)
	if !ok {
		return OKMarkdown(fmt.Sprintf(
			"# Call graph: `%s`\n\n_Function not found._\n", funcName)), nil
	}

	var md strings.Builder
	fmt.Fprintf(&md, "# Call graph: `%s`\n\n", funcName)

	// Callers
	if len(cg.Callers) > 0 {
		fmt.Fprintf(&md, "## Callers (%d)\n\n", len(cg.Callers))
		var rows [][]string
		for _, c := range cg.Callers {
			rows = append(rows, []string{
				"`" + c.Function + "`",
				c.Position,
				"`" + c.Context + "`",
			})
		}
		md.WriteString(table([]string{"Function", "Location", "Context"}, rows))
	} else {
		md.WriteString("## Callers\n\n_No callers found._\n\n")
	}

	// Callees
	if len(cg.Callees) > 0 {
		fmt.Fprintf(&md, "## Callees (%d)\n\n", len(cg.Callees))
		var rows [][]string
		for _, c := range cg.Callees {
			rows = append(rows, []string{
				"`" + c.Function + "`",
				c.Position,
				"`" + c.Context + "`",
			})
		}
		md.WriteString(table([]string{"Function", "Location", "Context"}, rows))
	} else {
		md.WriteString("## Callees\n\n_No callees found._\n")
	}

	return OKMarkdown(md.String()), nil
}
