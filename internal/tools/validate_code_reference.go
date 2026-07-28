// validate_code_reference.go — EXISTS / NOT_FOUND for top-level symbols.
// Counterpart of `ValidateCodeReferenceTool` (rust-mcp-server).
package tools

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"go-mcp-server/internal/project"
)

// ValidateCodeReferenceTool implementa Tool.
type ValidateCodeReferenceTool struct {
	base BaseGoTool
}

// NewValidateCodeReferenceTool builds the tool.
func NewValidateCodeReferenceTool(manager *project.Manager) *ValidateCodeReferenceTool {
	return &ValidateCodeReferenceTool{base: NewBaseGoTool(manager)}
}

// Name returns the stable tool name.
func (t *ValidateCodeReferenceTool) Name() string { return "validate_code_reference" }

// Description returns the LLM-readable description.
func (t *ValidateCodeReferenceTool) Description() string {
	return "Returns EXISTS / NOT_FOUND for a top-level symbol name in a loaded project. " +
		"Symbols checked: functions, types (structs, interfaces, aliases), constants, " +
		"variables. Does NOT match methods (those live on receiver types — use find_references)."
}

// InputSchema: name + symbol required.
func (t *ValidateCodeReferenceTool) InputSchema() map[string]any {
	s := nameOnlySchema()
	s["properties"].(map[string]any)["symbol"] = map[string]any{
		"type":        "string",
		"description": "Symbol name to look up",
	}
	s["required"] = []string{"name", "symbol"}
	return s
}

// Call validates the symbol.
func (t *ValidateCodeReferenceTool) Call(_ context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	_, proj, err := findAndAnalyze(t.base, args)
	if err != nil {
		return OKError(err.Error()), nil
	}
	symbol, err := Arg(args, "symbol")
	if err != nil {
		return OKError(err.Error()), nil
	}

	kind, file, line, found := proj.TopLevelSymbol(symbol)
	if !found {
		return OKMarkdown(fmt.Sprintf(
			"# Validate: `%s`\n\n**NOT_FOUND** — no top-level symbol with that name.\n\n"+
				"Note: methods are not matched (top-level func/type/const/var only).\n",
			symbol)), nil
	}
	return OKMarkdown(fmt.Sprintf(
		"# Validate: `%s`\n\n**EXISTS** — %s at %s\n", symbol, kind, loc(file, line))), nil
}
