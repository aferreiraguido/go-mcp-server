// queries.go — semantic query facade over the type-checked project.
// Counterpart of rust-mcp-server semantic tools (iter 3.5/3.6), with the
// Go particularity: interface satisfaction is implicit and only
// discoverable with go/types (it is the star query of this layer).
package sem

import (
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"
)

// ResolvedSymbol is the result of resolve_type.
type ResolvedSymbol struct {
	Name     string
	Kind     string // func | method | type | const | var
	Type     string // full rendered type
	Package  string // object's pkg path ("" for builtins)
	Position string // formatted PosString
}

// Resolve resolves a name to its objects (definitions), with type.
func (p *Project) Resolve(name string) []ResolvedSymbol {
	var out []ResolvedSymbol
	for _, obj := range p.Lookup(name) {
		rs := ResolvedSymbol{
			Name:     obj.Name(),
			Kind:     kindOf(obj),
			Type:     types.TypeString(obj.Type(), Qualifier(obj)),
			Position: p.ObjPosString(obj),
		}
		if obj.Pkg() != nil {
			rs.Package = obj.Pkg().Path()
		} else {
			rs.Package = "(builtin)"
		}
		out = append(out, rs)
	}
	return out
}

// kindOf classifies the object for the Kind column.
func kindOf(obj types.Object) string {
	switch o := obj.(type) {
	case *types.Func:
		if sig, ok := o.Type().(*types.Signature); ok && sig.Recv() != nil {
			return "method"
		}
		return "func"
	case *types.TypeName:
		return "type"
	case *types.Const:
		return "const"
	case *types.Var:
		return "var"
	default:
		return "unknown"
	}
}

// Implementation describes a type that satisfies an interface.
type Implementation struct {
	TypeName string // `Greeter` o `*Greeter`
	Via      string // value | pointer
	Position string
}

// Implementations returns project types that satisfy the
// interface ifaceName. If the symbol does not exist or is not an interface,
// returns (nil, false, reason).
func (p *Project) Implementations(ifaceName string) ([]Implementation, bool, string) {
	ifaceObj := p.findInterface(ifaceName)
	if ifaceObj == nil {
		return nil, false, "interface '" + ifaceName + "' not found in project packages"
	}
	iface, ok := ifaceObj.Type().Underlying().(*types.Interface)
	if !ok {
		return nil, false, "'" + ifaceName + "' is not an interface type"
	}

	var out []Implementation
	for _, named := range p.NamedProjectTypes() {
		// The interface itself is never an "implementation" of itself.
		if SameObject(named.Obj(), ifaceObj) {
			continue
		}
		obj := named.Obj()
		switch {
		case types.Implements(named, iface):
			out = append(out, Implementation{
				TypeName: obj.Name(), Via: "value", Position: p.ObjPosString(obj),
			})
		case types.Implements(types.NewPointer(named), iface):
			out = append(out, Implementation{
				TypeName: "*" + obj.Name(), Via: "pointer", Position: p.ObjPosString(obj),
			})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TypeName < out[j].TypeName })
	return out, true, ""
}

// findInterface locates the TypeName of an interface in the project.
func (p *Project) findInterface(name string) *types.TypeName {
	for _, obj := range p.Lookup(name) {
		if tn, ok := obj.(*types.TypeName); ok {
			if _, isIface := tn.Type().Underlying().(*types.Interface); isIface {
				return tn
			}
		}
	}
	return nil
}

// TypeHierarchy is the hierarchy of a type: embedding (up/down) for
// structs, embeds + implementors for interfaces.
type TypeHierarchy struct {
	Kind       string // struct | interface | other
	Embeds     []EmbedEntry
	EmbeddedBy []EmbedEntry
	Impls      []Implementation
}

// EmbedEntry is an embedded type (or a container that embeds).
type EmbedEntry struct {
	TypeName string
	Position string
}

