package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"go-mcp-server/internal/project"
)

type ProjectFilesTool struct {
	base BaseGoTool
}

func NewProjectFilesTool(manager *project.Manager) *ProjectFilesTool {
	return &ProjectFilesTool{base: NewBaseGoTool(manager)}
}

func (t *ProjectFilesTool) Name() string { return "project_files" }

func (t *ProjectFilesTool) Description() string {
	return "Lists all .go files in a loaded project, sorted by relative path."
}

func (t *ProjectFilesTool) InputSchema() map[string]any { return nameOnlySchema() }

func (t *ProjectFilesTool) Call(_ context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	_, proj, err := findAndAnalyze(t.base, args)
	if err != nil {
		return OKError(err.Error()), nil
	}
	files := proj.ListFiles()
	if len(files) == 0 {
		return OKMarkdown("# Project Files\n\n_No files found._\n"), nil
	}
	var rows [][]string
	for _, f := range files {
		rows = append(rows, []string{"`" + f + "`"})
	}
	var md strings.Builder
	fmt.Fprintf(&md, "# Project Files (%d)\n\n", len(files))
	md.WriteString(table([]string{"Path"}, rows))
	return OKMarkdown(md.String()), nil
}
