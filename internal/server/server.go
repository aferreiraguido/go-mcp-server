// Package server implements the GoMcpServer: tool registration and
// MCP server startup over stdio.
//
// Counterpart of `server.rs` (rust-mcp-server) and
// `com.softtek.mcp.CobolMcpServer`/`JavaMcpServer`.
package server

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"go-mcp-server/internal/project"
	"go-mcp-server/internal/tools"
)

// Instructions is the text the LLM sees when connecting.
// Counterpart of `with_instructions(...)` in rust-mcp-server.
const Instructions = "Deterministic Go code analysis via go/ast + go/types. " +
	"Ground truth for AI agents — no hallucination."

// GoMcpServer wraps the shared manager and builds the MCP server.
type GoMcpServer struct {
	Manager *project.Manager
	Version string
}

// New constructs a GoMcpServer.
func New(manager *project.Manager, version string) *GoMcpServer {
	return &GoMcpServer{Manager: manager, Version: version}
}

// buildTools is the SINGLE registration point for tools.
// Adding a new one = one line here. (Counterpart of `build_tools()` in rust
// and `List.of(new XTool(manager).build(), ...)` in Java.)
func (s *GoMcpServer) buildTools() []tools.Tool {
	return []tools.Tool{
		tools.NewListLoadedProjectsTool(s.Manager),
		// Source management (iter 2)
		tools.NewLoadGoProjectTool(s.Manager),
		tools.NewUnloadGoProjectTool(s.Manager),
		tools.NewProjectMetadataTool(s.Manager),
		tools.NewInspectBuildConfigTool(s.Manager),
		// Module level (iter 2)
		tools.NewListPackagesTool(s.Manager),
		tools.NewListDependenciesTool(s.Manager),
		tools.NewProgramSummaryTool(s.Manager),
		// File ops (iter 2)
		tools.NewGetFileContentTool(s.Manager),
		tools.NewSearchSourceTool(s.Manager),
		// Source listings — go/ast (iter 2)
		tools.NewListFunctionsTool(s.Manager),
		tools.NewListStructsTool(s.Manager),
		tools.NewListInterfacesTool(s.Manager),
		tools.NewListConstantsTool(s.Manager),
		tools.NewListVariablesTool(s.Manager),
		tools.NewListImportsTool(s.Manager),
		// Inspections (iter 2)
		tools.NewInspectFunctionTool(s.Manager),
		tools.NewInspectStructTool(s.Manager),
		tools.NewInspectInterfaceTool(s.Manager),
		// Cross-references — textuales (iter 2)
		tools.NewValidateCodeReferenceTool(s.Manager),
		tools.NewFindReferencesTool(s.Manager),
		tools.NewFunctionCallersTool(s.Manager),
		// Dominio (iter 2)
		tools.NewFindEntryPointsTool(s.Manager),
		// Semantic layer — go/packages + go/types (iter 3)
		tools.NewResolveTypeTool(s.Manager),
		tools.NewFindImplementationsTool(s.Manager),
		tools.NewFunctionCallGraphTool(s.Manager),
		tools.NewStructFieldXRefTool(s.Manager),
		tools.NewGetTypeHierarchyTool(s.Manager),
		// Bug pattern analysis (iter 4)
		tools.NewFindPanicSitesTool(s.Manager),
		tools.NewFindUnsafePatternsTool(s.Manager),
		tools.NewFindDeferInLoopsTool(s.Manager),
		tools.NewFindUncheckedErrorsTool(s.Manager),
		tools.NewFindGoroutineLeaksTool(s.Manager),
		tools.NewFindDeadCodeTool(s.Manager),
		// Domain analysis (iter 5)
		tools.NewDetectFrameworkTool(s.Manager),
		tools.NewListAPIEndpointsTool(s.Manager),
		tools.NewExtractEnvironmentDependenciesTool(s.Manager),
		// Project exploration (iter 5)
		tools.NewListDirectoriesTool(s.Manager),
		tools.NewProjectFilesTool(s.Manager),
		// Code quality metrics (iter 5)
		tools.NewFindLongFunctionsTool(s.Manager),
		tools.NewFindHighCyclomaticComplexityTool(s.Manager),
		tools.NewFindUnusedParametersTool(s.Manager),
		tools.NewFindUnimplementedInterfacesTool(s.Manager),
	}
}

// MCPServer builds the official SDK *mcp.Server with all tools
// registered via the Tool → ToolHandler adapter.
func (s *GoMcpServer) MCPServer() *mcp.Server {
	srv := mcp.NewServer(
		&mcp.Implementation{Name: "go-mcp-server", Version: s.Version},
		&mcp.ServerOptions{Instructions: Instructions},
	)
	for _, t := range s.buildTools() {
		srv.AddTool(&mcp.Tool{
			Name:        t.Name(),
			Description: t.Description(),
			InputSchema: t.InputSchema(),
		}, adaptTool(t))
	}
	return srv
}

// Start mounts the stdio transport and blocks until the host shuts
// down the process or stdio drops.
func (s *GoMcpServer) Start(ctx context.Context) error {
	return s.MCPServer().Run(ctx, &mcp.StdioTransport{})
}

// adaptTool converts our tools.Tool into an SDK mcp.ToolHandler.
// Deserializes the JSON arguments once and passes them as a map.
func adaptTool(t tools.Tool) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := map[string]any{}
		if len(req.Params.Arguments) > 0 {
			if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
				return nil, fmt.Errorf("invalid arguments for tool '%s': %w", t.Name(), err)
			}
		}
		return t.Call(ctx, args)
	}
}