// Hierarchy calculates the hierarchy of a symbol.
// Go has no inheritance: hierarchy is embedding (structs) and
// satisfaction (interfaces).
func (p *Project) Hierarchy(name string) *TypeHierarchy {
	for _, obj := range p.Lookup(name) {
		tn, ok := obj.(*types.TypeName)
		if !ok {
			continue
		}
		named, ok := tn.Type().(*types.Named)
		if !ok {
			continue
		}
		switch u := named.Underlying().(type) {
		case *types.Struct:
			h := &TypeHierarchy{Kind: "struct"}
			h.Embeds = p.embedsOf(u)
			h.EmbeddedBy = p.containersOf(named)
			return h
		case *types.Interface:
			h := &TypeHierarchy{Kind: "interface"}
			for i := 0; i < u.NumEmbeddeds(); i++ {
				et := u.EmbeddedType(i)
				h.Embeds = append(h.Embeds, EmbedEntry{
					TypeName: types.TypeString(et, Qualifier(tn)),
					Position: p.ObjPosString(typeObjOf(et)),
				})
			}
			impls, ok, _ := p.Implementations(name)
			if ok {
				h.Impls = impls
			}
			return h
		}
	}
	return &TypeHierarchy{Kind: "other"}
}

// embedsOf lists the embedded fields of a struct.
func (p *Project) embedsOf(st *types.Struct) []EmbedEntry {
	var out []EmbedEntry
	for i := 0; i < st.NumFields(); i++ {
		f := st.Field(i)
		if !f.Embedded() {
			continue
		}
		out = append(out, EmbedEntry{
			TypeName: types.TypeString(f.Type(), Qualifier(f)),
			Position: p.ObjPosString(f),
		})
	}
	return out
}

// containersOf lists project structs that embed `named`.
func (p *Project) containersOf(named *types.Named) []EmbedEntry {
	var out []EmbedEntry
	for _, other := range p.NamedProjectTypes() {
		st, ok := other.Underlying().(*types.Struct)
		if !ok {
			continue
		}
		for i := 0; i < st.NumFields(); i++ {
			f := st.Field(i)
			if !f.Embedded() {
				continue
			}
			base := f.Type()
			if ptr, ok := base.(*types.Pointer); ok {
				base = ptr.Elem()
			}
			if n, ok := base.(*types.Named); ok && SameObject(n.Obj(), named.Obj()) {
				out = append(out, EmbedEntry{
					TypeName: other.Obj().Name(),
					Position: p.ObjPosString(other.Obj()),
				})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TypeName < out[j].TypeName })
	return out
}

// typeObjOf extracts the object of a type (to position external embeds).
func typeObjOf(t types.Type) types.Object {
	switch tt := t.(type) {
	case *types.Named:
		return tt.Obj()
	case *types.Pointer:
		return typeObjOf(tt.Elem())
	case *types.Alias:
		return typeObjOf(tt.Rhs())
	}
	return nil
}

// CallSite is a caller or callee with location and context.
type CallSite struct {
	Function string
	Position string
	Context  string // trimmed source line
}

// CallGraph is the local call graph of a function: callers and callees.
type CallGraph struct {
	FuncName string
	Position string
	Callers  []CallSite
	Callees  []CallSite
}

// CallGraphOf builds the call graph of a function/method.
//
// Callers: identifiers whose Uses denote the object (SameObject matches
// test variants). Callees: CallExprs inside the object's FuncDecl,
// resolved via Uses (includes calls to stdlib/deps, marked external).
func (p *Project) CallGraphOf(funcName string) (*CallGraph, bool) {
	var target *types.Func
	for _, obj := range p.Lookup(funcName) {
		if fn, ok := obj.(*types.Func); ok {
			target = fn
			break
		}
	}
	if target == nil {
		return nil, false
	}

	cg := &CallGraph{
		FuncName: target.Name(),
		Position: p.ObjPosString(target),
	}

	seenCallers := map[token.Pos]bool{}
	seenCallees := map[string]bool{}

	for _, pkg := range p.Pkgs {
		if pkg.TypesInfo == nil {
			continue
		}
		for _, file := range pkg.Syntax {
			ast.Inspect(file, func(n ast.Node) bool {
				switch node := n.(type) {
			case *ast.Ident:
				// Caller: usage of the target object inside a FuncDecl.
					if used, ok := pkg.TypesInfo.Uses[node]; ok && SameObject(used, target) {
						pos := node.Pos()
						if !seenCallers[pos] {
							seenCallers[pos] = true
							caller := "(package level)"
							if fd := EnclosingFunc(file, pos); fd != nil {
								caller = FuncDisplayName(fd)
							}
							cg.Callers = append(cg.Callers, CallSite{
								Function: caller,
								Position: p.PosString(pos),
								Context:  p.LineAt(pos),
							})
						}
					}
			case *ast.FuncDecl:
				// Callees: the body of the target itself.
					if decl, ok := pkg.TypesInfo.Defs[node.Name]; ok && SameObject(decl, target) && node.Body != nil {
						p.collectCallees(pkg, node.Body, cg, seenCallees)
					}
				}
				return true
			})
		}
	}

	sortCallSites(cg.Callers)
	sortCallSites(cg.Callees)
	return cg, true
}

// collectCallees walks the body and adds called functions.
func (p *Project) collectCallees(pkg *packages.Package, body *ast.BlockStmt, cg *CallGraph, seen map[string]bool) {
	if pkg.TypesInfo == nil {
		return
	}
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		var obj types.Object
		switch fun := call.Fun.(type) {
		case *ast.Ident:
			obj = pkg.TypesInfo.Uses[fun]
			if obj == nil {
				obj = pkg.TypesInfo.Defs[fun]
			}
		case *ast.SelectorExpr:
			obj = pkg.TypesInfo.Uses[fun.Sel]
		}
		if fn, ok := obj.(*types.Func); ok {
			key := fn.FullName() + "|" + p.PosString(call.Pos())
			if !seen[key] {
				seen[key] = true
				cg.Callees = append(cg.Callees, CallSite{
					Function: calleeName(fn),
					Position: p.ObjPosString(fn),
					Context:  p.LineAt(call.Pos()),
				})
			}
		}
		return true
	})
}

