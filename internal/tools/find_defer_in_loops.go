package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"go-mcp-server/internal/project"
)

type FindDeferInLoopsTool struct {
	base BaseGoTool
}

func NewFindDeferInLoopsTool(manager *project.Manager) *FindDeferInLoopsTool {
	return &FindDeferInLoopsTool{base: NewBaseGoTool(manager)}
}

func (t *FindDeferInLoopsTool) Name() string { return "find_defer_in_loops" }

func (t *FindDeferInLoopsTool) Description() string {
	return "Finds defer statements inside for/range loops. These are likely bugs: " +
		"deferred resources accumulate until the function returns instead of being " +
		"released per iteration. Static analysis via go/parser."
}

func (t *FindDeferInLoopsTool) InputSchema() map[string]any { return nameOnlySchema() }

func (t *FindDeferInLoopsTool) Call(_ context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	_, proj, err := findAndAnalyze(t.base, args)
	if err != nil {
		return OKError(err.Error()), nil
	}

	sites := proj.FindDeferInLoops()
	if len(sites) == 0 {
		return OKMarkdown("# Defer in Loops\n\n_No defer-in-loop occurrences found._\n"), nil
	}

	var rows [][]string
	for _, s := range sites {
		rows = append(rows, []string{
			"`" + s.Severity + "`",
			"`" + s.Message + "`",
			loc(s.File, s.Line),
			"`" + s.Context + "`",
		})
	}
	var md strings.Builder
	fmt.Fprintf(&md, "# Defer in Loops (%d)\n\n", len(sites))
	md.WriteString(table([]string{"Severity", "Pattern", "Location", "Context"}, rows))
	return OKMarkdown(md.String()), nil
}
