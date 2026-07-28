package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"go-mcp-server/internal/project"
)

type FindDeadCodeTool struct {
	base BaseGoTool
}

func NewFindDeadCodeTool(manager *project.Manager) *FindDeadCodeTool {
	return &FindDeadCodeTool{base: NewBaseGoTool(manager)}
}

func (t *FindDeadCodeTool) Name() string { return "find_dead_code" }

func (t *FindDeadCodeTool) Description() string {
	return "Finds non-exported functions, types, constants, and variables that are never " +
		"referenced elsewhere in the project. Uses go/types Uses index. " +
		"Only unexported symbols are reported (exported ones may be used externally)."
}

func (t *FindDeadCodeTool) InputSchema() map[string]any { return nameOnlySchema() }

func (t *FindDeadCodeTool) Call(_ context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	_, proj, err := findAndAnalyzeSemantic(t.base, args)
	if err != nil {
		return OKError(err.Error()), nil
	}

	findings := proj.FindDeadCode()
	if len(findings) == 0 {
		return OKMarkdown("# Dead Code\n\n_No dead code found._\n"), nil
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
	fmt.Fprintf(&md, "# Dead Code (%d)\n\n", len(findings))
	md.WriteString(table([]string{"Severity", "Pattern", "Location", "Context"}, rows))
	return OKMarkdown(md.String()), nil
}
