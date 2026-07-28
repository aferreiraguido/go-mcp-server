// snapshots_test.go — golden snapshot tests (determinism enforced).
//
// Counterpart of `tests/snapshots_test.rs` (rust-mcp-server, which uses `insta`).
// Here the harness is the idiomatic Go golden-file pattern:
//
//   - Each test executes a tool and compares the Markdown BYTE-BY-BYTE against
//     `testdata/snapshots/<name>.snap`.
//   - Regenerate after an intentional change:
//     `go test ./internal/tools/ -update` (the flag only exists in this
//     package; a normal `go test ./...` verifies against the snapshots).
//
// Rules (same as in the family):
//   - Any change that intentionally modifies the output must be
//     accompanied by snapshot regeneration.
//   - Snapshots are part of the contract — a change that affects them
//     requires the same level of review as an API change.
package tools

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"go-mcp-server/internal/model"
	"go-mcp-server/internal/project"
)

var update = flag.Bool("update", false, "update golden snapshots")

// assertSnapshot compares `got` byte-by-byte against snapshot `name`.
// With `-update`, (re)writes the snapshot.
func assertSnapshot(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", "snapshots", name+".snap")

	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir snapshots: %v", err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("write snapshot: %v", err)
		}
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("snapshot %q does not exist; run with -update to create it: %v", name, err)
	}
	if got != string(want) {
		t.Errorf("snapshot %q differs (-want +got):\n--- want ---\n%s\n--- got ---\n%s\n"+
			"If the change is intentional, regenerate with: go test ./internal/tools/ -update",
			name, want, got)
	}
}

// callTool runs a tool and returns the text of the first Content.
// Fails the test if the result is not exactly a TextContent.
func callTool(t *testing.T, tool Tool, args map[string]any) string {
	t.Helper()
	res, err := tool.Call(context.Background(), args)
	if err != nil {
		t.Fatalf("tool %s returned protocol error: %v", tool.Name(), err)
	}
	if res == nil || len(res.Content) != 1 {
		t.Fatalf("tool %s: content = %v, want exactly 1 block", tool.Name(), res)
	}
	tc, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("tool %s: content[0] = %T, want *mcp.TextContent", tool.Name(), res.Content[0])
	}
	return tc.Text
}

// fixedEntry builds an entry with SYNTHETIC and stable path and dates.
// Dates never appear in Markdown (determinism), so it only
// matters that it is not expired.
func fixedEntry(name, alias string, kind model.ProjectKind, modules []string) *model.ProjectEntry {
	return &model.ProjectEntry{
		Name:      name,
		Alias:     alias,
		Path:      "/fixtures/" + name,
		Kind:      kind,
		ExpiresAt: time.Now().Add(time.Hour),
		LoadedAt:  time.Now(),
		Modules:   modules,
	}
}

// loadFixture loads testdata/samplemodule with alias "sample".
// Tool output uses paths RELATIVE to the project root, so
// the snapshots are machine-independent (lesson from rust iter 7+: no
// absolute host paths in snapshots).
func loadFixture(t *testing.T) *project.Manager {
	t.Helper()
	mgr := project.NewManager()
	if _, err := mgr.Load("testdata/samplemodule", "sample", time.Hour); err != nil {
		t.Fatalf("load fixture: %v", err)
	}
	return mgr
}

// nameArg is the most common args map: {"name": "sample"}.
func nameArg() map[string]any { return map[string]any{"name": "sample"} }

// --- list_loaded_projects (iter 1) ---

func TestListLoadedProjectsEmpty(t *testing.T) {
	tool := NewListLoadedProjectsTool(project.NewManager())
	text := callTool(t, tool, map[string]any{})
	assertSnapshot(t, "list_loaded_projects_empty", text)
}

func TestListLoadedProjectsWithOneProject(t *testing.T) {
	mgr := project.NewManager()
	mgr.Add(fixedEntry("sample_module", "sample", model.SingleModule, nil))
	tool := NewListLoadedProjectsTool(mgr)
	text := callTool(t, tool, map[string]any{})
	assertSnapshot(t, "list_loaded_projects_with_one_project", text)
}

func TestListLoadedProjectsWithWorkspace(t *testing.T) {
	mgr := project.NewManager()
	mgr.Add(fixedEntry("sample_ws", "", model.Workspace, []string{"./cmd/app", "./lib/core"}))
	mgr.Add(fixedEntry("sample_module", "sample", model.SingleModule, nil))
	tool := NewListLoadedProjectsTool(mgr)
	text := callTool(t, tool, map[string]any{})
	assertSnapshot(t, "list_loaded_projects_with_workspace", text)
}

