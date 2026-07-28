package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"go-mcp-server/internal/project"
)

type FindUncheckedErrorsTool struct {
	base BaseGoTool
}

func NewFindUncheckedErrorsTool(manager *project.Manager) *FindUncheckedErrorsTool {
	return &FindUncheckedErrorsTool{base: NewBaseGoTool(manager)}
}

func (t *FindUncheckedErrorsTool) Name() string { return "find_unchecked_errors" }

func (t *FindUncheckedErrorsTool) Description() string {
	return "Finds function calls that return an error whose result is discarded " +
		"(errcheck-style). Uses go/types to verify the return type is error. " +
		"Semantic analysis via go/packages."
}

func (t *FindUncheckedErrorsTool) InputSchema() map[string]any { return nameOnlySchema() }

func (t *FindUncheckedErrorsTool) Call(_ context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	_, proj, err := findAndAnalyzeSemantic(t.base, args)
	if err != nil {
		return OKError(err.Error()), nil
	}

	findings := proj.FindUncheckedErrors()
	if len(findings) == 0 {
		return OKMarkdown("# Unchecked Errors\n\n_No unchecked errors found._\n"), nil
	}

	var rows [][]string
	for _, f := range findings {
		rows = append(rows, []string{
			"`" + f.Severity + "`",
			"`" + f.Message + "`",
			f.Position,
			"`" + f.Context + "`",
		})
	}
	var md strings.Builder
	fmt.Fprintf(&md, "# Unchecked Errors (%d)\n\n", len(findings))
	md.WriteString(table([]string{"Severity", "Pattern", "Location", "Context"}, rows))
	return OKMarkdown(md.String()), nil
}
