// list_packages.go — project packages (directories with .go files).
package tools

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"go-mcp-server/internal/project"
)

// ListPackagesTool implementa Tool.
type ListPackagesTool struct {
	base BaseGoTool
}

// NewListPackagesTool builds the tool.
func NewListPackagesTool(manager *project.Manager) *ListPackagesTool {
	return &ListPackagesTool{base: NewBaseGoTool(manager)}
}

// Name returns the stable tool name.
func (t *ListPackagesTool) Name() string { return "list_packages" }

// Description returns the LLM-readable description.
func (t *ListPackagesTool) Description() string {
	return "Lists all packages in a loaded Go project (directories containing .go files, " +
		"excluding vendor/, testdata/, hidden and _ dirs). Output is Markdown, sorted by directory."
}

// InputSchema: name required.
func (t *ListPackagesTool) InputSchema() map[string]any {
	return nameOnlySchema()
}

// Call lists the packages.
func (t *ListPackagesTool) Call(_ context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	_, proj, err := findAndAnalyze(t.base, args)
	if err != nil {
		return OKError(err.Error()), nil
	}
	pkgs := proj.Packages()
	if len(pkgs) == 0 {
		return OKMarkdown("# Packages\n\n_No packages found._\n"), nil
	}
	var rows [][]string
	for _, p := range pkgs {
		rows = append(rows, []string{"`" + p.Name + "`", "`" + p.Dir + "`", fmt.Sprintf("%d", p.Files)})
	}
	md := fmt.Sprintf("# Packages (%d)\n\n", len(pkgs)) +
		table([]string{"Package", "Dir", "Files"}, rows)
	return OKMarkdown(md), nil
}
