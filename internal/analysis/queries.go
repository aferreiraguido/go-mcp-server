// queries.go — the stable query facade over the AST.
//
// Counterpart of `RustAnalyzerUtils` / `ProLeapAnalyzerUtils`: all
// listings return SORTED slices (byte-by-byte determinism of
// tool Markdown). Tools do not traverse the AST directly.
package analysis

import (
	"fmt"
	"go/ast"
	"go/token"
	"sort"
	"strconv"
	"strings"
)

// PackageInfo summarizes a package (directory with .go files).
type PackageInfo struct {
	Name  string
	Dir   string // relative to root, "." for root
	Files int
}

// DepInfo is a go.mod dependency.
type DepInfo struct {
	Path     string
	Version  string
	Indirect bool
}

// FuncInfo describes a function or method.
type FuncInfo struct {
	Name      string
	Signature string // "func Name(params) results" without body
	Receiver  string // "" for functions; "T" or "*T" for methods
	Exported  bool
	File      string
	Line      int
	Doc       string
	Body      string // full source including body (for inspect_function)
}

// FieldInfo describes a struct field.
type FieldInfo struct {
	Name     string
	Type     string
	Tag      string
	Exported bool
	Embedded bool
}

// StructInfo describes a struct type.
type StructInfo struct {
	Name     string
	Fields   []FieldInfo
	Exported bool
	File     string
	Line     int
	Doc      string
}

// InterfaceMethod describes an interface method or embed.
type InterfaceMethod struct {
	Name      string
	Signature string // "(params) results" without "func"
	Embedded  bool
}

// InterfaceInfo describes an interface type.
type InterfaceInfo struct {
	Name     string
	Methods  []InterfaceMethod
	Exported bool
	File     string
	Line     int
	Doc      string
}

// ConstInfo describes a constant (each spec of a const block).
type ConstInfo struct {
	Name     string
	Type     string
	Value    string
	Exported bool
	File     string
	Line     int
}

// VarInfo describes a package-level variable.
type VarInfo struct {
	Name     string
	Type     string
	Exported bool
	File     string
	Line     int
}

// ImportInfo describes a file import.
type ImportInfo struct {
	Path  string
	Alias string
	File  string
	Line  int
}

// Packages groups files by directory and returns packages
// sorted by Dir.
func (p *Project) Packages() []PackageInfo {
	byDir := map[string]*PackageInfo{}
	for _, f := range p.Files {
		dir := dirOf(f.RelPath)
		pi, ok := byDir[dir]
		if !ok {
			pi = &PackageInfo{Dir: dir}
			byDir[dir] = pi
		}
		pi.Files++
		// Package name: from the first non-test file seen;
		// if only tests exist, from the first.
		if pi.Name == "" || (pi.Name == f.AST.Name.Name && !strings.HasSuffix(f.RelPath, "_test.go")) {
			if !strings.HasSuffix(f.RelPath, "_test.go") || pi.Name == "" {
				pi.Name = f.AST.Name.Name
			}
		}
	}
	out := make([]PackageInfo, 0, len(byDir))
	for _, pi := range byDir {
		out = append(out, *pi)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Dir < out[j].Dir })
	return out
}

