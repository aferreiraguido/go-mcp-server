package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"go-mcp-server/internal/project"
)

type DetectFrameworkTool struct {
	base BaseGoTool
}

func NewDetectFrameworkTool(manager *project.Manager) *DetectFrameworkTool {
	return &DetectFrameworkTool{base: NewBaseGoTool(manager)}
}

func (t *DetectFrameworkTool) Name() string { return "detect_framework" }

func (t *DetectFrameworkTool) Description() string {
	return "Detects web frameworks and common libraries used in a Go project. " +
		"Scans go.mod and source imports for known frameworks (gin, echo, cobra, chi, " +
		"gorilla/mux, fiber, grpc, net/http, etc.)."
}

func (t *DetectFrameworkTool) InputSchema() map[string]any { return nameOnlySchema() }

func (t *DetectFrameworkTool) Call(_ context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	_, proj, err := findAndAnalyze(t.base, args)
	if err != nil {
		return OKError(err.Error()), nil
	}

	fws := proj.DetectFrameworks()
	if len(fws) == 0 {
		return OKMarkdown("# Detected Frameworks\n\n_No frameworks detected._\n"), nil
	}

	var rows [][]string
	for _, f := range fws {
		fwLoc := "-"
		if f.File != "" {
			fwLoc = loc(f.File, f.Line)
		}
		ver := dash(f.Version)
		rows = append(rows, []string{
			"`" + f.Name + "`",
			"`" + ver + "`",
			fwLoc,
		})
	}
	var md strings.Builder
	fmt.Fprintf(&md, "# Detected Frameworks (%d)\n\n", len(fws))
	md.WriteString(table([]string{"Framework", "Version", "Source"}, rows))
	return OKMarkdown(md.String()), nil
}
