package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"go-mcp-server/internal/project"
)

type StructFieldXRefTool struct {
	base BaseGoTool
}

func NewStructFieldXRefTool(manager *project.Manager) *StructFieldXRefTool {
	return &StructFieldXRefTool{base: NewBaseGoTool(manager)}
}

func (t *StructFieldXRefTool) Name() string { return "struct_field_xref" }

func (t *StructFieldXRefTool) Description() string {
	return "Cross-references a struct field by name: shows field declarations and all " +
		"references across the project, using go/types semantic analysis. " +
		"If multiple structs have a field with that name, all are reported."
}

func (t *StructFieldXRefTool) InputSchema() map[string]any {
	s := nameOnlySchema()
	props := s["properties"].(map[string]any)
	props["field_name"] = map[string]any{
		"type":        "string",
		"description": "Field name to cross-reference",
	}
	s["required"] = []string{"name", "field_name"}
	return s
}

func (t *StructFieldXRefTool) Call(_ context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	_, proj, err := findAndAnalyzeSemantic(t.base, args)
	if err != nil {
		return OKError(err.Error()), nil
	}
	fieldName, err := Arg(args, "field_name")
	if err != nil {
		return OKError(err.Error()), nil
	}

	decls, refs := proj.FieldXRef(fieldName)
	if len(decls) == 0 {
		return OKMarkdown(fmt.Sprintf(
			"# Field XRef: `%s`\n\n_No field declarations found._\n", fieldName)), nil
	}

	var md strings.Builder
	fmt.Fprintf(&md, "# Field XRef: `%s`\n\n", fieldName)

	// Declarations
	fmt.Fprintf(&md, "## Declarations (%d)\n\n", len(decls))
	var dRows [][]string
	for _, d := range decls {
		dRows = append(dRows, []string{
			"`" + d.StructName + "`",
			d.Position,
			"`" + d.Context + "`",
		})
	}
	md.WriteString(table([]string{"Struct", "Location", "Declaration"}, dRows))

	// References
	if len(refs) > 0 {
		fmt.Fprintf(&md, "\n## References (%d)\n\n", len(refs))
		var rRows [][]string
		for _, r := range refs {
			rRows = append(rRows, []string{
				"`" + r.StructName + "`",
				r.Position,
				"`" + r.Context + "`",
			})
		}
		md.WriteString(table([]string{"Function", "Location", "Context"}, rRows))
	} else {
		md.WriteString("\n## References\n\n_No references found._\n")
	}

	return OKMarkdown(md.String()), nil
}
