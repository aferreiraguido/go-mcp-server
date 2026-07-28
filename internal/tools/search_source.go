// search_source.go — text search across project sources.
// Counterpart of `SearchSourceTool` (rust-mcp-server).
package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"go-mcp-server/internal/project"
)

// SearchSourceTool implementa Tool.
type SearchSourceTool struct {
	base BaseGoTool
}

// NewSearchSourceTool builds the tool.
func NewSearchSourceTool(manager *project.Manager) *SearchSourceTool {
	return &SearchSourceTool{base: NewBaseGoTool(manager)}
}

// Name returns the stable tool name.
func (t *SearchSourceTool) Name() string { return "search_source" }

// Description returns the LLM-readable description.
func (t *SearchSourceTool) Description() string {
	return "Text search across all .go files in a loaded project. Returns matching lines " +
		"with file:line:content format. Case-sensitive by default; set case_sensitive=false " +
		"for case-insensitive."
}

// InputSchema: name + query required; case_sensitive and max_results optional.
func (t *SearchSourceTool) InputSchema() map[string]any {
	s := nameOnlySchema()
	props := s["properties"].(map[string]any)
	props["query"] = map[string]any{
		"type":        "string",
		"description": "Text to search for",
	}
	props["case_sensitive"] = map[string]any{
		"type":        "boolean",
		"description": "Case-sensitive search (default true)",
	}
	props["max_results"] = map[string]any{
		"type":        "integer",
		"description": "Limit number of results (default 200)",
	}
	s["required"] = []string{"name", "query"}
	return s
}

// Call runs the search.
func (t *SearchSourceTool) Call(_ context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	_, proj, err := findAndAnalyze(t.base, args)
	if err != nil {
		return OKError(err.Error()), nil
	}
	query, err := Arg(args, "query")
	if err != nil {
		return OKError(err.Error()), nil
	}
	caseSensitive := true
	if v, ok := args["case_sensitive"].(bool); ok {
		caseSensitive = v
	}
	maxResults := IntArg(args, "max_results", 200)

	matches := proj.SearchSubstring(query, caseSensitive, maxResults)
	if len(matches) == 0 {
		return OKMarkdown(fmt.Sprintf("# Search: `%s`\n\n_No matches found._\n", query)), nil
	}
	var rows [][]string
	for _, m := range matches {
		rows = append(rows, []string{loc(m.File, m.Line), "`" + truncate(m.Text, 120) + "`"})
	}
	var md strings.Builder
	fmt.Fprintf(&md, "# Search: `%s` (%d matches)\n\n", query, len(matches))
	if len(matches) >= maxResults {
		fmt.Fprintf(&md, "_Showing first %d results._\n\n", maxResults)
	}
	md.WriteString(table([]string{"Location", "Content"}, rows))
	return OKMarkdown(md.String()), nil
}
