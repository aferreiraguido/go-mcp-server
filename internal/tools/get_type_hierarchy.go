package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"go-mcp-server/internal/project"
)

type GetTypeHierarchyTool struct {
	base BaseGoTool
}

func NewGetTypeHierarchyTool(manager *project.Manager) *GetTypeHierarchyTool {
	return &GetTypeHierarchyTool{base: NewBaseGoTool(manager)}
}

func (t *GetTypeHierarchyTool) Name() string { return "get_type_hierarchy" }

func (t *GetTypeHierarchyTool) Description() string {
	return "Returns the type hierarchy for a symbol: for structs, shows embedded and " +
		"embedding types; for interfaces, shows embedded interfaces and implementing " +
		"types. Uses go/types semantic analysis."
}

func (t *GetTypeHierarchyTool) InputSchema() map[string]any {
	s := nameOnlySchema()
	props := s["properties"].(map[string]any)
	props["type_name"] = map[string]any{
		"type":        "string",
		"description": "Type name to inspect hierarchy for",
	}
	s["required"] = []string{"name", "type_name"}
	return s
}

func (t *GetTypeHierarchyTool) Call(_ context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	_, proj, err := findAndAnalyzeSemantic(t.base, args)
	if err != nil {
		return OKError(err.Error()), nil
	}
	typeName, err := Arg(args, "type_name")
	if err != nil {
		return OKError(err.Error()), nil
	}

	h := proj.Hierarchy(typeName)

	var md strings.Builder
	fmt.Fprintf(&md, "# Hierarchy: `%s`\n\n", typeName)
	fmt.Fprintf(&md, "**Kind:** %s\n\n", h.Kind)

	// Embeds (upward: what this type embeds)
	if len(h.Embeds) > 0 {
		fmt.Fprintf(&md, "## Embeds (%d)\n\n", len(h.Embeds))
		var rows [][]string
		for _, e := range h.Embeds {
			rows = append(rows, []string{"`" + e.TypeName + "`", e.Position})
		}
		md.WriteString(table([]string{"Type", "Location"}, rows))
	}

	// EmbeddedBy (downward: what embeds this type)
	if len(h.EmbeddedBy) > 0 {
		fmt.Fprintf(&md, "\n## Embedded by (%d)\n\n", len(h.EmbeddedBy))
		var rows [][]string
		for _, e := range h.EmbeddedBy {
			rows = append(rows, []string{"`" + e.TypeName + "`", e.Position})
		}
		md.WriteString(table([]string{"Type", "Location"}, rows))
	}

	// Implementations (for interfaces)
	if len(h.Impls) > 0 {
		fmt.Fprintf(&md, "\n## Implementations (%d)\n\n", len(h.Impls))
		var rows [][]string
		for _, im := range h.Impls {
			rows = append(rows, []string{
				"`" + im.TypeName + "`",
				"`" + im.Via + "`",
				im.Position,
			})
		}
		md.WriteString(table([]string{"Type", "Via", "Location"}, rows))
	}

	if len(h.Embeds) == 0 && len(h.EmbeddedBy) == 0 && len(h.Impls) == 0 {
		md.WriteString("_No hierarchy information._\n")
	}

	return OKMarkdown(md.String()), nil
}
