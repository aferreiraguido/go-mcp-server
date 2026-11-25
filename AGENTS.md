# AGENTS.md — go-mcp-server

> Operational manual for agents (human or LLM) working on this repository.
> This file **does not merely describe the project**: it defines **how** you should approach it.

---

## 1. Purpose

MCP (Model Context Protocol) server written in Go that analyzes Go projects
via the stdlib parser (`go/ast` + `go/parser`, iter 2) and the official
semantic layer (`golang.org/x/tools/go/packages` + `go/types`, iter 3). Exposes **43 tools**
that return **deterministic Markdown** ready for LLM consumption.

It is the Go counterpart of `cobol-mcp-server`, `java-mcp-server`,
`python-mcp-server` and `rust-mcp-server`: same philosophy ("ground truth",
no hallucination), different domain and different parser (stdlib + go/types
instead of ProLeap/Spoon/LibCST/ra_ap_*).

> ✅ **Status**: **iteration 6 completed** — dual transport (`stdio` default + Streamable `http`
> on `/mcp` with `--mode/--port/--host/--max-threads/--api-keys` and Bearer token auth;
> loader (path/Git/archive), static analysis with `go/ast` + `x/mod/modfile`, **43 tools**
> (23 iter 2 + 5 semantic + 6 bug pattern + 3 domain + 6 iter 5). **72 tests** (56 golden
> snapshots + 16 unit/integration + 1 content). CI workflow + Dockerfile multi-stage.

---

## 1.1 Current tools

| Group | Tools |
|-------|-------|
| Lifecycle | `list_loaded_projects`, `load_go_project`, `unload_go_project`, `project_metadata` |
| Module | `inspect_build_config`, `list_packages`, `list_dependencies`, `program_summary` |
| Files | `get_file_content`, `search_source` |
| AST listings | `list_functions`, `list_structs`, `list_interfaces`, `list_constants`, `list_variables`, `list_imports` |
| Inspection | `inspect_function`, `inspect_struct`, `inspect_interface` |
| Textual xref | `validate_code_reference`, `find_references`, `function_callers` |
| Semantic (go/types) | `resolve_type`, `find_implementations`, `function_call_graph`, `struct_field_xref`, `get_type_hierarchy` |
| Bug patterns (iter 4) | `find_panic_sites`, `find_unsafe_patterns`, `find_defer_in_loops`, `find_unchecked_errors`, `find_goroutine_leaks`, `find_dead_code` |
| Domain (iter 2/5) | `find_entry_points`, `detect_framework`, `list_api_endpoints`, `extract_environment_dependencies` |
| Project exploration (iter 5) | `list_directories`, `project_files` |
| Code quality (iter 5) | `find_long_functions`, `find_high_cyclomatic_complexity`, `find_unused_parameters`, `find_unimplemented_interfaces` |

---

## 2. Stack and commands

- **Go 1.26+** (Homebrew; nothing else needed, self-contained toolchain)
- Build: `go build -o go-mcp-server .` → `./go-mcp-server` (~8 MB, static)
- Compile check: `go vet ./...`
- Test: `go test ./...` (65 tests: 56 golden snapshots + 9 unit + 1 content)
- Regenerate snapshots: `go test ./internal/tools/ -update`
- Smoke test end-to-end: `./scripts/smoke.sh`
- Version: managed in `version.txt` (read via `//go:embed` in `main.go`,
  **do not hardcode**). Current: **0.2.0**
- Key dependencies:
  - `github.com/modelcontextprotocol/go-sdk` v1.6.1 (official) — package `mcp`
  - `golang.org/x/mod` v0.38.0 — `modfile` (go.mod/go.work parsing)
  - `go/ast` + `go/parser` + `go/printer` (stdlib, iter 2)
- Transport:
  - Default: `stdio` (`mcp.StdioTransport`, JSON newline-delimited, no reflection)
  - HTTP mode: Streamable HTTP (`mcp.StreamableHTTPHandler` via standard library `net/http` on `/mcp`)
- Smoke tests:
  - Stdio: `./scripts/smoke.sh`
  - Streamable HTTP: `./scripts/smoke-http.sh --port 18080`
- Logging: `log/slog` to file (`GO_MCP_LOG_FILE`, default
  `/tmp/go_mcp_server.log`; level `GO_MCP_LOG_LEVEL`, default `info`).
  **NEVER to stdout** — stdout is the protocol channel.

---

## 3. Architecture

```
go-mcp-server/
├── go.mod / go.sum                     — module `go-mcp-server`, 2 direct deps
├── version.txt                         — single source of truth for version
├── main.go                             — entrypoint: logging + embed version + Start()
├── scripts/
│   └── smoke.sh                        — JSON-RPC end-to-end smoke test (2 phases)
└── internal/
    ├── model/
    │   ├── entry.go                    — ProjectEntry + ProjectKind (pure types)
    │   └── load_error.go               — LoadError with stable error codes
    ├── project/
    │   ├── manager.go                  — Manager: RWMutex, 10 min expiry, Load + analysis cache
    │   ├── loader.go                   — path / Git CLI / archive (.zip/.tar.gz) resolver
    │   └── testdata/tinymod/           — minimal fixture (load tests)
    ├── analysis/
    │   ├── project.go                  — LoadProject: parse go.mod+go.work+.go (skip vendor/testdata/._)
    │   ├── queries.go                  — AST query facade (≡ GoAnalyzerUtils)
    │   └── text.go                     — whole-word matching + substring search
    ├── server/
    │   └── server.go                   — GoMcpServer + buildTools() + adaptTool
    └── tools/                          — 43 tools (one file per tool)
        ├── tool.go                     — Tool interface (counterpart of McpTool)
        ├── base.go                     — BaseGoTool + nameOnlySchema + findAndAnalyze
        ├── render.go                   — Markdown helpers (table, loc, codeBlock, truncate)
        ├── <tool>.go                   — each tool
        ├── snapshots_test.go           — golden snapshot harness (-update)
        └── testdata/
            ├── samplemodule/           — main fixture (package main + sub + test)
            └── snapshots/              — 56 *.snap (byte-by-byte)
```

**Architectural rules**:

- `Tool` (tools/tool.go) is the **stable interface**. Tools implement it,
  they do not instantiate `mcp` directly. A change to `Tool` touches all tools.
- `BaseGoTool` is the **utility struct**. Each tool contains it and uses it
  for `Arg()`, `OKMarkdown()`, `FindEntry()`, `FindAndAnalyze()`.
- The **analysis facade** is the set of methods on `*analysis.Project`
  (queries.go) — equivalent to `RustAnalyzerUtils`/`ProLeapAnalyzerUtils`.
  Tools do NOT traverse the AST directly.
- `Manager` is thread-safe (`sync.RWMutex` + map) and applies per-entry
  expiration (10 min, same as rust/java/cobol). **Lazy** sweep on each
  operation (no goroutine). Caches `*analysis.Project` per entry.
- **The go-sdk dispatches requests concurrently** (one goroutine per request):
  tools MUST be thread-safe (they are via Manager) and MUST NOT assume
  ordering between requests. See §14 iter 2.
- `GoMcpServer.buildTools()` is the **single registration point** for tools.
  Adding a new one = one line there. (Counterpart of `build_tools()` in Rust and
  `List.of(...)` in Java.)
- `adaptTool` (server.go) is the sole bridge Tool → `mcp.ToolHandler`:
  it deserializes `req.Params.Arguments` (json.RawMessage) to `map[string]any`.
- `model/` is pure types, no logic, no I/O.

---

## 4. MCP servers available to the agent

This project runs alongside four other MCP servers.

### 4.1 — The siblings (cross-validation of the pattern)

- `cobol-mcp-server` — the "grandfather" of the pattern: 66 tools, facade
  `ProLeapAnalyzerUtils`, 215 golden snapshots.
- `java-mcp-server` — 28 tools, Spoon.
- `python-mcp-server` — 43 tools, functional pattern (one module per tool).
- `rust-mcp-server` — **the closest reference**: 37 tools, same style of
  AGENTS.md, same Manager/snapshot design. When in doubt about how something
  should look here, compare against rust-mcp-server.

### 4.2 — Self-analysis of this repo's Go code

Since iter 2, `go-mcp-server` **self-analyzes**: once registered in
opencode (see README §Usage), load it at the start of each session:

```
load_go_project(source="<path to repo>", alias="go-mcp-server")
```

Use cases: `list_functions` / `inspect_function` — understand a tool;
`find_references` — locate extension points; `validate_code_reference`
— before assuming a symbol exists after editing; `search_source` —
search for patterns across all code.

### 4.3 — Available skills

- `diagnose-stale-mcp` (global) — non-responsive MCPs, outdated binaries.
- The other MCP-specific skills (`cobol-bug-hunt`, etc.) **do not
  directly apply** here — the domain is different.

### 4.4 — The Go source of `go-mcp-server` itself

It is the **source of truth** for what information the server extracts and how
it is formatted. If you doubt what a tool returns, read its `Call()` and the
helpers in `base.go` it invokes. **Do not infer from the tool name.**

---

## 5. Deterministic procedures (recipes)

### 5.1 — Adding a new MCP tool

1. Read `internal/tools/tool.go` (contract) and an existing tool (style).
2. Create `internal/tools/<tool_name>.go` with:
   - struct `XxxTool { base BaseGoTool }`
   - `NewXxxTool(manager *project.Manager) *XxxTool`
   - `Name()`, `Description()`, `InputSchema()`, `Call(ctx, args)`
3. Add `tools.NewXxxTool(s.Manager)` in `GoMcpServer.buildTools()`
   (server.go).
4. `go vet ./... && go build ./...` — must pass.
5. Add golden snapshot in `internal/tools/snapshots_test.go` following the
   pattern of `TestListLoadedProjectsWithOneProject`.
6. `go test ./internal/tools/ -update` to create the snapshot, then
   `go test ./...` green.
7. `./scripts/smoke.sh` — still OK (or extend it if the tool requires).

### 5.2 — Modifying an existing tool

1. Read its `Call()`.
2. **Minimal and local** change — do not refactor along the way.
3. `go test ./...` must stay green. If the output change is intentional,
   regenerate with `go test ./internal/tools/ -update` and review the `.snap`
   diff as if it were an API change.
4. `./scripts/smoke.sh`.

### 5.3 — Investigating a tool bug

1. Reproduce with a minimal golden test or via smoke.sh with the exact request.
2. Read the `Call()` and trace the helpers in `base.go`.
3. Minimal fix.
4. `go test ./...` green + smoke OK.

---

## 6. Go/go-sdk knowledge the agent must apply

### 6.1 — go-sdk v1.6.1 API (verified, do not infer)

```
mcp.NewServer(&mcp.Implementation{Name, Version}, &mcp.ServerOptions{
    Instructions: "...",          // text the LLM sees on connect
})
srv.AddTool(&mcp.Tool{
    Name, Description,
    InputSchema: map[string]any{...},   // field `any`; accepts map or json.RawMessage
}, mcp.ToolHandler)                      // func(ctx, *CallToolRequest) (*CallToolResult, error)

req.Params.Arguments                     // json.RawMessage — deserialize once in adaptTool
&mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: ...}}, IsError: bool}
srv.Run(ctx, &mcp.StdioTransport{})      // blocks until EOF/cancel
```

### 6.2 — Particularities already discovered (iter 1)

- **`Run` returns error on EOF** (`server is closing: EOF`). EOF = the host
  closed stdio (normal shutdown). Treat as clean exit:
  `errors.Is(err, io.EOF)` → NOT a crash (see main.go).
- **EOF discards pending responses**: if stdin closes right after requests,
  the session dies before writing. In smoke tests, keep the pipe open
  (`{ printf ...; sleep 2; } | ./go-mcp-server`).
- **`notifications/initialized` as REQUEST (opencode quirk)**: the SDK
  responds `-32600 invalid request: unexpected id` and **keeps working**
  (tools/list and tools/call later respond fine). In rmcp we had to
  override `on_custom_request`; here there is no equivalent hook and it
  seems unnecessary. **Validate when registering in opencode** (see §13,
  checkpoint pre-iter 2): if opencode does not inject the tools, wrap the
  transport to rewrite that message as notification (without id).
- **`ServerOptions.InitializedHandler`** exists for the well-formed
  notification; we do not use it (no need to react).

### 6.3 — Canonical code patterns

Tool construction (always this shape):

```go
type XxxTool struct{ base BaseGoTool }

func NewXxxTool(manager *project.Manager) *XxxTool {
    return &XxxTool{base: NewBaseGoTool(manager)}
}

func (t *XxxTool) Name() string        { return "xxx_tool" }
func (t *XxxTool) Description() string { return "..." }
func (t *XxxTool) InputSchema() map[string]any {
    return map[string]any{"type": "object", "properties": map[string]any{}}
}
func (t *XxxTool) Call(ctx context.Context, args map[string]any) (*mcp.CallToolResult, error) {
    // user input errors → OKError(...) (IsError, visible to LLM)
    // OK → OKMarkdown(md)
}
```

Golden snapshot (always this shape):

```go
func TestXxx(t *testing.T) {
    mgr := project.NewManager()
    mgr.Add(fixedEntry(...))
    text := callTool(t, NewXxxTool(mgr), map[string]any{...})
    assertSnapshot(t, "xxx_case", text)
}
```

### 6.4 — Embedded version

`version.txt` is read with `//go:embed` in `main.go`. On bump: edit
`version.txt` only. The server's `Implementation.Version` comes from there.

---

## 7. Code conventions

- **Go**: idiomatic (`gofmt`), no external frameworks besides go-sdk.
  `log/slog` for logs.
- Imports: stdlib → go-sdk → `go-mcp-server/internal/...`, in separate
  groups.
- Tools: one struct per tool, one file per tool, central registration in
  `buildTools()`.
- Naming: snake_case for files, PascalCase for structs, camelCase for
  unexported functions. Tool names: snake_case without prefixes.
- Logging: `slog.Info/Warn/Error` with key-value pairs, one line per
  event, no PII or absolute user paths.
- Output: stable Markdown (via `OKMarkdown`), **no timestamps or volatile
  content**. Paths in test snapshots are synthetic (`/fixtures/...`).
- Error messages: readable for the final LLM (in English).
- Tests: `go test`, golden files in `testdata/snapshots/`, Go fixtures in
  `testdata/` (iter 2+), do not use external projects in tests.

---

## 8. Anti-patterns (DO NOT do)

- ❌ Do not log to **stdout** — it is the MCP protocol channel. slog goes to
  file (or stderr as fallback).
- ❌ Do not treat `io.EOF` from `Run()` as a crash — it is normal shutdown.
- ❌ Do not include timestamps (`LoadedAt`, `ExpiresAt`) in tool Markdown —
  it breaks byte-by-byte determinism of snapshots. If needed, put them in a
  separate tool (`project_metadata` with `include_expiry: bool`, iter 2+).
- ❌ Do not change the signature of `Tool` or `BaseGoTool` without updating
  all tools — it is the stable API of the ground truth.
- ❌ Do not add a tool that does not implement `Tool` (breaks homogeneity
  and the `buildTools()` aggregator).
- ❌ Do not hardcode the version — read from `version.txt` via `//go:embed`.
- ❌ Do not use `flag.Bool("update", ...)` in more than one test package —
  `go test ./... -update` would fail in other packages. The flag lives only
  in `internal/tools` and is invoked `go test ./internal/tools/ -update`.
- ❌ Do not register tools with the generic `mcp.AddTool[In,Out]` from the
  SDK — we use the low-level `srv.AddTool` + `adaptTool` to keep the `Tool`
  interface of the family (args as map, freeform Markdown).
- ❌ Do not use a tool's result as truth without verifying against the Go
  code that generates it.
- ❌ Do not break determinism: two calls on the same project must produce
  **exactly** the same Markdown (byte-by-byte).

---

## 9. Self-check before answering

If the user's question touches the Go code of `go-mcp-server` or the output
of a tool:

1. Have I read the tool's `Call()` and the helpers in `base.go`? If not → read them.
2. If the proposal touches a tool, have I added/updated its golden snapshot
   and run `go test ./...`? If not → do it.
3. Have I run `./scripts/smoke.sh` after changes to server/main? If not → do it.
4. Does my answer explicitly distinguish between **"what the code does today"**
   (verified) and **"what it should do"** (proposal)?
5. If I propose a change, does it include a procedure to verify it?
6. **Are you calling tools that exist in this MCP?** Don't confuse with tools from
   other MCP servers (`java-mcp-server`, `cobol-mcp-server`, `rust-mcp-server`,
   `python-mcp-server`, `javascript-mcp-server`). Each MCP has its own tools.
   If you need to analyze code in another language, use the corresponding MCP.

---

## 10. Example session transcript (pattern to imitate)

**Task**: "the `list_loaded_projects` tool should show the total number
of workspace modules in the header".

```
[1] read internal/tools/list_loaded_projects.go          # see Call()
[2] read internal/tools/snapshots_test.go                # see existing tests
[3] (proposal) add " (N modules)" to H1 when workspace
[4] edit + go vet ./... && go build ./...                 # compiles
[5] go test ./internal/tools/                            # FAILS (old snapshot) ✔ expected
[6] go test ./internal/tools/ -update                    # regenerate
[7] git diff testdata/snapshots/                          # review byte-by-byte diff
[8] go test ./...                                         # 7/7 green
[9] ./scripts/smoke.sh                                    # end-to-end OK
[10] commit with message explaining the Markdown schema change
```

---

## 11. Resources

- go-sdk MCP: https://github.com/modelcontextprotocol/go-sdk (pkg `mcp`)
- go/ast + go/parser (iter 2): https://pkg.go.dev/go/ast
- go/packages + go/types (iter 3): https://pkg.go.dev/golang.org/x/tools/go/packages
- x/mod/modfile (go.mod parsing, iter 2): https://pkg.go.dev/golang.org/x/mod/modfile
- MCP spec: https://modelcontextprotocol.io/specification/2025-11-25
- README: `./README.md` (user-facing)
- Sibling projects:
  - `../cobol-mcp-server/AGENTS.md` (same pattern, ProLeap, 66 tools)
  - `../java-mcp-server/AGENTS.md` (same pattern, Spoon, 28 tools)
  - `../python-mcp-server/AGENTS.md` (same pattern, LibCST, 43 tools)
  - `../rust-mcp-server/AGENTS.md` (same pattern, syn+ra_ap_*, 37 tools — reference)

---

## 12. Golden snapshot tests (determinism enforced)

`internal/tools/snapshots_test.go` runs each tool against synthetic Manager
entries or the `testdata/samplemodule` fixture, and compares the Markdown
**byte-by-byte** against snapshots in
`internal/tools/testdata/snapshots/`.

**Current coverage**: 56 snapshots — all tools, variants (visibility
filter, case-insensitive), and edge cases (symbol not found,
path traversal blocked, project not loaded, source not found).

**Machine-independence**: snapshots NEVER contain absolute host paths.
Analysis tools emit paths relative to the project root;
`load_go_project` (which does show the absolute path) is tested with
content checks, not snapshots (lesson from rust iter 7+).

**Harness behavior**:
- `go test ./...` — verifies against snapshots (fails with diff if different).
- `go test ./internal/tools/ -update` — (re)generates. The `-update` flag
  lives ONLY in that package (a `go test ./... -update` would break others).

**Rules**:
- Any change that **intentionally modifies** output must be accompanied by
  snapshot regeneration and review of the diff.
- Snapshots are part of the contract — a change affecting them requires the
  same level of review as an API change.

---

## 13. Iteration plan

| Iter | What is added | Status |
|------|--------------|--------|
| **1** | Skeleton: go-sdk + `Tool` pattern + `Manager` (10 min expiry) + 1 tool (`list_loaded_projects`) + golden snapshots + AGENTS.md/README + end-to-end smoke test | ✅ Done |
| **2** | `Loader` (local path / Git clone CLI / archive .zip/.tar.gz) + `go/ast`+`go/parser`+`x/mod/modfile` analysis. **22 new tools (23 total)**: lifecycle (`load_go_project`, `unload_go_project`, `project_metadata`, `inspect_build_config`), module (`list_packages`, `list_dependencies`, `program_summary`), files (`get_file_content` with `os.OpenRoot`, `search_source`), AST listings (`list_functions`, `list_structs`, `list_interfaces`, `list_constants`, `list_variables`, `list_imports`), inspections (`inspect_function`, `inspect_struct`, `inspect_interface`), textual xref (`validate_code_reference`, `find_references`, `function_callers`), domain (`find_entry_points`). Facade = methods on `*analysis.Project`. Fixtures in `testdata/`. **41 tests (31 snapshots)**. | ✅ Done |
| **checkpoint** | Register in `~/.config/opencode/opencode.json` and verify opencode injects the tools (risk: `notifications/initialized` quirk, see §6.2) | ⏳ Pending |
| **3** | Semantic layer `go/packages`+`go/types` (lazy — expensive cold): `resolve_type`, `find_implementations` (interfaces — implicit in Go, high value), `function_call_graph`, `struct_field_xref`, `get_type_hierarchy` (embedding). **5 new tools, 11 tests (6 snapshots).** | ✅ Done |
| **4** | `BugPatternAnalyzer` — sweeps like `cobol-bug-hunt`: `find_unchecked_errors` (errcheck-style), `find_panic_sites`, `find_unsafe_patterns`, `find_goroutine_leaks` (heuristic), `find_defer_in_loops`, `find_dead_code` (semantic with go/types). **6 new tools.** | ✅ Done |
| **5** | CI workflow + Dockerfile multi-stage + domain (`detect_framework` gin/echo/grpc/cobra, `list_api_endpoints`, `extract_environment_dependencies`). **3 new tools + infra.** | ✅ Done |
| **6** | Dual transport parity (MCP_CONTRACT): `stdio` (default) + Streamable HTTP (`/mcp`, `--mode http --port/--host/--max-threads/--api-keys`). Bearer token auth (`ApiKeyStore`), `net/http` + `mcp.NewStreamableHTTPHandler`, `internal/transport/http_transport_test.go` (lifecycle + auth), `scripts/smoke-http.sh` (E2E check). **72 tests green (56 snapshots + 16 unit/integration), 0 warnings.** | ✅ Done |

**Iteration rule**: each iteration ends with `go test ./...` green + smoke
test end-to-end OK + snapshot diff reviewed if applicable. No skipping to
the next iteration without validating the previous one.

---

## 14. Lessons learned

### iter 2 (loader + go/ast + 22 tools)

- **The go-sdk dispatches each request in its own goroutine**: responses
  arrive OUT OF ORDER (in smoke: id 4 responded before id 3, and id 6 before
  id 5 → `list_functions` failed with "project not found" because it ran
  before `load_go_project` finished). Implications: (1) tools MUST be
  thread-safe — they are via `Manager` with `sync.RWMutex`; (2) real LLM
  hosts wait for each response before the next call, so it is not a problem
  in production; (3) the smoke test needs PHASES (pause between dependent
  requests).
- **`go/printer` with artificial positions breaks output**: trying to force
  parentheses in an `*ast.FieldList` by injecting `Opening/Closing = 1`
  produced empty signatures (` `). Solution: render params/results manually
  (`funcTypeString`/`fieldListString`, go/doc style: results without
  parentheses only if a single unnamed result). Analogous to the rust lesson
  on `proc_macro2::Span`: position hacks do not play well with printers.
- **`exprString` must accept `ast.Node`, not `ast.Expr`**: `*ast.FieldList`
  implements Node but not Expr.
- **Go tool conventions when walking the tree**: exclude `vendor/`,
  `testdata/`, and dirs starting with `.` or `_`. Without this, when
  self-analyzing this repo, `testdata/samplemodule` would appear as part
  of the project.
- **`os.OpenRoot` + `filepath.IsLocal`** (Go 1.24+) is the correct defense
  against path traversal: better than normalizing `..` by hand (lesson from
  rust iter 6+, solved at the root by modern stdlib).
- **Archives with a single root directory**: GitHub/GitLab convention;
  the loader descends one level (`singleChildDir`) so `Path` points to
  the project and not the wrapper.
- **Machine-independent snapshots**: tools emit paths RELATIVE to the
  project root. `load_go_project` shows the absolute path → tested with
  `strings.Contains`, not snapshot.
- **`Manager.Find` by alias is redundant with the key** (EffectiveName):
  the useful fallback is by real `Name` (already fixed in iter 1).
- **`modfile.Parse` does NOT resolve modules**: the fixture declares
  non-existent deps (`github.com/foo/bar`) and analysis works — parsing is
  textual. `go/packages` (iter 3) WILL need downloadable deps.

### iter 1 (skeleton)

- **`Server.Run` returns `server is closing: EOF` when stdin closes**: it is
  normal shutdown (opencode kills the process), NOT a crash. Without
  `errors.Is(err, io.EOF)` in main.go, the server exited with code 1 always.
- **EOF on stdin kills the session before writing pending responses**:
  a bare `printf | ./server` produces ZERO output. In smoke tests, keep the
  pipe open: `{ printf ...; sleep 2; } | ./server`.
- **opencode quirk (`notifications/initialized` with id)**: the go-sdk
  responds `-32600 invalid request: unexpected id` (correct per spec) and
  the session continues fine — unlike rmcp, which needed an override of
  `on_custom_request`. Pending real validation in opencode (checkpoint
  pre-iter 2); if it breaks tool injection, the solution is a `Transport`
  wrapper that rewrites that message.
- **`Manager.Find` by real name**: in rust-mcp-server the search fallback
  only looks at alias (redundant, because the map key IS the EffectiveName).
  A unit test (`TestAddThenFind`) caught that searching by the original name
  of an entry WITH alias returned nil. Here the fallback checks `e.Name` —
  strictly better behavior and consistent with the "name or alias" description.
- **`-update` flag per package**: `flag.Bool` in `internal/tools` only exists
  in that test binary. `go test ./... -update` fails in other packages with
  "flag provided but not defined". Regenerate with `go test ./internal/tools/ -update`.
- **Binary**: 8.0 MB (macOS arm64, with symbols). With `-ldflags="-s -w"`
  it shrinks further; cross-compile to Linux is `GOOS=linux GOARCH=amd64 go build`
  (static by default, no zig/musl).
- **1 single direct dep** (go-sdk); transitives include `golang.org/x/tools`
  — reused in iter 3 for go/packages.