// calleeName renders `Name`, `Type.Name` (methods) or `pkg.Name` (external).
func calleeName(fn *types.Func) string {
	if sig, ok := fn.Type().(*types.Signature); ok && sig.Recv() != nil {
		base := sig.Recv().Type()
		if ptr, ok := base.(*types.Pointer); ok {
			base = ptr.Elem()
		}
		if named, ok := base.(*types.Named); ok {
			return named.Obj().Name() + "." + fn.Name()
		}
	}
	if fn.Pkg() != nil {
		return fn.Pkg().Name() + "." + fn.Name()
	}
	return fn.Name()
}

// SemanticRef is a semantically resolved reference to a symbol.
type SemanticRef struct {
	Symbol   string
	Kind     string // func | method | type | const | var
	Position string
	Function string // enclosing function, or "(package level)"
	Context  string
}

// FindSemanticRefs finds all semantic references to a symbol
// in the project using go/types (Uses + Defs). It is the semantic version of
// find_references (vs textual whole-word).
func (p *Project) FindSemanticRefs(symbol string, maxResults int) []SemanticRef {
	objs := p.Lookup(symbol)
	if len(objs) == 0 {
		return nil
	}

	seen := map[token.Pos]bool{}
	var out []SemanticRef

	for _, target := range objs {
		for _, pkg := range p.Pkgs {
			if pkg.TypesInfo == nil {
				continue
			}
			for _, file := range pkg.Syntax {
				ast.Inspect(file, func(n ast.Node) bool {
					id, ok := n.(*ast.Ident)
					if !ok {
						return true
					}
					used, ok := pkg.TypesInfo.Uses[id]
					if !ok || !SameObject(used, target) {
						return true
					}
					pos := id.Pos()
					if !seen[pos] {
						seen[pos] = true
						funcName := "(package level)"
						if fd := EnclosingFunc(file, pos); fd != nil {
							funcName = FuncDisplayName(fd)
						}
						out = append(out, SemanticRef{
							Symbol:   target.Name(),
							Kind:     kindOf(target),
							Position: p.PosString(pos),
							Function: funcName,
							Context:  p.LineAt(pos),
						})
						if maxResults > 0 && len(out) >= maxResults {
							return false
						}
					}
					return true
				})
				if maxResults > 0 && len(out) >= maxResults {
					break
				}
			}
			if maxResults > 0 && len(out) >= maxResults {
				break
			}
		}
		if maxResults > 0 && len(out) >= maxResults {
			break
		}
	}
	return out
}

