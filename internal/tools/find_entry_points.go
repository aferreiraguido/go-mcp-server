// find_entry_points.go — entry points: func main() and func init().
// Counterpart of `find_entry_points` (python-mcp-server).
package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"go-mcp-server/internal/project"
)

// FindEntryPointsTool implementa Tool.
type FindEntryPointsTool struct {
	base BaseGoTool
}

// NewFindEntryPointsTool builds the tool.
func NewFindEntryPointsTool(manager *project.Manager) *FindEntryPointsTool {
	return &FindEntryPointsTool{base: NewBaseGoTool(manager)}
}

// Name returns the stable tool name.
func (t *FindEntryPointsTool) Name() string { return "find_entry_points" }

// Description returns the LLM-readable description.
func (t *FindEntryPointsTool) Description() string {
	return "Finds where the program starts execution: func main() in main packages, " +
		"and all func init() (package initialization hooks)."
}

// InputSchema: name required.
func (t *FindEntryPointsTool) InputSchema() map[string]any {
	return nameOnlySchema()
}

// Call lists the entry points.
func (t *FindEntryPointsTool) Call(_ context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	_, proj, err := findAndAnalyze(t.base, args)
	if err != nil {
		return OKError(err.Error()), nil
	}
	eps := proj.EntryPoints()
	if len(eps) == 0 {
		return OKMarkdown("# Entry Points\n\n_No entry points found (library-only project?)._\n"), nil
	}

	var mains, inits [][]string
	for _, ep := range eps {
		row := []string{"`" + ep.Package + "`", loc(ep.File, ep.Line)}
		if ep.Kind == "main" {
			mains = append(mains, row)
		} else {
			inits = append(inits, row)
		}
	}

	var md strings.Builder
	fmt.Fprintf(&md, "# Entry Points (%d)\n\n", len(eps))
	md.WriteString("## func main()\n\n")
	if len(mains) == 0 {
		md.WriteString("_No main packages._\n")
	} else {
		md.WriteString(table([]string{"Package", "Location"}, mains))
	}
	md.WriteString("\n## func init()\n\n")
	if len(inits) == 0 {
		md.WriteString("_No init functions._\n")
	} else {
		md.WriteString(table([]string{"Package", "Location"}, inits))
	}
	return OKMarkdown(md.String()), nil
}