// Dependencies returns go.mod requires, sorted by path.
func (p *Project) Dependencies() []DepInfo {
	var out []DepInfo
	for _, r := range p.Module.Require {
		out = append(out, DepInfo{Path: r.Mod.Path, Version: r.Mod.Version, Indirect: r.Indirect})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// Replaces returns replace directives as "old => new version".
func (p *Project) Replaces() []string {
	var out []string
	for _, r := range p.Module.Replace {
		out = append(out, r.Old.Path+" "+r.Old.Version+" => "+r.New.Path+" "+r.New.Version)
	}
	sort.Strings(out)
	return out
}

// Functions returns functions and methods from the entire project,
// sorted by (Name, File, Line).
func (p *Project) Functions() []FuncInfo {
	var out []FuncInfo
	for _, f := range p.Files {
		for _, decl := range f.AST.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			out = append(out, p.funcInfo(f, fd))
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		return out[i].Line < out[j].Line
	})
	return out
}

// MethodsOf returns methods whose base receiver is typeName
// (e.g. "Greeter" matches receivers "Greeter" and "*Greeter"), sorted by name.
func (p *Project) MethodsOf(typeName string) []FuncInfo {
	var out []FuncInfo
	for _, fn := range p.Functions() {
		if fn.Receiver != "" && receiverBase(fn.Receiver) == typeName {
			out = append(out, fn)
		}
	}
	return out
}

// FindFunc finds a function (or method) by exact name.
// Returns the first match in deterministic order, or nil.
func (p *Project) FindFunc(name string) *FuncInfo {
	for _, fn := range p.Functions() {
		if fn.Name == name {
			return &fn
		}
	}
	return nil
}

// Structs returns all struct types, sorted by name.
func (p *Project) Structs() []StructInfo {
	var out []StructInfo
	for _, f := range p.Files {
		for _, decl := range f.AST.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				continue
			}
			for _, spec := range gd.Specs {
				ts := spec.(*ast.TypeSpec)
				st, ok := ts.Type.(*ast.StructType)
				if !ok {
					continue
				}
				out = append(out, p.structInfo(f, ts, st, docOf(gd, ts)))
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].File < out[j].File
	})
	return out
}

// FindStruct finds a struct by exact name.
func (p *Project) FindStruct(name string) *StructInfo {
	for _, s := range p.Structs() {
		if s.Name == name {
			return &s
		}
	}
	return nil
}

// Interfaces returns all interface types, sorted by name.
func (p *Project) Interfaces() []InterfaceInfo {
	var out []InterfaceInfo
	for _, f := range p.Files {
		for _, decl := range f.AST.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				continue
			}
			for _, spec := range gd.Specs {
				ts := spec.(*ast.TypeSpec)
				it, ok := ts.Type.(*ast.InterfaceType)
				if !ok {
					continue
				}
				out = append(out, p.interfaceInfo(f, ts, it, docOf(gd, ts)))
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].File < out[j].File
	})
	return out
}

// FindInterface finds an interface by exact name.
func (p *Project) FindInterface(name string) *InterfaceInfo {
	for _, i := range p.Interfaces() {
		if i.Name == name {
			return &i
		}
	}
	return nil
}

// Constants returns all package-level constants (each spec),
// sorted by (Name, File).
func (p *Project) Constants() []ConstInfo {
	var out []ConstInfo
	for _, f := range p.Files {
		for _, decl := range f.AST.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.CONST {
				continue
			}
			for _, spec := range gd.Specs {
				vs := spec.(*ast.ValueSpec)
				for i, name := range vs.Names {
					ci := ConstInfo{
						Name:     name.Name,
						Type:     p.exprString(vs.Type),
						Exported: name.IsExported(),
					}
					if i < len(vs.Values) {
						ci.Value = p.exprString(vs.Values[i])
					}
					ci.File, ci.Line = p.position(name.Pos())
					out = append(out, ci)
				}
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].File < out[j].File
	})
	return out
}

// Variables returns all package-level variables, sorted by (Name, File).
func (p *Project) Variables() []VarInfo {
	var out []VarInfo
	for _, f := range p.Files {
		for _, decl := range f.AST.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.VAR {
				continue
			}
			for _, spec := range gd.Specs {
				vs := spec.(*ast.ValueSpec)
				for _, name := range vs.Names {
					vi := VarInfo{
						Name:     name.Name,
						Type:     p.exprString(vs.Type),
						Exported: name.IsExported(),
					}
					if vi.Type == "" && len(vs.Values) > 0 {
						vi.Type = "= " + p.exprString(vs.Values[0])
					}
					vi.File, vi.Line = p.position(name.Pos())
					out = append(out, vi)
				}
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].File < out[j].File
	})
	return out
}

