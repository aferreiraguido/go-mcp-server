// get_file_content.go — raw content of a project file.
// Counterpart of `GetFileContentTool` (rust-mcp-server).
//
// Security: uses os.OpenRoot + filepath.IsLocal — rel_path CANNOT
// escape from the project root (path traversal blocked by design,
// lesson from rust iter 6+).
package tools

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"go-mcp-server/internal/project"
)

// GetFileContentTool implementa Tool.
type GetFileContentTool struct {
	base BaseGoTool
}

// NewGetFileContentTool builds the tool.
func NewGetFileContentTool(manager *project.Manager) *GetFileContentTool {
	return &GetFileContentTool{base: NewBaseGoTool(manager)}
}

// Name returns the stable tool name.
func (t *GetFileContentTool) Name() string { return "get_file_content" }

// Description returns the LLM-readable description.
func (t *GetFileContentTool) Description() string {
	return "Returns the raw content of a file inside a loaded project. rel_path is relative " +
		"to the project root and must not escape it. Output is wrapped in a code block with " +
		"the appropriate language."
}

// InputSchema: name + rel_path required; max_lines optional.
func (t *GetFileContentTool) InputSchema() map[string]any {
	s := nameOnlySchema()
	props := s["properties"].(map[string]any)
	props["rel_path"] = map[string]any{
		"type":        "string",
		"description": "Path relative to the project root (e.g. 'internal/foo/bar.go')",
	}
	props["max_lines"] = map[string]any{
		"type":        "integer",
		"description": "Maximum number of lines returned (default 2000)",
	}
	s["required"] = []string{"name", "rel_path"}
	return s
}

// Call reads the file under the project root.
func (t *GetFileContentTool) Call(_ context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	entry, err := t.base.FindEntry(OptArg(args, "name"))
	if err != nil {
		return OKError(err.Error()), nil
	}
	relPath, err := Arg(args, "rel_path")
	if err != nil {
		return OKError(err.Error()), nil
	}
	maxLines := IntArg(args, "max_lines", 2000)

	if !filepath.IsLocal(relPath) {
		return OKError(fmt.Sprintf(
			"path '%s' is not local: it must be relative and must not escape the project root",
			relPath)), nil
	}

	root, err := os.OpenRoot(entry.Path)
	if err != nil {
		return OKError("cannot open project root: " + err.Error()), nil
	}
	defer root.Close()

	f, err := root.Open(relPath)
	if err != nil {
		return OKError(fmt.Sprintf("file '%s' not found in project '%s': %v",
			relPath, entry.EffectiveName(), err)), nil
	}
	defer f.Close()

	src, err := io.ReadAll(f)
	if err != nil {
		return OKError("cannot read file: " + err.Error()), nil
	}

	lines := strings.Split(string(src), "\n")
	truncated := false
	if len(lines) > maxLines {
		lines = lines[:maxLines]
		truncated = true
	}

	lang := ""
	if strings.HasSuffix(relPath, ".go") {
		lang = "go"
	}

	var md strings.Builder
	md.WriteString("# " + relPath + "\n\n")
	if truncated {
		fmt.Fprintf(&md, "_Truncated to %d lines._\n\n", maxLines)
	}
	md.WriteString(codeBlock(lang, strings.Join(lines, "\n")))
	return OKMarkdown(md.String()), nil
}
