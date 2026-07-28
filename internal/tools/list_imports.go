// list_imports.go — imports from all project files.
package tools

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"go-mcp-server/internal/project"
)

// ListImportsTool implementa Tool.
type ListImportsTool struct {
	base BaseGoTool
}

// NewListImportsTool builds the tool.
func NewListImportsTool(manager *project.Manager) *ListImportsTool {
	return &ListImportsTool{base: NewBaseGoTool(manager)}
}

// Name returns the stable tool name.
func (t *ListImportsTool) Name() string { return "list_imports" }

// Description returns the LLM-readable description.
func (t *ListImportsTool) Description() string {
	return "Lists all imports across all files in a loaded project: import path, alias " +
		"(if any: dot, blank, or named), and file location. Sorted by file then path."
}

// InputSchema: name required.
func (t *ListImportsTool) InputSchema() map[string]any {
	return nameOnlySchema()
}

// Call lists the imports.
func (t *ListImportsTool) Call(_ context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	_, proj, err := findAndAnalyze(t.base, args)
	if err != nil {
		return OKError(err.Error()), nil
	}
	imports := proj.Imports()
	if len(imports) == 0 {
		return OKMarkdown("# Imports\n\n_No imports found._\n"), nil
	}
	var rows [][]string
	for _, imp := range imports {
		rows = append(rows, []string{
			"`" + imp.File + "`", "`" + imp.Path + "`", dash(imp.Alias),
		})
	}
	md := fmt.Sprintf("# Imports (%d)\n\n", len(imports)) +
		table([]string{"File", "Import", "Alias"}, rows)
	return OKMarkdown(md), nil
}
