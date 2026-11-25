package transport

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"go-mcp-server/internal/server"
)

const (
	unauthorizedResponse = `{"jsonrpc":"2.0","error":{"code":-32001,"message":"Authentication required: missing or invalid API key. Include Authorization: Bearer <api-key> header."},"id":null}`
)

// HttpTransportLauncher manages the HTTP Streamable transport for GoMcpServer.
type HttpTransportLauncher struct{}

// Start starts the HTTP Streamable server on host:port and blocks until ctx is canceled
// or the server encounters a fatal error.
func (l *HttpTransportLauncher) Start(
	ctx context.Context,
	srv *server.GoMcpServer,
	host string,
	port int,
	apiKeys *ApiKeyStore,
) error {
	addr := fmt.Sprintf("%s:%d", host, port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to bind HTTP listener on %s: %w", addr, err)
	}
	defer listener.Close()

	return l.StartWithListener(ctx, listener, srv, apiKeys)
}

// StartWithListener starts the HTTP Streamable server using an existing net.Listener.
// Useful for tests binding to port 0.
func (l *HttpTransportLauncher) StartWithListener(
	ctx context.Context,
	listener net.Listener,
	srv *server.GoMcpServer,
	apiKeys *ApiKeyStore,
) error {
	opts := &mcp.StreamableHTTPOptions{
		DisableLocalhostProtection: true,
	}

	streamableHandler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return srv.MCPServer()
	}, opts)

	// Wrap in route handler that matches /mcp and /mcp/
	mcpHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/mcp" || path == "/mcp/" || strings.HasPrefix(path, "/mcp/") {
			streamableHandler.ServeHTTP(w, r)
			return
		}
		http.NotFound(w, r)
	})

	var rootHandler http.Handler = mcpHandler
	if apiKeys != nil {
		rootHandler = apiKeyAuthMiddleware(apiKeys, rootHandler)
	}

	httpServer := &http.Server{
		Handler:      rootHandler,
		ReadTimeout:  120 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	authDesc := "disabled"
	if apiKeys != nil {
		authDesc = fmt.Sprintf("%d api-key(s)", apiKeys.Count())
	}

	slog.Info(fmt.Sprintf("MCP server listening on http://%s/mcp (Streamable HTTP)", listener.Addr().String()))
	slog.Info(fmt.Sprintf("go-mcp-server v%s (HTTP mode, auth: %s)", srv.Version, authDesc))

	serverErrCh := make(chan error, 1)
	go func() {
		if err := httpServer.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrCh <- err
		}
		close(serverErrCh)
	}()

	select {
	case <-ctx.Done():
		slog.Info("Shutting down HTTP transport...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
		return nil
	case err := <-serverErrCh:
		if err != nil {
			return fmt.Errorf("HTTP transport server error: %w", err)
		}
		return nil
	}
}

func apiKeyAuthMiddleware(apiKeys *ApiKeyStore, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/mcp" || path == "/mcp/" || strings.HasPrefix(path, "/mcp/") {
			authHeader := r.Header.Get("Authorization")
			if !strings.HasPrefix(authHeader, "Bearer ") {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(unauthorizedResponse))
				return
			}
			token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
			if !apiKeys.Validate(token) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(unauthorizedResponse))
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