// --- source management (iter 2) ---

// load_go_project has NO snapshot: its output includes the absolute path
// (machine-dependent). Validated with unit test in internal/project and with
// content checks here.
func TestLoadGoProjectContent(t *testing.T) {
	mgr := project.NewManager()
	tool := NewLoadGoProjectTool(mgr)
	text := callTool(t, tool, map[string]any{"source": "testdata/samplemodule", "alias": "sample"})
	for _, want := range []string{
		"# Project Loaded", "| Name | `samplemodule` |", "| Alias | `sample` |",
		"| Kind | module |", "| Module | `example.com/samplemodule` |", "| Go | 1.21 |",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("load output does not contain %q:\n%s", want, text)
		}
	}
}

func TestLoadGoProjectNotFound(t *testing.T) {
	tool := NewLoadGoProjectTool(project.NewManager())
	text := callTool(t, tool, map[string]any{"source": "/nonexistent/path/12345"})
	assertSnapshot(t, "load_go_project_not_found", text)
}

func TestUnloadGoProject(t *testing.T) {
	mgr := project.NewManager()
	mgr.Add(fixedEntry("sample_module", "sample", model.SingleModule, nil))
	tool := NewUnloadGoProjectTool(mgr)
	text := callTool(t, tool, map[string]any{"name": "sample"})
	assertSnapshot(t, "unload_go_project", text)
}

func TestUnloadGoProjectNotFound(t *testing.T) {
	tool := NewUnloadGoProjectTool(project.NewManager())
	text := callTool(t, tool, map[string]any{"name": "ghost"})
	assertSnapshot(t, "unload_go_project_not_found", text)
}

func TestProjectMetadata(t *testing.T) {
	mgr := project.NewManager()
	e := fixedEntry("sample_module", "sample", model.SingleModule, nil)
	e.ModulePath = "example.com/samplemodule"
	e.GoVersion = "1.21"
	mgr.Add(e)
	tool := NewProjectMetadataTool(mgr)
	text := callTool(t, tool, map[string]any{"name": "sample"})
	assertSnapshot(t, "project_metadata", text)
}

func TestInspectBuildConfig(t *testing.T) {
	mgr := loadFixture(t)
	text := callTool(t, NewInspectBuildConfigTool(mgr), nameArg())
	assertSnapshot(t, "inspect_build_config", text)
}

// --- module level (iter 2) ---

func TestListPackages(t *testing.T) {
	mgr := loadFixture(t)
	text := callTool(t, NewListPackagesTool(mgr), nameArg())
	assertSnapshot(t, "list_packages", text)
}

func TestListDependencies(t *testing.T) {
	mgr := loadFixture(t)
	text := callTool(t, NewListDependenciesTool(mgr), nameArg())
	assertSnapshot(t, "list_dependencies", text)
}

func TestProgramSummary(t *testing.T) {
	mgr := loadFixture(t)
	text := callTool(t, NewProgramSummaryTool(mgr), nameArg())
	assertSnapshot(t, "program_summary", text)
}

// --- file ops (iter 2) ---

func TestGetFileContent(t *testing.T) {
	mgr := loadFixture(t)
	args := map[string]any{"name": "sample", "rel_path": "greeter.go"}
	text := callTool(t, NewGetFileContentTool(mgr), args)
	assertSnapshot(t, "get_file_content", text)
}

func TestGetFileContentPathTraversalBlocked(t *testing.T) {
	mgr := loadFixture(t)
	args := map[string]any{"name": "sample", "rel_path": "../../../etc/passwd"}
	text := callTool(t, NewGetFileContentTool(mgr), args)
	assertSnapshot(t, "get_file_content_path_traversal_blocked", text)
}

func TestSearchSource(t *testing.T) {
	mgr := loadFixture(t)
	args := map[string]any{"name": "sample", "query": "greeter", "case_sensitive": false}
	text := callTool(t, NewSearchSourceTool(mgr), args)
	assertSnapshot(t, "search_source_case_insensitive", text)
}

// --- source listings (iter 2) ---

func TestListFunctions(t *testing.T) {
	mgr := loadFixture(t)
	text := callTool(t, NewListFunctionsTool(mgr), nameArg())
	assertSnapshot(t, "list_functions", text)
}