// Semantic BugFinding (iter 4).
type SemBugFinding struct {
	Kind     string // unchecked_error | dead_code | goroutine_leak
	Severity string // critical | high | medium | low
	Message  string
	Position string
	Context  string
}

// FindUncheckedErrors finds calls to functions that return an error
// whose result is not assigned or checked, using go/types to
// verify that the last return type is error.
func (p *Project) FindUncheckedErrors() []SemBugFinding {
	var out []SemBugFinding
	for _, pkg := range p.Pkgs {
		if pkg.TypesInfo == nil {
			continue
		}
		for _, file := range pkg.Syntax {
			for _, decl := range file.Decls {
				fd, ok := decl.(*ast.FuncDecl)
				if !ok || fd.Body == nil {
					continue
				}
				p.findUncheckedInBlock(pkg, fd.Body, &out)
			}
		}
	}
	// Final dedup by position (go/packages may duplicate the same AST
	// across package variants with Tests:true).
	out = dedupSemFindings(out)
	return out
}

func dedupSemFindings(in []SemBugFinding) []SemBugFinding {
	seen := map[string]bool{}
	out := make([]SemBugFinding, 0, len(in))
	for _, f := range in {
		key := f.Position
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, f)
	}
	return out
}

func (p *Project) findUncheckedInBlock(pkg *packages.Package, block *ast.BlockStmt, out *[]SemBugFinding) {
	for _, stmt := range block.List {
		exprStmt, ok := stmt.(*ast.ExprStmt)
		if !ok {
			// Search in sub-blocks (if, for, switch, etc.)
			p.recurseBlocks(pkg, stmt, out)
			continue
		}
		call, ok := exprStmt.X.(*ast.CallExpr)
		if !ok {
			continue
		}
		// Get the signature of the called function.
		sig, ok := pkg.TypesInfo.TypeOf(call.Fun).(*types.Signature)
		if !ok || sig.Results() == nil || sig.Results().Len() == 0 {
			continue
		}
		lastRes := sig.Results().At(sig.Results().Len() - 1)
		errType := types.Universe.Lookup("error").Type()
		if !types.Identical(lastRes.Type(), errType) {
			continue
		}
		*out = append(*out, SemBugFinding{
			Kind:     "unchecked_error",
			Severity: "high",
			Message:  "unchecked error return from " + types.ExprString(call.Fun),
			Position: p.PosString(call.Pos()),
			Context:  p.LineAt(call.Pos()),
		})
	}
}

func (p *Project) recurseBlocks(pkg *packages.Package, stmt ast.Stmt, out *[]SemBugFinding) {
	switch s := stmt.(type) {
	case *ast.IfStmt:
		if s.Body != nil {
			p.findUncheckedInBlock(pkg, s.Body, out)
		}
		if s.Else != nil {
			if elseBlock, ok := s.Else.(*ast.BlockStmt); ok {
				p.findUncheckedInBlock(pkg, elseBlock, out)
			}
		}
	case *ast.ForStmt:
		if s.Body != nil {
			p.findUncheckedInBlock(pkg, s.Body, out)
		}
	case *ast.RangeStmt:
		if s.Body != nil {
			p.findUncheckedInBlock(pkg, s.Body, out)
		}
	case *ast.SwitchStmt:
		if s.Body != nil {
			for _, cc := range s.Body.List {
				if c, ok := cc.(*ast.CaseClause); ok {
					for _, caseStmt := range c.Body {
						p.recurseBlocks(pkg, caseStmt, out)
					}
				}
			}
		}
	case *ast.BlockStmt:
		p.findUncheckedInBlock(pkg, s, out)
	}
}