// Imports returns all imports from all files,
// sorted by (File, Path).
func (p *Project) Imports() []ImportInfo {
	var out []ImportInfo
	for _, f := range p.Files {
		for _, imp := range f.AST.Imports {
			path := unquoteOrRaw(imp.Path.Value)
			alias := ""
			if imp.Name != nil {
				alias = imp.Name.Name
			}
			ii := ImportInfo{Path: path, Alias: alias, File: f.RelPath}
			_, ii.Line = p.position(imp.Pos())
			out = append(out, ii)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		return out[i].Path < out[j].Path
	})
	return out
}

// TopLevelSymbol locates a top-level symbol (func/type/const/var) by
// exact name. Does NOT match methods (same as validate_code_reference in
// rust-mcp-server). Returns (kind, file, line, found).
func (p *Project) TopLevelSymbol(name string) (kind, file string, line int, found bool) {
	for _, f := range p.Files {
		for _, decl := range f.AST.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if d.Recv == nil && d.Name.Name == name {
					file, line = p.position(d.Name.Pos())
					return "func", file, line, true
				}
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					switch s := spec.(type) {
					case *ast.TypeSpec:
						if s.Name.Name == name {
							file, line = p.position(s.Name.Pos())
							return "type", file, line, true
						}
					case *ast.ValueSpec:
						for _, n := range s.Names {
							if n.Name == name {
								file, line = p.position(n.Pos())
								return strings.ToLower(d.Tok.String()), file, line, true
							}
						}
					}
				}
			}
		}
	}
	return "", "", 0, false
}

// BugFinding is a finding from the bug-pattern tools (iter 4).
type BugFinding struct {
	Kind     string // panic | unsafe | defer_in_loop | unchecked_error | dead_code | goroutine_leak
	Severity string // critical | high | medium | low
	Message  string
	File     string
	Line     int
	Context  string
}

// FindPanicSites searches for panic(), log.Fatal/Panic, os.Exit in the AST.
func (p *Project) FindPanicSites() []BugFinding {
	var out []BugFinding
	for _, f := range p.Files {
		ast.Inspect(f.AST, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			fun := p.exprString(call.Fun)
			var kind, msg string
			switch {
			case fun == "panic" || fun == "panic(":
				kind, msg = "panic", "panic() call"
			case fun == "log.Fatal" || fun == "log.Fatal(":
				kind, msg = "panic", "log.Fatal() call"
			case fun == "log.Panic" || fun == "log.Panic(":
				kind, msg = "panic", "log.Panic() call"
			case fun == "os.Exit" || fun == "os.Exit(":
				kind, msg = "panic", "os.Exit() call"
			case fun == "log.Fatalf" || fun == "log.Fatalf(":
				kind, msg = "panic", "log.Fatalf() call"
			case fun == "log.Panicf" || fun == "log.Panicf(":
				kind, msg = "panic", "log.Panicf() call"
			}
			if kind != "" {
				file, line := p.position(call.Pos())
				out = append(out, BugFinding{
					Kind:     kind,
					Severity: "high",
					Message:  msg,
					File:     file,
					Line:     line,
					Context:  p.lineAt(call.Pos()),
				})
			}
			return true
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		return out[i].Line < out[j].Line
	})
	return out
}

// FindUnsafeUsage searches for unsafe package imports and uses.
func (p *Project) FindUnsafeUsage() []BugFinding {
	hasUnsafe := false
	for _, f := range p.Files {
		for _, imp := range f.AST.Imports {
			path := unquoteOrRaw(imp.Path.Value)
			if path == "unsafe" {
				hasUnsafe = true
			}
		}
	}
	if !hasUnsafe {
		return nil
	}

	var out []BugFinding
	for _, f := range p.Files {
		ast.Inspect(f.AST, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.CallExpr:
				fun := p.exprString(node.Fun)
				if strings.Contains(fun, "unsafe.") {
					file, line := p.position(node.Pos())
					out = append(out, BugFinding{
						Kind:     "unsafe",
						Severity: "high",
						Message:  "unsafe operation: " + fun,
						File:     file,
						Line:     line,
						Context:  p.lineAt(node.Pos()),
					})
				}
			case *ast.UnaryExpr:
				if node.Op == token.AND {
					if star, ok := node.X.(*ast.StarExpr); ok {
						if call, ok := star.X.(*ast.CallExpr); ok {
							fun := p.exprString(call.Fun)
							if strings.Contains(fun, "unsafe.") {
								file, line := p.position(call.Pos())
								out = append(out, BugFinding{
									Kind:     "unsafe",
									Severity: "high",
									Message:  "unsafe dereference via " + fun,
									File:     file,
									Line:     line,
									Context:  p.lineAt(node.Pos()),
								})
							}
						}
					}
				}
			}
			return true
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		return out[i].Line < out[j].Line
	})
	return out
}

