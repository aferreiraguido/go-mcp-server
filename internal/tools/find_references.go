package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"go-mcp-server/internal/project"
	"go-mcp-server/internal/sem"
)

type FindReferencesTool struct {
	base BaseGoTool
}

func NewFindReferencesTool(manager *project.Manager) *FindReferencesTool {
	return &FindReferencesTool{base: NewBaseGoTool(manager)}
}

func (t *FindReferencesTool) Name() string { return "find_references" }

func (t *FindReferencesTool) Description() string {
	return "Finds all references to a symbol across the project using go/types semantic " +
		"analysis (Uses + Defs). When the symbol is not found in the semantic index, " +
		"falls back to whole-word textual search. Set semantic=false to force textual."
}

func (t *FindReferencesTool) InputSchema() map[string]any {
	s := nameOnlySchema()
	props := s["properties"].(map[string]any)
	props["symbol"] = map[string]any{
		"type":        "string",
		"description": "Symbol name to look up",
	}
	props["semantic"] = map[string]any{
		"type":        "boolean",
		"description": "Use semantic (go/types) resolution (default true). Set false for textual only.",
	}
	props["max_results"] = map[string]any{
		"type":        "integer",
		"description": "Limit number of results (default 200)",
	}
	return s
}

func (t *FindReferencesTool) Call(_ context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	symbol, err := Arg(args, "symbol")
	if err != nil {
		return OKError(err.Error()), nil
	}
	maxResults := IntArg(args, "max_results", 200)
	semantic := true
	if v, ok := args["semantic"]; ok {
		if b, ok := v.(bool); ok {
			semantic = b
		}
	}

	if semantic {
		_, sProj, err := findAndAnalyzeSemantic(t.base, args)
		if err == nil {
			refs := sProj.FindSemanticRefs(symbol, maxResults)
			if len(refs) > 0 {
				return formatSemanticRefs(symbol, refs, maxResults), nil
			}
		}
	}

	_, proj, err := findAndAnalyze(t.base, args)
	if err != nil {
		return OKError(err.Error()), nil
	}

	matches := proj.FindWholeWordRefs(symbol, false, maxResults)
	if len(matches) == 0 {
		return OKMarkdown(fmt.Sprintf("# References: `%s`\n\n_No references found._\n", symbol)), nil
	}
	var rows [][]string
	for _, m := range matches {
		rows = append(rows, []string{loc(m.File, m.Line), "`" + truncate(m.Text, 120) + "`"})
	}
	var md strings.Builder
	fmt.Fprintf(&md, "# References: `%s` (%d, textual)\n\n", symbol, len(matches))
	if len(matches) >= maxResults {
		fmt.Fprintf(&md, "_Showing first %d results._\n\n", maxResults)
	}
	md.WriteString(table([]string{"Location", "Context"}, rows))
	return OKMarkdown(md.String()), nil
}

// formatSemanticRefs renders semantic references as Markdown.
func formatSemanticRefs(symbol string, refs []sem.SemanticRef, maxResults int) *mcp.CallToolResult {
	var md strings.Builder
	fmt.Fprintf(&md, "# References: `%s` (%d, semantic)\n\n", symbol, len(refs))
	if len(refs) >= maxResults {
		fmt.Fprintf(&md, "_Showing first %d results._\n\n", maxResults)
	}
	var rows [][]string
	for _, r := range refs {
		rows = append(rows, []string{
			"`" + r.Kind + "`",
			"`" + r.Function + "`",
			r.Position,
			"`" + r.Context + "`",
		})
	}
	md.WriteString(table([]string{"Kind", "Function", "Location", "Context"}, rows))
	return OKMarkdown(md.String())
}
