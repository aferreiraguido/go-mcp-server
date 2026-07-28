package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"go-mcp-server/internal/project"
)

type ListDirectoriesTool struct {
	base BaseGoTool
}

func NewListDirectoriesTool(manager *project.Manager) *ListDirectoriesTool {
	return &ListDirectoriesTool{base: NewBaseGoTool(manager)}
}

func (t *ListDirectoriesTool) Name() string { return "list_directories" }

func (t *ListDirectoriesTool) Description() string {
	return "Lists all directories containing .go files in a loaded project."
}

func (t *ListDirectoriesTool) InputSchema() map[string]any { return nameOnlySchema() }

func (t *ListDirectoriesTool) Call(_ context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	_, proj, err := findAndAnalyze(t.base, args)
	if err != nil {
		return OKError(err.Error()), nil
	}
	dirs := proj.ListDirectories()
	if len(dirs) == 0 {
		return OKMarkdown("# Directories\n\n_No directories found._\n"), nil
	}
	var rows [][]string
	for _, d := range dirs {
		rows = append(rows, []string{"`" + d + "`"})
	}
	var md strings.Builder
	fmt.Fprintf(&md, "# Directories (%d)\n\n", len(dirs))
	md.WriteString(table([]string{"Path"}, rows))
	return OKMarkdown(md.String()), nil
}