func TestListFunctionsExportedOnly(t *testing.T) {
	mgr := loadFixture(t)
	args := map[string]any{"name": "sample", "visibility": "exported"}
	text := callTool(t, NewListFunctionsTool(mgr), args)
	assertSnapshot(t, "list_functions_exported", text)
}

func TestListStructs(t *testing.T) {
	mgr := loadFixture(t)
	text := callTool(t, NewListStructsTool(mgr), nameArg())
	assertSnapshot(t, "list_structs", text)
}

func TestListInterfaces(t *testing.T) {
	mgr := loadFixture(t)
	text := callTool(t, NewListInterfacesTool(mgr), nameArg())
	assertSnapshot(t, "list_interfaces", text)
}

func TestListConstants(t *testing.T) {
	mgr := loadFixture(t)
	text := callTool(t, NewListConstantsTool(mgr), nameArg())
	assertSnapshot(t, "list_constants", text)
}

func TestListVariables(t *testing.T) {
	mgr := loadFixture(t)
	text := callTool(t, NewListVariablesTool(mgr), nameArg())
	assertSnapshot(t, "list_variables", text)
}

func TestListImports(t *testing.T) {
	mgr := loadFixture(t)
	text := callTool(t, NewListImportsTool(mgr), nameArg())
	assertSnapshot(t, "list_imports", text)
}

// --- inspections (iter 2) ---

func TestInspectFunction(t *testing.T) {
	mgr := loadFixture(t)
	args := map[string]any{"name": "sample", "function_name": "Greet"}
	text := callTool(t, NewInspectFunctionTool(mgr), args)
	assertSnapshot(t, "inspect_function", text)
}

func TestInspectFunctionNotFound(t *testing.T) {
	mgr := loadFixture(t)
	args := map[string]any{"name": "sample", "function_name": "Ghost"}
	text := callTool(t, NewInspectFunctionTool(mgr), args)
	assertSnapshot(t, "inspect_function_not_found", text)
}

func TestInspectStruct(t *testing.T) {
	mgr := loadFixture(t)
	args := map[string]any{"name": "sample", "struct_name": "Greeter"}
	text := callTool(t, NewInspectStructTool(mgr), args)
	assertSnapshot(t, "inspect_struct", text)
}

func TestInspectInterface(t *testing.T) {
	mgr := loadFixture(t)
	args := map[string]any{"name": "sample", "interface_name": "Speaker"}
	text := callTool(t, NewInspectInterfaceTool(mgr), args)
	assertSnapshot(t, "inspect_interface", text)
}

// --- cross-references (iter 2) ---

func TestValidateCodeReferenceExists(t *testing.T) {
	mgr := loadFixture(t)
	args := map[string]any{"name": "sample", "symbol": "Greeter"}
	text := callTool(t, NewValidateCodeReferenceTool(mgr), args)
	assertSnapshot(t, "validate_code_reference_exists", text)
}

func TestValidateCodeReferenceNotFound(t *testing.T) {
	mgr := loadFixture(t)
	args := map[string]any{"name": "sample", "symbol": "Ghost"}
	text := callTool(t, NewValidateCodeReferenceTool(mgr), args)
	assertSnapshot(t, "validate_code_reference_not_found", text)
}

func TestFindReferencesSemantic(t *testing.T) {
	mgr := loadFixture(t)
	args := map[string]any{"name": "sample", "symbol": "Greet", "semantic": true}
	text := callTool(t, NewFindReferencesTool(mgr), args)
	assertSnapshot(t, "find_references_semantic", text)
}

func TestFindReferencesTextual(t *testing.T) {
	mgr := loadFixture(t)
	args := map[string]any{"name": "sample", "symbol": "Greet", "semantic": false}
	text := callTool(t, NewFindReferencesTool(mgr), args)
	assertSnapshot(t, "find_references_textual", text)
}

func TestFindReferencesSemanticNotFound(t *testing.T) {
	mgr := loadFixture(t)
	args := map[string]any{"name": "sample", "symbol": "GhostFunc", "semantic": true}
	text := callTool(t, NewFindReferencesTool(mgr), args)
	assertSnapshot(t, "find_references_semantic_not_found", text)
}

func TestFunctionCallers(t *testing.T) {
	mgr := loadFixture(t)
	args := map[string]any{"name": "sample", "symbol": "NewGreeter"}
	text := callTool(t, NewFunctionCallersTool(mgr), args)
	assertSnapshot(t, "function_callers", text)
}

func TestFindEntryPoints(t *testing.T) {
	mgr := loadFixture(t)
	text := callTool(t, NewFindEntryPointsTool(mgr), nameArg())
	assertSnapshot(t, "find_entry_points", text)
}

