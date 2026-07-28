package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"go-mcp-server/internal/project"
)

type ResolveTypeTool struct {
	base BaseGoTool
}

func NewResolveTypeTool(manager *project.Manager) *ResolveTypeTool {
	return &ResolveTypeTool{base: NewBaseGoTool(manager)}
}

func (t *ResolveTypeTool) Name() string { return "resolve_type" }

func (t *ResolveTypeTool) Description() string {
	return "Resolves a symbol name to its definition site using the go/types semantic engine. " +
		"Returns the kind, type, package, and location of each definition. " +
		"Works for functions, types, constants, and variables."
}

func (t *ResolveTypeTool) InputSchema() map[string]any {
	s := nameOnlySchema()
	props := s["properties"].(map[string]any)
	props["symbol"] = map[string]any{
		"type":        "string",
		"description": "Symbol name to resolve",
	}
	s["required"] = []string{"name", "symbol"}
	return s
}

func (t *ResolveTypeTool) Call(_ context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	_, proj, err := findAndAnalyzeSemantic(t.base, args)
	if err != nil {
		return OKError(err.Error()), nil
	}
	symbol, err := Arg(args, "symbol")
	if err != nil {
		return OKError(err.Error()), nil
	}

	results := proj.Resolve(symbol)
	if len(results) == 0 {
		return OKMarkdown(fmt.Sprintf(
			"# Resolve: `%s`\n\n_No symbol found._\n", symbol)), nil
	}

	var md strings.Builder
	fmt.Fprintf(&md, "# Resolve: `%s` (%d)\n\n", symbol, len(results))
	var rows [][]string
	for _, r := range results {
		rows = append(rows, []string{
			"`" + r.Kind + "`",
			"`" + r.Type + "`",
			"`" + r.Package + "`",
			r.Position,
		})
	}
	md.WriteString(table([]string{"Kind", "Type", "Package", "Location"}, rows))
	return OKMarkdown(md.String()), nil
}