// FindDeadCode identifies non-exported functions and types that are never
// referenced in the project. Similar to `unused` in rust-mcp-server.
// Only reports symbols with 0 references (Uses) across the project.
func (p *Project) FindDeadCode() []SemBugFinding {
	// Collect all function and type objects (non-exported).
	type candidate struct {
		obj  types.Object
		pkg  *packages.Package
		file *ast.File
	}
	var cands []candidate

	for _, pkg := range p.Pkgs {
		if pkg.Types == nil || pkg.TypesInfo == nil {
			continue
		}
		scope := pkg.Types.Scope()
		for _, name := range scope.Names() {
			obj := scope.Lookup(name)
			if obj == nil || obj.Exported() {
				continue
			}
			// Only functions, types, const, var.
			switch obj.(type) {
			case *types.Func, *types.TypeName, *types.Var, *types.Const:
			default:
				continue
			}
			// Find the AST file that defines this object.
			for _, file := range pkg.Syntax {
				for _, decl := range file.Decls {
					if fd, ok := decl.(*ast.FuncDecl); ok {
						if pkg.TypesInfo.Defs[fd.Name] == obj {
							cands = append(cands, candidate{obj: obj, pkg: pkg, file: file})
						}
					}
					if gd, ok := decl.(*ast.GenDecl); ok {
						for _, spec := range gd.Specs {
							if ts, ok := spec.(*ast.TypeSpec); ok {
								if pkg.TypesInfo.Defs[ts.Name] == obj {
									cands = append(cands, candidate{obj: obj, pkg: pkg, file: file})
								}
							}
							if vs, ok := spec.(*ast.ValueSpec); ok {
								for _, n := range vs.Names {
									if pkg.TypesInfo.Defs[n] == obj {
										cands = append(cands, candidate{obj: obj, pkg: pkg, file: file})
									}
								}
							}
						}
					}
				}
			}
		}
	}

	// Count references for each candidate.
	refCount := map[types.Object]int{}
	for _, pkg := range p.Pkgs {
		if pkg.TypesInfo == nil {
			continue
		}
		for _, file := range pkg.Syntax {
			ast.Inspect(file, func(n ast.Node) bool {
				id, ok := n.(*ast.Ident)
				if !ok {
					return true
				}
				used, ok := pkg.TypesInfo.Uses[id]
				if !ok {
					return true
				}
				refCount[used]++
				return true
			})
		}
	}

	var out []SemBugFinding
	for _, cand := range cands {
		obj := cand.obj
		if refCount[obj] > 0 {
			continue
		}
		// init() and main() are not counted as dead — runtime.
		if fn, ok := obj.(*types.Func); ok && (fn.Name() == "init" || fn.Name() == "main") {
			continue
		}
		out = append(out, SemBugFinding{
			Kind:     "dead_code",
			Severity: "medium",
			Message:  "unused symbol: " + kindOf(obj) + " " + obj.Name(),
			Position: p.ObjPosString(obj),
			Context:  p.LineAt(obj.Pos()),
		})
	}
	out = dedupSemFindings(out)
	return out
}