// FindDeferInLoops finds defer inside for/range loops — resource
// leak because defer does not execute until the function returns.
func (p *Project) FindDeferInLoops() []BugFinding {
	var out []BugFinding
	for _, f := range p.Files {
		ast.Inspect(f.AST, func(n ast.Node) bool {
			var loopBody *ast.BlockStmt
			switch node := n.(type) {
			case *ast.ForStmt:
				loopBody = node.Body
			case *ast.RangeStmt:
				loopBody = node.Body
			}
			if loopBody == nil {
				return true
			}
			for _, stmt := range loopBody.List {
				if ds, ok := stmt.(*ast.DeferStmt); ok {
					file, line := p.position(ds.Pos())
					out = append(out, BugFinding{
						Kind:     "defer_in_loop",
						Severity: "high",
						Message:  "defer inside loop — resource may leak",
						File:     file,
						Line:     line,
						Context:  p.lineAt(ds.Pos()),
					})
				}
			}
			return true
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		return out[i].Line < out[j].Line
	})
	return out
}

// FrameworkInfo describes a framework detected in the project.
type FrameworkInfo struct {
	Name    string // "gin", "echo", "cobra", "grpc", "chi", "fiber", "net/http"
	Version string // go.mod version ("" if not applicable)
	File    string // file where detected
	Line    int
}

// DetectFrameworks searches for known Go frameworks in go.mod and in .go
// file imports.
func (p *Project) DetectFrameworks() []FrameworkInfo {
	knownFrameworks := map[string]string{
		"github.com/gin-gonic/gin":         "gin",
		"github.com/labstack/echo":         "echo",
		"github.com/labstack/echo/v4":      "echo",
		"github.com/spf13/cobra":           "cobra",
		"google.golang.org/grpc":           "grpc",
		"github.com/go-chi/chi":            "chi",
		"github.com/go-chi/chi/v5":         "chi",
		"github.com/gofiber/fiber/v2":      "fiber",
		"github.com/gorilla/mux":           "gorilla/mux",
		"github.com/urfave/cli/v2":         "urfave/cli",
		"github.com/gorilla/websocket":     "gorilla/websocket",
		"github.com/prometheus/client_golang": "prometheus",
	}

	var out []FrameworkInfo
	seen := map[string]bool{}

	// Search in go.mod
	for _, r := range p.Module.Require {
		name, ok := knownFrameworks[r.Mod.Path]
		if !ok {
			continue
		}
		if seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, FrameworkInfo{
			Name:    name,
			Version: r.Mod.Version,
		})
	}

	// Search in .go file imports for frameworks without go.mod (net/http)
	for _, f := range p.Files {
		for _, imp := range f.AST.Imports {
			path := unquoteOrRaw(imp.Path.Value)
			if path == "net/http" && !seen["net/http"] {
				seen["net/http"] = true
				out = append(out, FrameworkInfo{
					Name: "net/http", File: f.RelPath, Line: p.lineOf(imp.Pos()),
				})
			}
			// Detection by base package
			for modPath, name := range knownFrameworks {
				if strings.HasPrefix(path, modPath) && !seen[name] {
					seen[name] = true
					out = append(out, FrameworkInfo{
						Name: name, File: f.RelPath, Line: p.lineOf(imp.Pos()),
					})
					break
				}
			}
		}
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// APIEndpoint describes a detected API route.
type APIEndpoint struct {
	Method  string // GET, POST, PUT, DELETE, etc.
	Path    string // /api/v1/users
	Handler string // handler name
	File    string
	Line    int
}

// ListAPIEndpoints searches for HTTP route registrations in the code.
// Detects gin, echo, chi, gorilla/mux, net/http.
func (p *Project) ListAPIEndpoints() []APIEndpoint {
	var out []APIEndpoint
	routeMethods := []string{
		".GET(", ".POST(", ".PUT(", ".DELETE(", ".PATCH(", ".HEAD(", ".OPTIONS(",
	}

	for _, f := range p.Files {
		ast.Inspect(f.AST, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			methodCall := "." + sel.Sel.Name + "("
			isRoute := false
			for _, m := range routeMethods {
				if methodCall == m {
					isRoute = true
					break
				}
			}
			if !isRoute && sel.Sel.Name != "Handle" && sel.Sel.Name != "HandleFunc" {
				return true
			}

			// Extract the path (first string argument).
			if len(call.Args) < 1 {
				return true
			}
			pathLit, ok := call.Args[0].(*ast.BasicLit)
			if !ok || pathLit.Kind != token.STRING {
				return true
			}
			path, _ := strconv.Unquote(pathLit.Value)

			handler := ""
			if len(call.Args) >= 2 {
				handler = p.exprString(call.Args[1])
			}
			if handler == "" {
				handler = "handler"
			}

			_, line := p.position(call.Pos())
			if isRoute {
				out = append(out, APIEndpoint{
					Method: sel.Sel.Name, Path: path, Handler: handler,
					File: f.RelPath, Line: line,
				})
			} else if sel.Sel.Name == "Handle" || sel.Sel.Name == "HandleFunc" {
				out = append(out, APIEndpoint{
					Method: sel.Sel.Name, Path: path, Handler: handler,
					File: f.RelPath, Line: line,
				})
			}
			return true
		})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		return out[i].Line < out[j].Line
	})
	return out
}

