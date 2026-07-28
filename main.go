// Entry point of go-mcp-server.
//
//  1. Initializes logging (file, configurable via env; NEVER stdout —
//     stdout is the MCP protocol channel).
//  2. Builds the ProjectManager.
//  3. Builds the GoMcpServer.
//  4. Calls Start() — blocks until stdio closes.
package main

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"go-mcp-server/internal/project"
	"go-mcp-server/internal/server"
)

// Version is read from version.txt (source of truth, DO NOT hardcode).
//
//go:embed version.txt
var versionRaw string

func main() {
	initLogging()

	version := strings.TrimSpace(versionRaw)
	manager := project.NewManager()
	srv := server.New(manager, version)

	slog.Info("go-mcp-server starting", "version", version)
	if err := srv.Start(context.Background()); err != nil && !errors.Is(err, io.EOF) {
		slog.Error("go-mcp-server crashed", "error", err)
		fmt.Fprintf(os.Stderr, "go-mcp-server: %v\n", err)
		os.Exit(1)
	}
	// io.EOF = the host closed stdio (normal shutdown, e.g. opencode kills
	// the process). NOT a crash.
	slog.Info("go-mcp-server stopped")
}

// initLogging sends logs to GO_MCP_LOG_FILE (default
// /tmp/go_mcp_server.log). If it cannot be opened, falls back to stderr.
// Level: GO_MCP_LOG_LEVEL (debug/info/warn/error, default info).
func initLogging() {
	logFile := os.Getenv("GO_MCP_LOG_FILE")
	if logFile == "" {
		logFile = "/tmp/go_mcp_server.log"
	}
	level := os.Getenv("GO_MCP_LOG_LEVEL")
	if level == "" {
		level = "info"
	}

	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		lvl = slog.LevelInfo
	}
	opts := &slog.HandlerOptions{Level: lvl}

	w := os.Stderr
	if f, err := os.OpenFile(logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
		w = f
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(w, opts)))
}
