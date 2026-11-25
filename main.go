// Entry point of go-mcp-server.
//
// Supports dual transport (mirrors rust-mcp-server, cobol-mcp-server, java-mcp-server):
//   - stdio (default): subprocess mode for local clients (OpenCode, Claude Desktop).
//   - http (--mode http): Streamable HTTP mode on /mcp with optional Bearer auth (--api-keys).
package main

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"go-mcp-server/internal/project"
	"go-mcp-server/internal/server"
	"go-mcp-server/internal/transport"
)

// Version is read from version.txt (source of truth, DO NOT hardcode).
//
//go:embed version.txt
var versionRaw string

type cliConfig struct {
	mode       string
	port       int
	host       string
	maxThreads int
	apiKeys    string
}

func parseCli() cliConfig {
	cfg := cliConfig{
		mode:       "stdio",
		port:       8080,
		host:       "0.0.0.0",
		maxThreads: 16,
		apiKeys:    "",
	}

	// Environment variable defaults
	if envMode := os.Getenv("GO_MCP_MODE"); envMode != "" {
		cfg.mode = strings.ToLower(envMode)
	} else if envTrans := os.Getenv("GO_MCP_TRANSPORT"); envTrans != "" {
		cfg.mode = strings.ToLower(envTrans)
	}
	if envPort := os.Getenv("GO_MCP_PORT"); envPort != "" {
		if p, err := strconv.Atoi(envPort); err == nil && p > 0 {
			cfg.port = p
		}
	}
	if envHost := os.Getenv("GO_MCP_HOST"); envHost != "" {
		cfg.host = envHost
	}
	if envKeys := os.Getenv("GO_MCP_API_KEYS"); envKeys != "" {
		cfg.apiKeys = envKeys
	}

	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--mode" && i+1 < len(args):
			i++
			cfg.mode = strings.ToLower(args[i])
		case strings.HasPrefix(arg, "--mode="):
			cfg.mode = strings.ToLower(strings.TrimPrefix(arg, "--mode="))

		case arg == "--port" && i+1 < len(args):
			i++
			if p, err := strconv.Atoi(args[i]); err == nil && p > 0 {
				cfg.port = p
			}
		case strings.HasPrefix(arg, "--port="):
			if p, err := strconv.Atoi(strings.TrimPrefix(arg, "--port=")); err == nil && p > 0 {
				cfg.port = p
			}

		case arg == "--host" && i+1 < len(args):
			i++
			cfg.host = args[i]
		case strings.HasPrefix(arg, "--host="):
			cfg.host = strings.TrimPrefix(arg, "--host=")

		case arg == "--max-threads" && i+1 < len(args):
			i++
			if t, err := strconv.Atoi(args[i]); err == nil && t > 0 {
				cfg.maxThreads = t
			}
		case strings.HasPrefix(arg, "--max-threads="):
			if t, err := strconv.Atoi(strings.TrimPrefix(arg, "--max-threads=")); err == nil && t > 0 {
				cfg.maxThreads = t
			}

		case arg == "--api-keys" && i+1 < len(args):
			i++
			cfg.apiKeys = args[i]
		case strings.HasPrefix(arg, "--api-keys="):
			cfg.apiKeys = strings.TrimPrefix(arg, "--api-keys=")
		}
	}

	return cfg
}

func main() {
	initLogging()

	version := strings.TrimSpace(versionRaw)
	manager := project.NewManager()
	srv := server.New(manager, version)
	cli := parseCli()

	if cli.mode == "http" {
		var keyStore *transport.ApiKeyStore
		if cli.apiKeys != "" {
			ks, err := transport.LoadApiKeyStore(cli.apiKeys)
			if err != nil {
				slog.Error("failed to load API keys", "error", err)
				fmt.Fprintf(os.Stderr, "go-mcp-server: failed to load API keys: %v\n", err)
				os.Exit(1)
			}
			keyStore = ks
		}

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		launcher := &transport.HttpTransportLauncher{}
		if err := launcher.Start(ctx, srv, cli.host, cli.port, keyStore); err != nil {
			slog.Error("go-mcp-server crashed during HTTP transport", "error", err)
			fmt.Fprintf(os.Stderr, "go-mcp-server: %v\n", err)
			os.Exit(1)
		}
		slog.Info("go-mcp-server (HTTP) stopped")
		return
	}

	slog.Info("go-mcp-server starting", "version", version, "transport", "stdio")
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