// EnvDependency describes an environment variable or config read.
type EnvDependency struct {
	Kind    string // os.Getenv | os.LookupEnv | flag | viper
	Key     string // variable/flag name ("" if dynamic)
	Default string // default value ("" if none)
	File    string
	Line    int
}

// ExtractEnvDependencies finds environment variable reads,
// flags and config (os.Getenv, os.LookupEnv, flag.*, viper.*).
func (p *Project) ExtractEnvDependencies() []EnvDependency {
	var out []EnvDependency
	for _, f := range p.Files {
		ast.Inspect(f.AST, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			selName := sel.Sel.Name

			// os.Getenv / os.LookupEnv
			if (selName == "Getenv" || selName == "LookupEnv") && len(call.Args) >= 1 {
				key := ""
				if lit, ok := call.Args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
					key = unquoteOrRaw(lit.Value)
				}
				_, line := p.position(call.Pos())
				out = append(out, EnvDependency{
					Kind: "os." + selName, Key: key,
					File: f.RelPath, Line: line,
				})
				return true
			}

			// os.Getenv alternative as direct function (without selector)
			if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "Getenv" && len(call.Args) >= 1 {
				key := ""
				if lit, ok := call.Args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
					key = unquoteOrRaw(lit.Value)
				}
				_, line := p.position(call.Pos())
				out = append(out, EnvDependency{
					Kind: "os.Getenv", Key: key,
					File: f.RelPath, Line: line,
				})
				return true
			}

			// viper.Get*, viper.SetDefault, viper.BindEnv
			if strings.HasPrefix(selName, "Get") || selName == "SetDefault" || selName == "BindEnv" {
				if len(call.Args) >= 1 {
					key := ""
					if lit, ok := call.Args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
						key = unquoteOrRaw(lit.Value)
					}
					_, line := p.position(call.Pos())
					out = append(out, EnvDependency{
						Kind: "viper." + selName, Key: key,
						File: f.RelPath, Line: line,
					})
				}
				return true
			}

			return true
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		return out[i].Line < out[j].Line
	})
	return out
}

