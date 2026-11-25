# go-mcp-server

MCP (Model Context Protocol) server for deterministic analysis of Go
projects. Returns **deterministic Markdown** ready for LLM consumption:
"ground truth, no hallucination".

Go sibling of [`cobol-mcp-server`](../cobol-mcp-server),
[`java-mcp-server`](../java-mcp-server),
[`python-mcp-server`](../python-mcp-server) and
[`rust-mcp-server`](../rust-mcp-server).

> **Status**: iteration 6 — 43 tools, static analysis with
> `go/ast` + semantic layer with `go/types`. See [AGENTS.md](AGENTS.md) §13
> for the full iteration plan.

## Stack

- Go 1.26+
- Official MCP SDK: [`modelcontextprotocol/go-sdk`](https://github.com/modelcontextprotocol/go-sdk) v1.6.1
- Analysis: `go/ast` + `go/parser` (stdlib) and
  [`golang.org/x/mod/modfile`](https://pkg.go.dev/golang.org/x/mod/modfile) (go.mod/go.work)
- Semantic: `golang.org/x/tools/go/packages` + `go/types`
- Transport:
  - Default: `stdio` (JSON newline-delimited)
  - HTTP mode: Streamable HTTP (`mcp.StreamableHTTPHandler` on `/mcp`) with optional Bearer auth

## Build and test

```sh
go build -o go-mcp-server .          # binary ~8 MB, static
go vet ./...                         # compile check
go test ./...                        # unit + integration + golden snapshot tests
go test ./internal/tools/ -update    # regenerate golden snapshots
./scripts/smoke.sh                   # end-to-end smoke test (stdio)
./scripts/smoke-http.sh              # end-to-end smoke test (Streamable HTTP on :18080)
```

## Transport modes

`go-mcp-server` supports dual transport:

### Stdio mode (default)

Subprocess mode for local clients like OpenCode or Claude Desktop.

```sh
./go-mcp-server                      # defaults to --mode stdio
```

### HTTP Streamable mode

Listens on HTTP using the official Streamable HTTP transport at `/mcp`.

```sh
./go-mcp-server --mode http --port 8080 --host 0.0.0.0
```

CLI flags and environment variables:

| Flag | Env | Default | Description |
|------|-----|---------|-------------|
| `--mode` | `GO_MCP_MODE` | `stdio` | Transport mode: `stdio` or `http` |
| `--port` | `GO_MCP_PORT` | `8080` | HTTP port to bind |
| `--host` | `GO_MCP_HOST` | `0.0.0.0` | HTTP host to bind |
| `--max-threads` | - | `16` | Accepted for CLI contract compatibility |
| `--api-keys` | `GO_MCP_API_KEYS` | - | Path to keys file (JSON array or lines) or comma-separated tokens |

When `--api-keys` is set, any request to `/mcp` without a valid `Authorization: Bearer <token>` header returns `401 Unauthorized`.

## Usage with opencode

Add to `~/.config/opencode/opencode.json`:

```json
"go-mcp-server": {
  "type": "local",
  "command": [
    "/absolute/path/go-mcp-server/go-mcp-server"
  ],
  "enabled": true
}
```

Restart opencode for the server to load.

## Tools (43)

**Lifecycle**

| Tool | Description |
|------|-------------|
| `load_go_project` | Load a project (local path, Git URL, .zip/.tar.gz archive). 10 min cache. |
| `unload_go_project` | Unload a project from memory (cleans temp dirs). |
| `list_loaded_projects` | List loaded projects (sorted by effective name). |
| `project_metadata` | Metadata: name, alias, kind, module path, Go version, members. |

**Module and build**

| Tool | Description |
|------|-------------|
| `inspect_build_config` | Raw content of go.mod (and go.work if present). |
| `list_packages` | Project packages (dirs with .go, excluding vendor/testdata/hidden). |
| `list_dependencies` | go.mod requires with indirect flag + replace. |
| `program_summary` | Executive summary: packages, files, lines, functions, structs, deps... |

**Files**

| Tool | Description |
|------|-------------|
| `get_file_content` | Raw content of a file (path traversal blocked via `os.OpenRoot`). |
| `search_source` | Text search in all .go files (case-sensitive by default). |

**AST listings**

| Tool | Description |
|------|-------------|
| `list_functions` | Functions and methods: signature, receiver, exported, location. `visibility` filter. |
| `list_structs` | Structs: name, field count, exported, location. |
| `list_interfaces` | Interfaces: name, method count (incl. embeds), exported, location. |
| `list_constants` | Package-level constants (incl. iota groups): type, value, location. |
| `list_variables` | Package-level variables (mutable state — good audit starting point). |
| `list_imports` | Imports per file: path, alias, location. |

**Inspection**

| Tool | Description |
|------|-------------|
| `inspect_function` | Signature, receiver, doc and full body of a function/method. |
| `inspect_struct` | Fields (type, tag, embedded, exported) and receiver methods. |
| `inspect_interface` | Required methods and embedded interfaces. |

**Cross-references (textual)**

| Tool | Description |
|------|-------------|
| `validate_code_reference` | EXISTS / NOT_FOUND for top-level symbols. |
| `find_references` | Whole-word references with file:line:context. |
| `function_callers` | Lines with `symbol(` (call style). |

**Domain**

| Tool | Description |
|------|-------------|
| `find_entry_points` | `func main()` in main packages + all `func init()`. |
| `detect_framework` | Detect web frameworks (gin, echo, cobra, chi, gorilla/mux, fiber, grpc, net/http). |
| `list_api_endpoints` | HTTP route definitions (r.GET, r.POST, router.Handle, etc.). |
| `extract_environment_dependencies` | Environment variable reads, flag definitions, viper config. |

**Project exploration**

| Tool | Description |
|------|-------------|
| `list_directories` | Directories containing .go files. |
| `project_files` | All .go files sorted by relative path. |

**Code quality metrics**

| Tool | Description |
|------|-------------|
| `find_long_functions` | Functions with body exceeding a configurable line count. |
| `find_high_cyclomatic_complexity` | Functions with cyclomatic complexity exceeding a threshold. |
| `find_unused_parameters` | Function parameters declared but never used. |
| `find_unimplemented_interfaces` | Interfaces with no concrete implementation (semantic). |

**Bug pattern analysis**

| Tool | Description |
|------|-------------|
| `find_panic_sites` | `panic()`, `log.Fatal()`, `log.Panic()`, `os.Exit()`. |
| `find_unsafe_patterns` | `unsafe.Pointer`, `unsafe.Sizeof`, unchecked memory operations. |
| `find_defer_in_loops` | `defer` inside for/range loops (likely resource leak). |
| `find_unchecked_errors` | Error-returning calls whose result is discarded (errcheck-style). |
| `find_goroutine_leaks` | Goroutines lacking synchronization mechanisms (heuristic). |
| `find_dead_code` | Non-exported symbols never referenced (semantic with go/types). |

**Semantic layer (go/types)**

| Tool | Description |
|------|-------------|
| `resolve_type` | Resolve a symbol name to its definition site and type. |
| `find_implementations` | Concrete types satisfying an interface. |
| `function_call_graph` | Callers and callees of a function/method. |
| `struct_field_xref` | Cross-reference a struct field name. |
| `get_type_hierarchy` | Type hierarchy: embedding and implementing types. |

## Environment configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `GO_MCP_LOG_FILE` | `/tmp/go_mcp_server.log` | Log file (never stdout). |
| `GO_MCP_LOG_LEVEL` | `info` | slog level: `debug`/`info`/`warn`/`error`. |

## Development

See [AGENTS.md](AGENTS.md): architecture, recipes for adding/modifying
tools, conventions, anti-patterns and lessons learned.