// --- error shape compartido ---

func TestProjectNotFoundErrorShape(t *testing.T) {
	mgr := project.NewManager()
	text := callTool(t, NewListFunctionsTool(mgr), map[string]any{"name": "ghost"})
	assertSnapshot(t, "project_not_found_error", text)
}

// --- semantic layer: resolve_type (iter 3) ---

func TestResolveTypeFunction(t *testing.T) {
	mgr := loadFixture(t)
	args := map[string]any{"name": "sample", "symbol": "NewGreeter"}
	text := callTool(t, NewResolveTypeTool(mgr), args)
	assertSnapshot(t, "resolve_type_function", text)
}

func TestResolveTypeNotFound(t *testing.T) {
	mgr := loadFixture(t)
	args := map[string]any{"name": "sample", "symbol": "GhostSymbol"}
	text := callTool(t, NewResolveTypeTool(mgr), args)
	assertSnapshot(t, "resolve_type_not_found", text)
}

// --- semantic layer: find_implementations (iter 3) ---

func TestFindImplementations(t *testing.T) {
	mgr := loadFixture(t)
	args := map[string]any{"name": "sample", "interface_name": "Speaker"}
	text := callTool(t, NewFindImplementationsTool(mgr), args)
	assertSnapshot(t, "find_implementations", text)
}

func TestFindImplementationsNotFound(t *testing.T) {
	mgr := loadFixture(t)
	args := map[string]any{"name": "sample", "interface_name": "NonExistent"}
	text := callTool(t, NewFindImplementationsTool(mgr), args)
	assertSnapshot(t, "find_implementations_not_found", text)
}

// --- semantic layer: function_call_graph (iter 3) ---

func TestFunctionCallGraph(t *testing.T) {
	mgr := loadFixture(t)
	args := map[string]any{"name": "sample", "function_name": "Greet"}
	text := callTool(t, NewFunctionCallGraphTool(mgr), args)
	assertSnapshot(t, "function_call_graph", text)
}

func TestFunctionCallGraphNotFound(t *testing.T) {
	mgr := loadFixture(t)
	args := map[string]any{"name": "sample", "function_name": "GhostFunc"}
	text := callTool(t, NewFunctionCallGraphTool(mgr), args)
	assertSnapshot(t, "function_call_graph_not_found", text)
}

// --- semantic layer: struct_field_xref (iter 3) ---

func TestStructFieldXRef(t *testing.T) {
	mgr := loadFixture(t)
	args := map[string]any{"name": "sample", "field_name": "Name"}
	text := callTool(t, NewStructFieldXRefTool(mgr), args)
	assertSnapshot(t, "struct_field_xref", text)
}

func TestStructFieldXRefNotFound(t *testing.T) {
	mgr := loadFixture(t)
	args := map[string]any{"name": "sample", "field_name": "GhostField"}
	text := callTool(t, NewStructFieldXRefTool(mgr), args)
	assertSnapshot(t, "struct_field_xref_not_found", text)
}

// --- semantic layer: get_type_hierarchy (iter 3) ---

func TestGetTypeHierarchyInterface(t *testing.T) {
	mgr := loadFixture(t)
	args := map[string]any{"name": "sample", "type_name": "Speaker"}
	text := callTool(t, NewGetTypeHierarchyTool(mgr), args)
	assertSnapshot(t, "get_type_hierarchy_interface", text)
}

func TestGetTypeHierarchyStruct(t *testing.T) {
	mgr := loadFixture(t)
	args := map[string]any{"name": "sample", "type_name": "Greeter"}
	text := callTool(t, NewGetTypeHierarchyTool(mgr), args)
	assertSnapshot(t, "get_type_hierarchy_struct", text)
}

func TestGetTypeHierarchyNotFound(t *testing.T) {
	mgr := loadFixture(t)
	args := map[string]any{"name": "sample", "type_name": "GhostType"}
	text := callTool(t, NewGetTypeHierarchyTool(mgr), args)
	assertSnapshot(t, "get_type_hierarchy_not_found", text)
}

// --- bug pattern analysis: find_panic_sites (iter 4) ---

func TestFindPanicSites(t *testing.T) {
	mgr := loadFixture(t)
	text := callTool(t, NewFindPanicSitesTool(mgr), nameArg())
	assertSnapshot(t, "find_panic_sites", text)
}

// --- bug pattern analysis: find_unsafe_patterns (iter 4) ---