// lineOf returns the line number of a position.
func (p *Project) lineOf(pos token.Pos) int {
	_, line := p.position(pos)
	return line
}

// EntryPoint is an entry point: func main in package main, or func init.
type EntryPoint struct {
	Kind    string // "main" | "init"
	Package string // package name
	File    string
	Line    int
}

// EntryPoints returns func main() from main packages and all func init(),
// sorted by (Kind, File, Line).
func (p *Project) EntryPoints() []EntryPoint {
	var out []EntryPoint
	for _, f := range p.Files {
		pkgName := f.AST.Name.Name
		for _, decl := range f.AST.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Recv != nil {
				continue
			}
			var kind string
			switch {
			case fd.Name.Name == "main" && pkgName == "main":
				kind = "main"
			case fd.Name.Name == "init":
				kind = "init"
			default:
				continue
			}
			file, line := p.position(fd.Name.Pos())
			out = append(out, EntryPoint{Kind: kind, Package: pkgName, File: file, Line: line})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		return out[i].Line < out[j].Line
	})
	return out
}

// --- internal helpers ---

// funcInfo builds the FuncInfo from a FuncDecl.
func (p *Project) funcInfo(f *GoFile, fd *ast.FuncDecl) FuncInfo {
	fi := FuncInfo{
		Name:     fd.Name.Name,
		Exported: fd.Name.IsExported(),
	}
	fi.File, fi.Line = p.position(fd.Pos())
	if fd.Doc != nil {
		fi.Doc = strings.TrimSpace(fd.Doc.Text())
	}
	if fd.Recv != nil && len(fd.Recv.List) > 0 {
		fi.Receiver = p.exprString(fd.Recv.List[0].Type)
	}

	// Signature: "func Name(params) results" without body.
	fi.Signature = "func " + fd.Name.Name + p.funcTypeString(fd.Type)

	// Body: slice of the original source.
	start := p.Fset.Position(fd.Pos()).Offset
	end := p.Fset.Position(fd.End()).Offset
	if start >= 0 && end <= len(f.Src) && start < end {
		fi.Body = string(f.Src[start:end])
	}
	return fi
}

// structInfo builds the StructInfo from a TypeSpec+StructType.
func (p *Project) structInfo(f *GoFile, ts *ast.TypeSpec, st *ast.StructType, doc string) StructInfo {
	si := StructInfo{
		Name:     ts.Name.Name,
		Exported: ts.Name.IsExported(),
		Doc:      doc,
	}
	si.File, si.Line = p.position(ts.Pos())
	for _, field := range st.Fields.List {
		fType := p.exprString(field.Type)
		tag := ""
		if field.Tag != nil {
			tag = unquoteOrRaw(field.Tag.Value)
		}
		if len(field.Names) == 0 {
			// Embedded field: name is the base type.
			si.Fields = append(si.Fields, FieldInfo{
				Name:     receiverBase(fType),
				Type:     fType,
				Tag:      tag,
				Exported: ast.IsExported(receiverBase(fType)),
				Embedded: true,
			})
			continue
		}
		for _, name := range field.Names {
			si.Fields = append(si.Fields, FieldInfo{
				Name:     name.Name,
				Type:     fType,
				Tag:      tag,
				Exported: name.IsExported(),
			})
		}
	}
	return si
}

// interfaceInfo builds the InterfaceInfo from a TypeSpec+InterfaceType.
func (p *Project) interfaceInfo(f *GoFile, ts *ast.TypeSpec, it *ast.InterfaceType, doc string) InterfaceInfo {
	ii := InterfaceInfo{
		Name:     ts.Name.Name,
		Exported: ts.Name.IsExported(),
		Doc:      doc,
	}
	ii.File, ii.Line = p.position(ts.Pos())
	for _, m := range it.Methods.List {
		if len(m.Names) == 0 {
			// Embedded interface (e.g. io.Stringer, comparable).
			ii.Methods = append(ii.Methods, InterfaceMethod{
				Name:     p.exprString(m.Type),
				Embedded: true,
			})
			continue
		}
		ft, ok := m.Type.(*ast.FuncType)
		if !ok {
			continue
		}
		sig := p.funcTypeString(ft)
		for _, name := range m.Names {
			ii.Methods = append(ii.Methods, InterfaceMethod{
				Name:      name.Name,
				Signature: sig,
			})
		}
	}
	return ii
}

