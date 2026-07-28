package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"go-mcp-server/internal/project"
)

type ExtractEnvironmentDependenciesTool struct {
	base BaseGoTool
}

func NewExtractEnvironmentDependenciesTool(manager *project.Manager) *ExtractEnvironmentDependenciesTool {
	return &ExtractEnvironmentDependenciesTool{base: NewBaseGoTool(manager)}
}

func (t *ExtractEnvironmentDependenciesTool) Name() string { return "extract_environment_dependencies" }

func (t *ExtractEnvironmentDependenciesTool) Description() string {
	return "Extracts environment variable reads, flag definitions, and viper config " +
		"accesses from the project. Finds os.Getenv, os.LookupEnv, and viper.Get* patterns."
}

func (t *ExtractEnvironmentDependenciesTool) InputSchema() map[string]any { return nameOnlySchema() }

func (t *ExtractEnvironmentDependenciesTool) Call(_ context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	_, proj, err := findAndAnalyze(t.base, args)
	if err != nil {
		return OKError(err.Error()), nil
	}

	deps := proj.ExtractEnvDependencies()
	if len(deps) == 0 {
		return OKMarkdown("# Environment Dependencies\n\n_No env dependencies detected._\n"), nil
	}

	var rows [][]string
	for _, d := range deps {
		key := dash(d.Key)
		rows = append(rows, []string{
			"`" + d.Kind + "`",
			"`" + key + "`",
			loc(d.File, d.Line),
		})
	}
	var md strings.Builder
	fmt.Fprintf(&md, "# Environment Dependencies (%d)\n\n", len(deps))
	md.WriteString(table([]string{"Kind", "Key", "Location"}, rows))
	return OKMarkdown(md.String()), nil
}