func TestFindUnsafePatterns(t *testing.T) {
	mgr := loadFixture(t)
	text := callTool(t, NewFindUnsafePatternsTool(mgr), nameArg())
	assertSnapshot(t, "find_unsafe_patterns", text)
}

// --- bug pattern analysis: find_defer_in_loops (iter 4) ---

func TestFindDeferInLoops(t *testing.T) {
	mgr := loadFixture(t)
	text := callTool(t, NewFindDeferInLoopsTool(mgr), nameArg())
	assertSnapshot(t, "find_defer_in_loops", text)
}

// --- bug pattern analysis: find_unchecked_errors (iter 4) ---

func TestFindUncheckedErrors(t *testing.T) {
	mgr := loadFixture(t)
	text := callTool(t, NewFindUncheckedErrorsTool(mgr), nameArg())
	assertSnapshot(t, "find_unchecked_errors", text)
}

// --- bug pattern analysis: find_goroutine_leaks (iter 4) ---

func TestFindGoroutineLeaks(t *testing.T) {
	mgr := loadFixture(t)
	text := callTool(t, NewFindGoroutineLeaksTool(mgr), nameArg())
	assertSnapshot(t, "find_goroutine_leaks", text)
}

// --- bug pattern analysis: find_dead_code (iter 4) ---

func TestFindDeadCode(t *testing.T) {
	mgr := loadFixture(t)
	text := callTool(t, NewFindDeadCodeTool(mgr), nameArg())
	assertSnapshot(t, "find_dead_code", text)
}

// --- domain analysis: detect_framework (iter 5) ---

func TestDetectFramework(t *testing.T) {
	mgr := loadFixture(t)
	text := callTool(t, NewDetectFrameworkTool(mgr), nameArg())
	assertSnapshot(t, "detect_framework", text)
}

// --- domain analysis: list_api_endpoints (iter 5) ---

func TestListAPIEndpoints(t *testing.T) {
	mgr := loadFixture(t)
	text := callTool(t, NewListAPIEndpointsTool(mgr), nameArg())
	assertSnapshot(t, "list_api_endpoints", text)
}

// --- domain analysis: extract_environment_dependencies (iter 5) ---

func TestExtractEnvironmentDependencies(t *testing.T) {
	mgr := loadFixture(t)
	text := callTool(t, NewExtractEnvironmentDependenciesTool(mgr), nameArg())
	assertSnapshot(t, "extract_environment_dependencies", text)
}

// --- project exploration: list_directories (iter 5) ---

func TestListDirectories(t *testing.T) {
	mgr := loadFixture(t)
	text := callTool(t, NewListDirectoriesTool(mgr), nameArg())
	assertSnapshot(t, "list_directories", text)
}

// --- project exploration: project_files (iter 5) ---

func TestProjectFiles(t *testing.T) {
	mgr := loadFixture(t)
	text := callTool(t, NewProjectFilesTool(mgr), nameArg())
	assertSnapshot(t, "project_files", text)
}

// --- code quality: find_long_functions (iter 5) ---

func TestFindLongFunctions(t *testing.T) {
	mgr := loadFixture(t)
	text := callTool(t, NewFindLongFunctionsTool(mgr), nameArg())
	assertSnapshot(t, "find_long_functions", text)
}

func TestFindLongFunctionsThresholdLow(t *testing.T) {
	mgr := loadFixture(t)
	args := map[string]any{"name": "sample", "max_lines": 1}
	text := callTool(t, NewFindLongFunctionsTool(mgr), args)
	assertSnapshot(t, "find_long_functions_threshold_1", text)
}

// --- code quality: find_high_cyclomatic_complexity (iter 5) ---

func TestFindHighCyclomaticComplexity(t *testing.T) {
	mgr := loadFixture(t)
	text := callTool(t, NewFindHighCyclomaticComplexityTool(mgr), nameArg())
	assertSnapshot(t, "find_high_cyclomatic_complexity", text)
}

// --- code quality: find_unused_parameters (iter 5) ---

func TestFindUnusedParameters(t *testing.T) {
	mgr := loadFixture(t)
	text := callTool(t, NewFindUnusedParametersTool(mgr), nameArg())
	assertSnapshot(t, "find_unused_parameters", text)
}

// --- code quality: find_unimplemented_interfaces (iter 5) ---

func TestFindUnimplementedInterfaces(t *testing.T) {
	mgr := loadFixture(t)
	text := callTool(t, NewFindUnimplementedInterfacesTool(mgr), nameArg())
	assertSnapshot(t, "find_unimplemented_interfaces", text)
}
