// Package tools contains the MCP tools, one per file.
//
// Counterpart of `tools/mcp_tool.rs` (rust-mcp-server) and
// `com.softtek.mcp.tools.McpTool` (java/cobol).
package tools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Tool is the stable interface that every tool implements.
// DO NOT break the shape — it is the consumer contract.
//
// Counterpart of the `McpTool` trait from rust-mcp-server. Verbose by design:
// one struct per tool, explicit registration in `GoMcpServer.buildTools()`.
type Tool interface {
	// Name is the stable name (e.g. "list_loaded_projects").
	// Snake_case, no prefixes.
	Name() string

	// Description is readable by the LLM.
	Description() string

	// InputSchema is the JSON Schema for input arguments.
	// If the tool takes no arguments, return
	// {"type": "object", "properties": {}}.
	InputSchema() map[string]any

	// Call executes the tool. `args` comes from the MCP client already deserialized.
	// User input errors → OKError (IsError=true in the content,
	// visible to the LLM). Protocol errors → Go error.
	Call(ctx context.Context, args map[string]any) (*mcp.CallToolResult, error)
}