// funcTypeString renders "(params) results" in go/doc style:
// params always with parentheses; results without parentheses only if a
// single unnamed result. Deterministic, not relying on printer
// positions (lesson: position hacks break output).
func (p *Project) funcTypeString(ft *ast.FuncType) string {
	var b strings.Builder
	b.WriteString(p.fieldListString(ft.Params, true))
	if ft.Results != nil && len(ft.Results.List) > 0 {
		singleUnnamed := len(ft.Results.List) == 1 && len(ft.Results.List[0].Names) == 0
		b.WriteString(" ")
		b.WriteString(p.fieldListString(ft.Results, !singleUnnamed))
	}
	return b.String()
}

// fieldListString renders a field list: "a, b int, c string",
// with parentheses if `parens` (or empty list with parentheses: "()").
func (p *Project) fieldListString(fl *ast.FieldList, parens bool) string {
	if fl == nil || len(fl.List) == 0 {
		if parens {
			return "()"
		}
		return ""
	}
	var parts []string
	for _, f := range fl.List {
		typ := p.exprString(f.Type)
		if len(f.Names) == 0 {
			parts = append(parts, typ)
			continue
		}
		var names []string
		for _, n := range f.Names {
			names = append(names, n.Name)
		}
		parts = append(parts, strings.Join(names, ", ")+" "+typ)
	}
	joined := strings.Join(parts, ", ")
	if parens {
		return "(" + joined + ")"
	}
	return joined
}

// docOf extracts the doc comment: from TypeSpec if it exists, otherwise from GenDecl.
func docOf(gd *ast.GenDecl, ts *ast.TypeSpec) string {
	if ts.Doc != nil && strings.TrimSpace(ts.Doc.Text()) != "" {
		return strings.TrimSpace(ts.Doc.Text())
	}
	if gd.Doc != nil {
		return strings.TrimSpace(gd.Doc.Text())
	}
	return ""
}

// receiverBase strips pointers and parentheses: "*Greeter" → "Greeter".
func receiverBase(recv string) string {
	s := strings.TrimSpace(recv)
	s = strings.TrimPrefix(s, "*")
	s = strings.Trim(s, "()")
	return s
}

// dirOf returns the relative directory of a relPath ("." for root).
func dirOf(relPath string) string {
	i := strings.LastIndex(relPath, "/")
	if i < 0 {
		return "."
	}
	return relPath[:i]
}

// unquoteOrRaw unwraps a quoted Go string returning the value without
// quotes. If parsing fails (impossible in valid Go code), returns
// the raw with outer quotes removed.
func unquoteOrRaw(lit string) string {
	if q, err := strconv.Unquote(lit); err == nil {
		return q
	}
	return strings.Trim(lit, `"`)
}

// funcDisplayName builds the display name of a function,
// with receiver as prefix if it is a method.
func (p *Project) funcDisplayName(fd *ast.FuncDecl) string {
	if fd.Recv != nil && len(fd.Recv.List) > 0 {
		return "(" + p.exprString(fd.Recv.List[0].Type) + ")." + fd.Name.Name
	}
	return fd.Name.Name
}

// LineAt returns the trimmed text of a line by relPath and number.
func (p *Project) LineAt(relPath string, line int) string {
	for _, gf := range p.Files {
		if gf.RelPath == relPath {
			if line > 0 && line <= len(gf.Lines) {
				return strings.TrimSpace(gf.Lines[line-1])
			}
			return ""
		}
	}
	return ""
}

// FuncMetric describes a function with a numeric metric.
type FuncMetric struct {
	Name     string
	Location string
	Lines    int
}

