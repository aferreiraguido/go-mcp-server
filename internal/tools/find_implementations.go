package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"go-mcp-server/internal/project"
)

type FindImplementationsTool struct {
	base BaseGoTool
}

func NewFindImplementationsTool(manager *project.Manager) *FindImplementationsTool {
	return &FindImplementationsTool{base: NewBaseGoTool(manager)}
}

func (t *FindImplementationsTool) Name() string { return "find_implementations" }

func (t *FindImplementationsTool) Description() string {
	return "Returns the concrete types that satisfy a given interface in the project, " +
		"using go/types semantic analysis. In Go, interface satisfaction is implicit — " +
		"this is the signature tool of the semantic layer."
}

func (t *FindImplementationsTool) InputSchema() map[string]any {
	s := nameOnlySchema()
	props := s["properties"].(map[string]any)
	props["interface_name"] = map[string]any{
		"type":        "string",
		"description": "Interface name to find implementations for",
	}
	s["required"] = []string{"name", "interface_name"}
	return s
}

func (t *FindImplementationsTool) Call(_ context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	_, proj, err := findAndAnalyzeSemantic(t.base, args)
	if err != nil {
		return OKError(err.Error()), nil
	}
	ifaceName, err := Arg(args, "interface_name")
	if err != nil {
		return OKError(err.Error()), nil
	}

	impls, ok, reason := proj.Implementations(ifaceName)
	if !ok {
		return OKMarkdown(fmt.Sprintf("# Implementations: `%s`\n\n%s\n", ifaceName, reason)), nil
	}
	if len(impls) == 0 {
		return OKMarkdown(fmt.Sprintf(
			"# Implementations: `%s`\n\n_No types implement this interface._\n", ifaceName)), nil
	}

	var md strings.Builder
	fmt.Fprintf(&md, "# Implementations: `%s` (%d)\n\n", ifaceName, len(impls))
	var rows [][]string
	for _, impl := range impls {
		rows = append(rows, []string{
			"`" + impl.TypeName + "`",
			"`" + impl.Via + "`",
			impl.Position,
		})
	}
	md.WriteString(table([]string{"Type", "Via", "Location"}, rows))
	return OKMarkdown(md.String()), nil
}
