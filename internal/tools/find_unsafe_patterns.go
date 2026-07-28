package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"go-mcp-server/internal/project"
)

type FindUnsafePatternsTool struct {
	base BaseGoTool
}

func NewFindUnsafePatternsTool(manager *project.Manager) *FindUnsafePatternsTool {
	return &FindUnsafePatternsTool{base: NewBaseGoTool(manager)}
}

func (t *FindUnsafePatternsTool) Name() string { return "find_unsafe_patterns" }

func (t *FindUnsafePatternsTool) Description() string {
	return "Finds unsafe operations (unsafe.Pointer, unsafe.Sizeof, etc.) and unchecked " +
		"memory patterns in the project. These bypass Go's safety guarantees. " +
		"Static analysis via go/parser."
}

func (t *FindUnsafePatternsTool) InputSchema() map[string]any { return nameOnlySchema() }

func (t *FindUnsafePatternsTool) Call(_ context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	_, proj, err := findAndAnalyze(t.base, args)
	if err != nil {
		return OKError(err.Error()), nil
	}

	sites := proj.FindUnsafeUsage()
	if len(sites) == 0 {
		return OKMarkdown("# Unsafe Patterns\n\n_No unsafe usage found._\n"), nil
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
	fmt.Fprintf(&md, "# Unsafe Patterns (%d)\n\n", len(sites))
	md.WriteString(table([]string{"Severity", "Pattern", "Location", "Context"}, rows))
	return OKMarkdown(md.String()), nil
}