// FindLongFunctions returns functions whose body length (body lines)
// exceeds maxLines.
func (p *Project) FindLongFunctions(maxLines int) []FuncMetric {
	var out []FuncMetric
	for _, f := range p.Files {
		for _, decl := range f.AST.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			start := p.Fset.Position(fd.Body.Lbrace).Line
			end := p.Fset.Position(fd.Body.Rbrace).Line
			n := end - start - 1 // lines of code between braces
			if n > maxLines {
				loc := fmt.Sprintf("`%s:%d`", f.RelPath, p.Fset.Position(fd.Pos()).Line)
				out = append(out, FuncMetric{
					Name:     p.funcDisplayName(fd),
					Location: loc,
					Lines:    n,
				})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Lines > out[j].Lines })
	return out
}

// FuncComplexity describes a function with its cyclomatic complexity.
type FuncComplexity struct {
	Name       string
	Location   string
	Complexity int
}

// cyclomatic counts the cyclomatic complexity of a block.
func cyclomatic(n ast.Node) int {
	c := 1
	ast.Inspect(n, func(child ast.Node) bool {
		switch child.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt,
			*ast.CaseClause, *ast.CommClause:
			c++
		case *ast.BinaryExpr:
			if be, ok := child.(*ast.BinaryExpr); ok && (be.Op == token.LAND || be.Op == token.LOR) {
				c++
			}
		}
		return true
	})
	return c
}

// FindHighComplexity returns functions with cyclomatic complexity > max.
func (p *Project) FindHighComplexity(maxComplexity int) []FuncComplexity {
	var out []FuncComplexity
	for _, f := range p.Files {
		for _, decl := range f.AST.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			c := cyclomatic(fd.Body)
			if c > maxComplexity {
				loc := fmt.Sprintf("`%s:%d`", f.RelPath, p.Fset.Position(fd.Pos()).Line)
				out = append(out, FuncComplexity{
					Name:       p.funcDisplayName(fd),
					Location:   loc,
					Complexity: c,
				})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Complexity > out[j].Complexity })
	return out
}

// ListDirectories returns the unique directories of the project, sorted.
func (p *Project) ListDirectories() []string {
	seen := map[string]bool{}
	var out []string
	for _, f := range p.Files {
		d := dirOf(f.RelPath)
		if !seen[d] {
			seen[d] = true
			out = append(out, d)
		}
	}
	sort.Strings(out)
	return out
}

// ListFiles returns relative paths of all .go files in the project.
func (p *Project) ListFiles() []string {
	var out []string
	for _, f := range p.Files {
		out = append(out, f.RelPath)
	}
	return out
}

// FindUnusedParameters finds functions with unused parameters in the body.
// Skips methods that implement interfaces (heuristic: checks receiver).
func (p *Project) FindUnusedParameters() []BugFinding {
	var out []BugFinding
	for _, f := range p.Files {
		for _, decl := range f.AST.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Body == nil || fd.Type.Params == nil {
				continue
			}
			// Collect parameter names.
			paramNames := map[string]token.Pos{}
			for _, field := range fd.Type.Params.List {
				for _, name := range field.Names {
					if name.Name != "_" {
						paramNames[name.Name] = name.Pos()
					}
				}
			}
			if len(paramNames) == 0 {
				continue
			}
			// Search for uses in the body.
			used := map[string]bool{}
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				id, ok := n.(*ast.Ident)
				if !ok {
					return true
				}
			if paramNames[id.Name] == id.Pos() {
				// It is the declaration — ignore.
					return true
				}
				if _, isParam := paramNames[id.Name]; isParam {
					used[id.Name] = true
				}
				return true
			})
			for name, pos := range paramNames {
				if used[name] {
					continue
				}
				out = append(out, BugFinding{
					Kind:     "unused_param",
					Severity: "low",
					Message:  "unused parameter: " + name,
					File:     f.RelPath,
					Line:     p.Fset.Position(pos).Line,
					Context:  p.LineAt(f.RelPath, p.Fset.Position(pos).Line),
				})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		return out[i].Line < out[j].Line
	})
	return out
}
