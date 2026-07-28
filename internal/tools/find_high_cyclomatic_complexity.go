package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"go-mcp-server/internal/project"
)

type FindHighCyclomaticComplexityTool struct {
	base BaseGoTool
}

func NewFindHighCyclomaticComplexityTool(manager *project.Manager) *FindHighCyclomaticComplexityTool {
	return &FindHighCyclomaticComplexityTool{base: NewBaseGoTool(manager)}
}

func (t *FindHighCyclomaticComplexityTool) Name() string { return "find_high_cyclomatic_complexity" }

func (t *FindHighCyclomaticComplexityTool) Description() string {
	return "Reports functions whose cyclomatic complexity exceeds a configurable threshold. " +
		"Optional parameter: max_complexity (default 15)."
}

func (t *FindHighCyclomaticComplexityTool) InputSchema() map[string]any {
	s := nameOnlySchema()
	props := s["properties"].(map[string]any)
	props["max_complexity"] = map[string]any{
		"type":        "number",
		"description": "Maximum allowed cyclomatic complexity (default: 15)",
	}
	return s
}

func (t *FindHighCyclomaticComplexityTool) Call(_ context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	_, proj, err := findAndAnalyze(t.base, args)
	if err != nil {
		return OKError(err.Error()), nil
	}
	maxComplexity := IntArg(args, "max_complexity", 15)

	metrics := proj.FindHighComplexity(maxComplexity)
	if len(metrics) == 0 {
		return OKMarkdown("# High Cyclomatic Complexity\n\n_No functions exceed complexity of "+
			fmt.Sprint(maxComplexity)+"._\n"), nil
	}

	var rows [][]string
	for _, m := range metrics {
		rows = append(rows, []string{
			"`" + m.Name + "`",
			fmt.Sprintf("%d", m.Complexity),
			m.Location,
		})
	}
	var md strings.Builder
	fmt.Fprintf(&md, "# High Cyclomatic Complexity (%d, threshold %d)\n\n", len(metrics), maxComplexity)
	md.WriteString(table([]string{"Function", "Complexity", "Location"}, rows))
	return OKMarkdown(md.String()), nil
}
