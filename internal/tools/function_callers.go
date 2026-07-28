// function_callers.go — calls to a function (textual: `symbol(`).
// Counterpart of `FunctionCallersTool` (rust-mcp-server iter 2).
//
// NOTE: textual. The semantic call graph arrives in iter 3 (go/packages).
package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"go-mcp-server/internal/project"
)

// FunctionCallersTool implementa Tool.
type FunctionCallersTool struct {
	base BaseGoTool
}

// NewFunctionCallersTool builds the tool.
func NewFunctionCallersTool(manager *project.Manager) *FunctionCallersTool {
	return &FunctionCallersTool{base: NewBaseGoTool(manager)}
}

// Name returns the stable tool name.
func (t *FunctionCallersTool) Name() string { return "function_callers" }

// Description returns the LLM-readable description.
func (t *FunctionCallersTool) Description() string {
	return "Text-based search for callers of a function: finds lines that contain " +
		"'symbol(' (function-call style, whole-word). For a true call-graph analysis " +
		"with type resolution, a semantic layer arrives in a later iteration."
}

// InputSchema: name + symbol required; max_results optional.
func (t *FunctionCallersTool) InputSchema() map[string]any {
	s := nameOnlySchema()
	props := s["properties"].(map[string]any)
	props["symbol"] = map[string]any{
		"type":        "string",
		"description": "Function name to look up callers for",
	}
	props["max_results"] = map[string]any{
		"type":        "integer",
		"description": "Limit number of results (default 100)",
	}
	s["required"] = []string{"name", "symbol"}
	return s
}

// Call searches for `symbol(` calls.
func (t *FunctionCallersTool) Call(_ context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	_, proj, err := findAndAnalyze(t.base, args)
	if err != nil {
		return OKError(err.Error()), nil
	}
	symbol, err := Arg(args, "symbol")
	if err != nil {
		return OKError(err.Error()), nil
	}
	maxResults := IntArg(args, "max_results", 100)

	matches := proj.FindWholeWordRefs(symbol, true, maxResults)
	if len(matches) == 0 {
		return OKMarkdown(fmt.Sprintf("# Callers: `%s(`\n\n_No call sites found._\n", symbol)), nil
	}
	var rows [][]string
	for _, m := range matches {
		rows = append(rows, []string{loc(m.File, m.Line), "`" + truncate(m.Text, 120) + "`"})
	}
	var md strings.Builder
	fmt.Fprintf(&md, "# Callers: `%s(` (%d)\n\n", symbol, len(matches))
	if len(matches) >= maxResults {
		fmt.Fprintf(&md, "_Showing first %d results._\n\n", maxResults)
	}
	md.WriteString(table([]string{"Location", "Context"}, rows))
	return OKMarkdown(md.String()), nil
}
