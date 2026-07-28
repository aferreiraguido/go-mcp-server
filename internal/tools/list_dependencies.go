// list_dependencies.go — go.mod requires (+ replace if present).
package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"go-mcp-server/internal/project"
)

// ListDependenciesTool implementa Tool.
type ListDependenciesTool struct {
	base BaseGoTool
}

// NewListDependenciesTool builds the tool.
func NewListDependenciesTool(manager *project.Manager) *ListDependenciesTool {
	return &ListDependenciesTool{base: NewBaseGoTool(manager)}
}

// Name returns the stable tool name.
func (t *ListDependenciesTool) Name() string { return "list_dependencies" }

// Description returns the LLM-readable description.
func (t *ListDependenciesTool) Description() string {
	return "Lists direct dependencies declared in go.mod (require directives), with " +
		"indirect flag, plus replace directives if present. Only the static manifest is " +
		"read — transitive deps are not computed (use `go mod graph` for that)."
}

// InputSchema: name required.
func (t *ListDependenciesTool) InputSchema() map[string]any {
	return nameOnlySchema()
}

// Call lists the dependencies.
func (t *ListDependenciesTool) Call(_ context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	_, proj, err := findAndAnalyze(t.base, args)
	if err != nil {
		return OKError(err.Error()), nil
	}
	deps := proj.Dependencies()

	var md strings.Builder
	if len(deps) == 0 {
		md.WriteString("# Dependencies\n\n_No dependencies declared in go.mod._\n")
	} else {
		var rows [][]string
		for _, d := range deps {
			rows = append(rows, []string{"`" + d.Path + "`", "`" + d.Version + "`", yesNo(d.Indirect)})
		}
		fmt.Fprintf(&md, "# Dependencies (%d)\n\n", len(deps))
		md.WriteString(table([]string{"Module", "Version", "Indirect"}, rows))
	}

	if reps := proj.Replaces(); len(reps) > 0 {
		fmt.Fprintf(&md, "\n## Replace (%d)\n\n", len(reps))
		for _, r := range reps {
			md.WriteString("- `" + r + "`\n")
		}
	}
	return OKMarkdown(md.String()), nil
}
