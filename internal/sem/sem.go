// Package sem is the semantic layer: loads the project with
// `golang.org/x/tools/go/packages` and full type-checking (`go/types`).
//
// Counterpart of `analysis/analysis_host.rs` (rust-mcp-server, ra_ap_*).
// It is EXPENSIVE cold (invokes `go list` and type-checks the dependency
// graph) — that is why it lives behind a cache in the Manager and only
// semantic tools (iter 3) use it. Static tools (iter 2)
// continue using `analysis.Project` (go/parser, instant).
//
// REQUIREMENT: the `go` binary must be in the server process PATH
// (go/packages invokes `go list` as a subprocess).
package sem

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

// Project is a Go project with full type-checking.
type Project struct {
	Root   string
	Fset   *token.FileSet
	Pkgs   []*packages.Package // project packages (./...), sorted by PkgPath
	Errors []string            // load errors per package (missing deps...), sorted

	byName map[string][]types.Object // name index → objects (dedup by position)

	lineCache map[string][]string // line context on demand
}

// Load type-checks the project at root (pattern ./..., with tests).
// Per-package errors are NOT fatal: they accumulate in Project.Errors
// (e.g. a non-downloadable dependency) and analysis continues with what
// is available. Load only fails on catastrophic errors (no go.mod...).
func Load(root string) (*Project, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax |
			packages.NeedTypes | packages.NeedTypesInfo | packages.NeedImports |
			packages.NeedDeps | packages.NeedModule,
		Dir:   root,
		Tests: true,
		Fset:  token.NewFileSet(),
	}
	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return nil, fmt.Errorf("go/packages load failed (is `go` in PATH?): %w", err)
	}

	p := &Project{
		Root:      root,
		Fset:      cfg.Fset,
		byName:    map[string][]types.Object{},
		lineCache: map[string][]string{},
	}

	var errs []string
	for _, pkg := range pkgs {
		for _, e := range pkg.Errors {
			errs = append(errs, e.Error())
		}
	}
	p.Errors = errs
	sort.Strings(p.Errors)

	// Only project packages (same module or pattern ./...): pkgs is already
	// restricted by the pattern, deps are not included as roots.
	p.Pkgs = pkgs
	sort.Slice(p.Pkgs, func(i, j int) bool { return p.Pkgs[i].PkgPath < p.Pkgs[j].PkgPath })

	p.buildIndex()
	return p, nil
}

// buildIndex indexes top-level objects (func/type/const/var) and methods
// by name. With Tests:true, the same file appears in two variants
// (normal package and augmented test package) with DIFFERENT objects at
// the same Pos — deduplicates by (name, position).
func (p *Project) buildIndex() {
	seen := map[string]bool{}
	add := func(obj types.Object) {
		key := fmt.Sprintf("%s@%d", obj.Name(), obj.Pos())
		if seen[key] {
			return
		}
		seen[key] = true
		p.byName[obj.Name()] = append(p.byName[obj.Name()], obj)
	}

	for _, pkg := range p.Pkgs {
		if pkg.Types == nil {
			continue
		}
		scope := pkg.Types.Scope()
		for _, name := range scope.Names() {
			obj := scope.Lookup(name)
			add(obj)
			// Methods of named types in the project (value + pointer).
			if tn, ok := obj.(*types.TypeName); ok {
				if named, ok := tn.Type().(*types.Named); ok {
					for _, ms := range []*types.MethodSet{
						types.NewMethodSet(named),
						types.NewMethodSet(types.NewPointer(named)),
					} {
						for i := 0; i < ms.Len(); i++ {
							if m, ok := ms.At(i).Obj().(*types.Func); ok {
								add(m)
							}
						}
					}
				}
			}
		}
	}
}

// Lookup returns all objects named `name` (there may be several in
// different packages), sorted deterministically by position.
func (p *Project) Lookup(name string) []types.Object {
	objs := append([]types.Object(nil), p.byName[name]...)
	sort.Slice(objs, func(i, j int) bool { return objs[i].Pos() < objs[j].Pos() })
	return objs
}

