// program_summary.go — executive summary of the project (counts).
// Counterpart of `ProgramSummaryTool` (rust-mcp-server): recommended first-call
// to understand a project's scope.
package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"go-mcp-server/internal/analysis"
	"go-mcp-server/internal/project"
)

// ProgramSummaryTool implementa Tool.
type ProgramSummaryTool struct {
	base BaseGoTool
}

// NewProgramSummaryTool builds the tool.
func NewProgramSummaryTool(manager *project.Manager) *ProgramSummaryTool {
	return &ProgramSummaryTool{base: NewBaseGoTool(manager)}
}

// Name returns the stable tool name.
func (t *ProgramSummaryTool) Name() string { return "program_summary" }

// Description returns the LLM-readable description.
func (t *ProgramSummaryTool) Description() string {
	return "Executive summary of a loaded Go project: module path, Go version, package/file/line " +
		"counts, function/method/struct/interface/const/var counts, import and dependency counts. " +
		"Useful as a first-call to understand a project's scope."
}

// InputSchema: name required.
func (t *ProgramSummaryTool) InputSchema() map[string]any {
	return nameOnlySchema()
}

// Call builds the summary.
func (t *ProgramSummaryTool) Call(_ context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	entry, proj, err := findAndAnalyze(t.base, args)
	if err != nil {
		return OKError(err.Error()), nil
	}

	funcs := proj.Functions()
	methods := 0
	for _, f := range funcs {
		if f.Receiver != "" {
			methods++
		}
	}
	uniqueImports := map[string]bool{}
	for _, imp := range proj.Imports() {
		uniqueImports[imp.Path] = true
	}

	rows := [][]string{
		{"Module", "`" + dash(entry.ModulePath) + "`"},
		{"Go version", dash(entry.GoVersion)},
		{"Kind", entry.Kind.String()},
		{"Packages", fmt.Sprintf("%d", len(proj.Packages()))},
		{"Files (.go)", fmt.Sprintf("%d", len(proj.Files))},
		{"Lines", fmt.Sprintf("%d", proj.TotalLines())},
		{"Functions", fmt.Sprintf("%d", len(funcs)-methods)},
		{"Methods", fmt.Sprintf("%d", methods)},
		{"Structs", fmt.Sprintf("%d", len(proj.Structs()))},
		{"Interfaces", fmt.Sprintf("%d", len(proj.Interfaces()))},
		{"Constants", fmt.Sprintf("%d", len(proj.Constants()))},
		{"Variables", fmt.Sprintf("%d", len(proj.Variables()))},
		{"Imports (unique)", fmt.Sprintf("%d", len(uniqueImports))},
		{"Dependencies (go.mod)", fmt.Sprintf("%d", len(proj.Dependencies()))},
	}

	var md strings.Builder
	md.WriteString("# Program Summary\n\n")
	md.WriteString(table([]string{"Metric", "Value"}, rows))

	if eps := entryPointsOf(proj, "main"); len(eps) > 0 {
		md.WriteString("\n## Entry Points\n\n")
		for _, ep := range eps {
			fmt.Fprintf(&md, "- `main` in package `%s` at %s\n", ep.Package, loc(ep.File, ep.Line))
		}
	}
	return OKMarkdown(md.String()), nil
}

// entryPointsOf filtra entry points por kind.
func entryPointsOf(proj *analysis.Project, kind string) []analysis.EntryPoint {
	var out []analysis.EntryPoint
	for _, ep := range proj.EntryPoints() {
		if ep.Kind == kind {
			out = append(out, ep)
		}
	}
	return out
}