// FindGoroutineLeaks heuristic: finds goroutines (go f()) whose body
// uses no synchronization mechanisms (sync.WaitGroup, channel ops,
// sync.Mutex, atomic), indicating a potential leak.
func (p *Project) FindGoroutineLeaks() []SemBugFinding {
	var out []SemBugFinding
	for _, pkg := range p.Pkgs {
		if pkg.TypesInfo == nil {
			continue
		}
		for _, file := range pkg.Syntax {
			ast.Inspect(file, func(n ast.Node) bool {
				goStmt, ok := n.(*ast.GoStmt)
				if !ok {
					return true
				}
				// Extract the called function name.
				callee := types.ExprString(goStmt.Call.Fun)

				// Check if it is an anonymous function.
				if _, ok := goStmt.Call.Fun.(*ast.FuncLit); ok {
					if !hasSyncMechanism(goStmt.Call.Fun) {
						declFile, declLine := p.fileLine(goStmt.Pos())
						out = append(out, SemBugFinding{
							Kind:     "goroutine_leak",
							Severity: "low",
							Message:  "goroutine with anonymous function may leak (no sync mechanism detected in body)",
							Position: declFile + ":" + itoa(declLine),
							Context:  p.LineAt(goStmt.Pos()),
						})
					}
					return true
				}

				// Named function: find its definition and check body.
				target := p.resolveFunc(goStmt.Call)
				if target == nil {
					return true
				}
				if !hasSyncInFunc(p, target) {
					declFile, declLine := p.fileLine(goStmt.Pos())
					out = append(out, SemBugFinding{
						Kind:     "goroutine_leak",
						Severity: "low",
						Message:  "goroutine with " + callee + " may leak (no sync mechanism detected)",
						Position: declFile + ":" + itoa(declLine),
						Context:  p.LineAt(goStmt.Pos()),
					})
				}
				return true
			})
		}
	}
	return out
}

// resolveFunc finds the FuncDecl for the called function in a CallExpr.
func (p *Project) resolveFunc(call *ast.CallExpr) *ast.FuncDecl {
	var name string
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		name = fun.Name
	case *ast.SelectorExpr:
		name = fun.Sel.Name
	default:
		return nil
	}
	for _, pkg := range p.Pkgs {
		for _, file := range pkg.Syntax {
			for _, decl := range file.Decls {
				if fd, ok := decl.(*ast.FuncDecl); ok && fd.Name.Name == name {
					return fd
				}
			}
		}
	}
	return nil
}

// hasSyncInFunc checks whether a func has synchronization.
func hasSyncInFunc(p *Project, fd *ast.FuncDecl) bool {
	if fd.Body == nil {
		return false
	}
	return hasSyncInBody(fd.Body)
}

// hasSyncMechanism inspects an AST node for synchronization
// mechanisms: channels, sync.WaitGroup, sync.Mutex, atomic.
func hasSyncMechanism(n ast.Node) bool {
	found := false
	ast.Inspect(n, func(child ast.Node) bool {
		if found {
			return false
		}
		switch node := child.(type) {
		case *ast.SendStmt: // ch <-
			found = true
			return false
		case *ast.UnaryExpr:
			if node.Op == token.ARROW { // <-ch
				found = true
				return false
			}
		case *ast.CallExpr:
			fun := types.ExprString(node.Fun)
			switch {
			case strings.Contains(fun, "sync.WaitGroup"),
				strings.Contains(fun, "sync.Mutex"),
				strings.Contains(fun, "sync.RWMutex"),
				strings.Contains(fun, "sync.Once"),
				strings.Contains(fun, "atomic."),
				strings.Contains(fun, "make(chan"),
				strings.Contains(fun, "close("):
				found = true
				return false
			}
		case *ast.SelectStmt: // select { case <-ch: ... }
			found = true
			return false
		}
		return true
	})
	return found
}

// hasSyncInBody is a wrapper to search in a body.
func hasSyncInBody(body *ast.BlockStmt) bool {
	return hasSyncMechanism(body)
}

// fileLine returns (file, line) of a position.
// Similar to PosString but without backticks.
func (p *Project) fileLine(pos token.Pos) (string, int) {
	tp := p.Fset.Position(pos)
	if !pos.IsValid() || tp.Filename == "" {
		return "-", 0
	}
	rel, err := filepath.Rel(p.Root, tp.Filename)
	if err != nil || strings.HasPrefix(rel, "..") {
		return tp.Filename, tp.Line
	}
	return filepath.ToSlash(rel), tp.Line
}

// itoa converts int to string for PosString helper.
func itoa(n int) string {
	return strconv.Itoa(n)
}

