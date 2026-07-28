package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"go-mcp-server/internal/project"
)

type FindGoroutineLeaksTool struct {
	base BaseGoTool
}

func NewFindGoroutineLeaksTool(manager *project.Manager) *FindGoroutineLeaksTool {
	return &FindGoroutineLeaksTool{base: NewBaseGoTool(manager)}
}

func (t *FindGoroutineLeaksTool) Name() string { return "find_goroutine_leaks" }

func (t *FindGoroutineLeaksTool) Description() string {
	return "Finds goroutines (go f()) whose body lacks synchronization mechanisms " +
		"(channels, WaitGroup, mutex, atomic). Heuristic — may produce false positives."
}

func (t *FindGoroutineLeaksTool) InputSchema() map[string]any { return nameOnlySchema() }

func (t *FindGoroutineLeaksTool) Call(_ context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	_, proj, err := findAndAnalyzeSemantic(t.base, args)
	if err != nil {
		return OKError(err.Error()), nil
	}

	findings := proj.FindGoroutineLeaks()
	if len(findings) == 0 {
		return OKMarkdown("# Goroutine Leaks\n\n_No potential goroutine leaks found._\n"), nil
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
	fmt.Fprintf(&md, "# Goroutine Leaks (%d)\n\n", len(findings))
	if len(findings) > 0 {
		md.WriteString("_Heuristic — review each finding._\n\n")
	}
	md.WriteString(table([]string{"Severity", "Pattern", "Location", "Context"}, rows))
	return OKMarkdown(md.String()), nil
}
