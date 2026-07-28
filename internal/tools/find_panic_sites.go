package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"go-mcp-server/internal/project"
)

type FindPanicSitesTool struct {
	base BaseGoTool
}

func NewFindPanicSitesTool(manager *project.Manager) *FindPanicSitesTool {
	return &FindPanicSitesTool{base: NewBaseGoTool(manager)}
}

func (t *FindPanicSitesTool) Name() string { return "find_panic_sites" }

func (t *FindPanicSitesTool) Description() string {
	return "Finds panic(), log.Fatal(), log.Panic(), and os.Exit() calls across the project. " +
		"Static analysis via go/parser."
}

func (t *FindPanicSitesTool) InputSchema() map[string]any { return nameOnlySchema() }

func (t *FindPanicSitesTool) Call(_ context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	_, proj, err := findAndAnalyze(t.base, args)
	if err != nil {
		return OKError(err.Error()), nil
	}

	sites := proj.FindPanicSites()
	if len(sites) == 0 {
		return OKMarkdown("# Panic Sites\n\n_No panic sites found._\n"), nil
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
	fmt.Fprintf(&md, "# Panic Sites (%d)\n\n", len(sites))
	md.WriteString(table([]string{"Severity", "Pattern", "Location", "Context"}, rows))
	return OKMarkdown(md.String()), nil
}
