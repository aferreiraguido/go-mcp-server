package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"go-mcp-server/internal/project"
)

type FindUnusedParametersTool struct {
	base BaseGoTool
}

func NewFindUnusedParametersTool(manager *project.Manager) *FindUnusedParametersTool {
	return &FindUnusedParametersTool{base: NewBaseGoTool(manager)}
}

func (t *FindUnusedParametersTool) Name() string { return "find_unused_parameters" }

func (t *FindUnusedParametersTool) Description() string {
	return "Finds function parameters that are declared but never used in the function body. " +
		"Static analysis via go/parser."
}

func (t *FindUnusedParametersTool) InputSchema() map[string]any { return nameOnlySchema() }

func (t *FindUnusedParametersTool) Call(_ context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	_, proj, err := findAndAnalyze(t.base, args)
	if err != nil {
		return OKError(err.Error()), nil
	}

	findings := proj.FindUnusedParameters()
	if len(findings) == 0 {
		return OKMarkdown("# Unused Parameters\n\n_No unused parameters found._\n"), nil
	}

	var rows [][]string
	for _, f := range findings {
		rows = append(rows, []string{
			"`" + f.Message + "`",
			loc(f.File, f.Line),
			"`" + f.Context + "`",
		})
	}
	var md strings.Builder
	fmt.Fprintf(&md, "# Unused Parameters (%d)\n\n", len(findings))
	md.WriteString(table([]string{"Parameter", "Location", "Context"}, rows))
	return OKMarkdown(md.String()), nil
}
