// Package analysis is the static analysis engine: parses the project
// with go/parser (stdlib AST) + go.mod/go.work with x/mod/modfile, and
// exposes the query facade consumed by tools.
//
// Counterpart of `analysis/` (rust-mcp-server: syntax.rs + cargo_manifest.rs +
// rust_analyzer_utils.rs). The "stable facade" is the set of methods
// on *Project — equivalent to RustAnalyzerUtils/ProLeapAnalyzerUtils.
package analysis

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/mod/modfile"

	"go-mcp-server/internal/model"
)

// Project is a parsed Go project: ASTs of all .go files + go.mod + go.work.
type Project struct {
	Root       string
	Fset       *token.FileSet
	ModulePath string
	GoVersion  string
	Module     *modfile.File
	Work       *modfile.WorkFile // nil if no go.work
	Files      []*GoFile         // ordenados por RelPath
}

// GoFile is a parsed .go file.
type GoFile struct {
	RelPath string
	AbsPath string
	AST     *ast.File
	Src     []byte
	Lines   []string
}

// LoadProject parses go.mod, go.work (if present) and all .go files in the
// tree (excluding vendor/, testdata/, and hidden/_* dirs, same as the
// go tool). Does NOT type-check or resolve imports — that is iter 3
// (go/packages). Parse errors for an individual file are not
// fatal: the file is skipped.
func LoadProject(root string) (*Project, error) {
	gomodPath := filepath.Join(root, "go.mod")
	gomodSrc, err := os.ReadFile(gomodPath)
	if err != nil {
		return nil, model.NewLoadError(model.ErrGoModMissing,
			"go.mod missing after resolution: "+root)
	}
	mf, err := modfile.Parse(gomodPath, gomodSrc, nil)
	if err != nil {
		return nil, model.NewLoadError(model.ErrGoModMalformed,
			"cannot parse go.mod: "+err.Error())
	}

	p := &Project{
		Root:   root,
		Fset:   token.NewFileSet(),
		Module: mf,
	}
	if mf.Module != nil {
		p.ModulePath = mf.Module.Mod.Path
	}
	if mf.Go != nil {
		p.GoVersion = mf.Go.Version
	}

	// Optional go.work → workspace.
	workSrc, err := os.ReadFile(filepath.Join(root, "go.work"))
	if err == nil {
		if wf, werr := modfile.ParseWork("go.work", workSrc, nil); werr == nil {
			p.Work = wf
		}
	}

	if err := p.parseGoFiles(); err != nil {
		return nil, err
	}
	return p, nil
}

// parseGoFiles walks the tree and parses each .go, in deterministic order.
func (p *Project) parseGoFiles() error {
	var paths []string
	err := filepath.WalkDir(p.Root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != p.Root && skipDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(d.Name(), ".go") {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return model.NewLoadError(model.ErrSourceNotFound,
			"cannot walk project: "+err.Error())
	}
	sort.Strings(paths)

	for _, abs := range paths {
		src, err := os.ReadFile(abs)
		if err != nil {
			continue
		}
		f, err := parser.ParseFile(p.Fset, abs, src, parser.ParseComments)
		if err != nil {
			continue // file with syntax errors: skip, not fatal
		}
		rel, err := filepath.Rel(p.Root, abs)
		if err != nil {
			rel = abs
		}
		p.Files = append(p.Files, &GoFile{
			RelPath: filepath.ToSlash(rel),
			AbsPath: abs,
			AST:     f,
			Src:     src,
			Lines:   strings.Split(string(src), "\n"),
		})
	}
	return nil
}

// skipDir applies go tool conventions: ignore vendor/,
// testdata/, and directories starting with "." or "_".
func skipDir(name string) bool {
	return name == "vendor" ||
		name == "testdata" ||
		strings.HasPrefix(name, ".") ||
		strings.HasPrefix(name, "_")
}

// exprString renders an AST node to its source text.
// Accepts ast.Node (not just ast.Expr) because *ast.FieldList (params/results)
// implements Node but not Expr.
func (p *Project) exprString(n ast.Node) string {
	if n == nil {
		return ""
	}
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, p.Fset, n); err != nil {
		return ""
	}
	return buf.String()
}

// position converts a Pos into (relPath, line) of the file.
func (p *Project) position(pos token.Pos) (string, int) {
	tp := p.Fset.Position(pos)
	rel, err := filepath.Rel(p.Root, tp.Filename)
	if err != nil {
		rel = tp.Filename
	}
	return filepath.ToSlash(rel), tp.Line
}

// lineAt returns the trimmed text of the line at pos.
func (p *Project) lineAt(pos token.Pos) string {
	file, line := p.position(pos)
	for _, gf := range p.Files {
		if gf.RelPath == file {
			if line > 0 && line <= len(gf.Lines) {
				return strings.TrimSpace(gf.Lines[line-1])
			}
			return ""
		}
	}
	return ""
}

// TotalLines sums the lines of all files (for program_summary).
func (p *Project) TotalLines() int {
	n := 0
	for _, f := range p.Files {
		n += len(f.Lines)
	}
	return n
}
