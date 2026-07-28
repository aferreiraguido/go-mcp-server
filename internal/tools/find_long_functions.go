package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"go-mcp-server/internal/project"
)

type FindLongFunctionsTool struct {
	base BaseGoTool
}

func NewFindLongFunctionsTool(manager *project.Manager) *FindLongFunctionsTool {
	return &FindLongFunctionsTool{base: NewBaseGoTool(manager)}
}

func (t *FindLongFunctionsTool) Name() string { return "find_long_functions" }

func (t *FindLongFunctionsTool) Description() string {
	return "Reports functions whose body exceeds a configurable line count threshold. " +
		"Optional parameter: max_lines (default 80)."
}

func (t *FindLongFunctionsTool) InputSchema() map[string]any {
	s := nameOnlySchema()
	props := s["properties"].(map[string]any)
	props["max_lines"] = map[string]any{
		"type":        "number",
		"description": "Maximum allowed lines in a function body (default: 80)",
	}
	return s
}

func (t *FindLongFunctionsTool) Call(_ context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	_, proj, err := findAndAnalyze(t.base, args)
	if err != nil {
		return OKError(err.Error()), nil
	}
	maxLines := IntArg(args, "max_lines", 80)

	metrics := proj.FindLongFunctions(maxLines)
	if len(metrics) == 0 {
		return OKMarkdown("# Long Functions\n\n_No functions exceed "+fmt.Sprint(maxLines)+" lines._\n"), nil
	}

	var rows [][]string
	for _, m := range metrics {
		rows = append(rows, []string{
			"`" + m.Name + "`",
			fmt.Sprintf("%d", m.Lines),
			m.Location,
		})
	}
	var md strings.Builder
	fmt.Fprintf(&md, "# Long Functions (%d, threshold %d lines)\n\n", len(metrics), maxLines)
	md.WriteString(table([]string{"Function", "Lines", "Location"}, rows))
	return OKMarkdown(md.String()), nil
}
