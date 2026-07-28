package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"go-mcp-server/internal/project"
)

type ListAPIEndpointsTool struct {
	base BaseGoTool
}

func NewListAPIEndpointsTool(manager *project.Manager) *ListAPIEndpointsTool {
	return &ListAPIEndpointsTool{base: NewBaseGoTool(manager)}
}

func (t *ListAPIEndpointsTool) Name() string { return "list_api_endpoints" }

func (t *ListAPIEndpointsTool) Description() string {
	return "Lists HTTP API route definitions found in the project. Detects patterns " +
		"like r.GET(), r.POST(), router.Handle(), etc. from popular Go web frameworks " +
		"(gin, echo, chi, gorilla/mux, net/http)."
}

func (t *ListAPIEndpointsTool) InputSchema() map[string]any { return nameOnlySchema() }

func (t *ListAPIEndpointsTool) Call(_ context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	_, proj, err := findAndAnalyze(t.base, args)
	if err != nil {
		return OKError(err.Error()), nil
	}

	endpoints := proj.ListAPIEndpoints()
	if len(endpoints) == 0 {
		return OKMarkdown("# API Endpoints\n\n_No API endpoints detected._\n"), nil
	}

	var rows [][]string
	for _, ep := range endpoints {
		rows = append(rows, []string{
			"`" + ep.Method + "`",
			"`" + ep.Path + "`",
			"`" + ep.Handler + "`",
			loc(ep.File, ep.Line),
		})
	}
	var md strings.Builder
	fmt.Fprintf(&md, "# API Endpoints (%d)\n\n", len(endpoints))
	md.WriteString(table([]string{"Method", "Path", "Handler", "Location"}, rows))
	return OKMarkdown(md.String()), nil
}