func sortCallSites(s []CallSite) {
	sort.Slice(s, func(i, j int) bool {
		if s[i].Function != s[j].Function {
			return s[i].Function < s[j].Function
		}
		return s[i].Position < s[j].Position
	})
}

// FieldRef is a reference to a struct field.
type FieldRef struct {
	StructName string // struct where the field is declared
	Position   string
	Context    string
}

// FieldXRef finds declarations of the field `fieldName` in project
// structs and all their semantic references (Uses), with context.
// If multiple structs have a field with that name, all are reported
// (same as struct_field_xref in rust-mcp-server).
func (p *Project) FieldXRef(fieldName string) (decls []FieldRef, refs []FieldRef) {
	var fieldObjs []types.Object

	for _, named := range p.NamedProjectTypes() {
		st, ok := named.Underlying().(*types.Struct)
		if !ok {
			continue
		}
		for i := 0; i < st.NumFields(); i++ {
			f := st.Field(i)
			if f.Name() != fieldName {
				continue
			}
			fieldObjs = append(fieldObjs, f)
			decls = append(decls, FieldRef{
				StructName: named.Obj().Name(),
				Position:   p.ObjPosString(f),
				Context:    p.LineAt(f.Pos()),
			})
		}
	}

	if len(fieldObjs) == 0 {
		return decls, nil
	}

	for _, pkg := range p.Pkgs {
		if pkg.TypesInfo == nil {
			continue
		}
		for _, file := range pkg.Syntax {
			ast.Inspect(file, func(n ast.Node) bool {
				id, ok := n.(*ast.Ident)
				if !ok {
					return true
				}
				used, ok := pkg.TypesInfo.Uses[id]
				if !ok {
					return true
				}
				for _, fobj := range fieldObjs {
					if !SameObject(used, fobj) {
						continue
					}
					pos := id.Pos()
					owner := ""
					if fd := EnclosingFunc(file, pos); fd != nil {
						owner = FuncDisplayName(fd)
					}
					refs = append(refs, FieldRef{
						StructName: owner,
						Position:   p.PosString(pos),
						Context:    p.LineAt(pos),
					})
				}
				return true
			})
		}
	}
	// Final dedup by Position (same reason as FindUncheckedErrors).
	seen := map[string]bool{}
	deduped := make([]FieldRef, 0, len(refs))
	for _, r := range refs {
		if seen[r.Position] {
			continue
		}
		seen[r.Position] = true
		deduped = append(deduped, r)
	}
	sort.Slice(deduped, func(i, j int) bool { return deduped[i].Position < deduped[j].Position })
	return decls, deduped
}

// UnimplementedInterface describes an interface with no implementation.
type UnimplementedInterface struct {
	Name     string
	Methods  int
	Position string
}

// FindUnimplementedInterfaces returns project interfaces that no
// concrete type implements. Skips empty interfaces (0 methods).
func (p *Project) FindUnimplementedInterfaces() []UnimplementedInterface {
	var out []UnimplementedInterface
	for _, named := range p.NamedProjectTypes() {
		tn := named.Obj()
		// Only project interfaces (not stdlib, not dependencies).
		if !tn.Pos().IsValid() {
			continue
		}
		iface, ok := named.Underlying().(*types.Interface)
		if !ok {
			continue
		}
		// Skip empty interfaces (any / interface{}).
		if iface.NumMethods() == 0 && iface.NumExplicitMethods() == 0 {
			continue
		}
		// Check if anyone implements it.
		hasImpl := false
		for _, other := range p.NamedProjectTypes() {
			if SameObject(other.Obj(), tn) {
				continue
			}
			if types.Implements(other, iface) || types.Implements(types.NewPointer(other), iface) {
				hasImpl = true
				break
			}
		}
		if !hasImpl {
			out = append(out, UnimplementedInterface{
				Name:     tn.Name(),
				Methods:  iface.NumExplicitMethods(),
				Position: p.ObjPosString(tn),
			})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