// SameObject compares by identity OR by (name, position) — necessary
// because Tests:true duplicates objects across package variants.
func SameObject(a, b types.Object) bool {
	if a == b {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Name() == b.Name() && a.Pos() == b.Pos()
}

// PosString formats a position as `file.go:42` relative to root,
// or `pkg/path (external)` if outside (stdlib, dependencies).
// It is the anti-machine-dependent snapshot rule: no GOROOT.
func (p *Project) PosString(pos token.Pos) string {
	if !pos.IsValid() {
		return "`-`"
	}
	tp := p.Fset.Position(pos)
	if tp.Filename == "" {
		return "`-`"
	}
	if rel, err := filepath.Rel(p.Root, tp.Filename); err == nil &&
		!strings.HasPrefix(rel, "..") {
		return fmt.Sprintf("`%s:%d`", filepath.ToSlash(rel), tp.Line)
	}
	return "`" + filepath.Base(tp.Filename) + " (external)`"
}

// ObjPosString formats an object's position; if external, uses the
// package path (`fmt (external)`), much more readable than the file.
func (p *Project) ObjPosString(obj types.Object) string {
	if obj == nil || !obj.Pos().IsValid() {
		return "`-`"
	}
	tp := p.Fset.Position(obj.Pos())
	if rel, err := filepath.Rel(p.Root, tp.Filename); err == nil &&
		!strings.HasPrefix(rel, "..") {
		return fmt.Sprintf("`%s:%d`", filepath.ToSlash(rel), tp.Line)
	}
	if obj.Pkg() != nil {
		return "`" + obj.Pkg().Path() + "` (external)"
	}
	return "`builtin` (external)"
}

// LineAt returns the (trimmed) text of the line at a position.
// Caches the read file — only for project files.
func (p *Project) LineAt(pos token.Pos) string {
	tp := p.Fset.Position(pos)
	if tp.Filename == "" || tp.Line <= 0 {
		return ""
	}
	lines, ok := p.lineCache[tp.Filename]
	if !ok {
		src, err := os.ReadFile(tp.Filename)
		if err != nil {
			p.lineCache[tp.Filename] = nil
			return ""
		}
		lines = strings.Split(string(src), "\n")
		p.lineCache[tp.Filename] = lines
	}
	if tp.Line > len(lines) {
		return ""
	}
	return strings.TrimSpace(lines[tp.Line-1])
}

// Qualifier renders types relative to their own package (readability).
func Qualifier(obj types.Object) types.Qualifier {
	if obj != nil && obj.Pkg() != nil {
		return types.RelativeTo(obj.Pkg())
	}
	return nil
}

// NamedProjectTypes returns all *types.Named declared in the
// project's packages (dedup by position), sorted by name.
// It is the universe of find_implementations / get_type_hierarchy.
func (p *Project) NamedProjectTypes() []*types.Named {
	seen := map[token.Pos]bool{}
	var out []*types.Named
	for _, pkg := range p.Pkgs {
		if pkg.Types == nil {
			continue
		}
		scope := pkg.Types.Scope()
		for _, name := range scope.Names() {
			tn, ok := scope.Lookup(name).(*types.TypeName)
			if !ok {
				continue
			}
			named, ok := tn.Type().(*types.Named)
			if !ok || seen[tn.Pos()] {
				continue
			}
			seen[tn.Pos()] = true
			out = append(out, named)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Obj().Name() != out[j].Obj().Name() {
			return out[i].Obj().Name() < out[j].Obj().Name()
		}
		return out[i].Obj().Pos() < out[j].Obj().Pos()
	})
	return out
}

// EnclosingFunc returns the FuncDecl containing pos in file, or nil.
func EnclosingFunc(file *ast.File, pos token.Pos) *ast.FuncDecl {
	for _, decl := range file.Decls {
		if fd, ok := decl.(*ast.FuncDecl); ok && fd.Pos() <= pos && pos < fd.End() {
			return fd
		}
	}
	return nil
}

// FuncDisplayName renders a function as `Name` or `ReceiverBase.Name`.
func FuncDisplayName(fd *ast.FuncDecl) string {
	if fd.Recv == nil || len(fd.Recv.List) == 0 {
		return fd.Name.Name
	}
	recv := fd.Recv.List[0].Type
	for {
		switch t := recv.(type) {
		case *ast.StarExpr:
			recv = t.X
			continue
		case *ast.Ident:
			return t.Name + "." + fd.Name.Name
		case *ast.IndexExpr: // generics: Foo[T]
			recv = t.X
			continue
		case *ast.IndexListExpr:
			recv = t.X
			continue
		}
		break
	}
	return fd.Name.Name
}
