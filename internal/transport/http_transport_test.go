package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"go-mcp-server/internal/project"
	"go-mcp-server/internal/server"
)

// parseMCPResponse extracts JSON from either a raw JSON response or SSE "data: {...}" line.
func parseMCPResponse(text string) (map[string]any, error) {
	var v map[string]any
	if err := json.Unmarshal([]byte(text), &v); err == nil {
		return v, nil
	}
	lines := strings.Split(text, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if strings.HasPrefix(line, "data:") {
			rest := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if strings.HasPrefix(rest, "{") {
				if err := json.Unmarshal([]byte(rest), &v); err == nil {
					return v, nil
				}
			}
		}
	}
	return nil, fmt.Errorf("unable to parse MCP response from: %s", text)
}

func TestHttpTransportLifecycle(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen on port 0: %v", err)
	}

	manager := project.NewManager()
	srv := server.New(manager, "0.2.0")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	launcher := &HttpTransportLauncher{}
	go func() {
		_ = launcher.StartWithListener(ctx, listener, srv, nil)
	}()

	client := &http.Client{Timeout: 10 * time.Second}
	url := fmt.Sprintf("http://%s/mcp", listener.Addr().String())

	// 1. Initialize
	initPayload := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2025-06-18",
			"capabilities":    map[string]any{},
			"clientInfo": map[string]any{
				"name":    "test-http-client",
				"version": "1.0",
			},
		},
	}
	initBytes, _ := json.Marshal(initPayload)
	req, _ := http.NewRequest("POST", url, bytes.NewReader(initBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("initialize failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	sessionID := resp.Header.Get("Mcp-Session-Id")
	if sessionID == "" {
		// Also check case-insensitive
		for k, v := range resp.Header {
			if strings.EqualFold(k, "mcp-session-id") && len(v) > 0 {
				sessionID = v[0]
				break
			}
		}
	}
	if sessionID == "" {
		t.Fatal("expected Mcp-Session-Id header in initialize response")
	}

	initBodyBytes, _ := io.ReadAll(resp.Body)
	initParsed, err := parseMCPResponse(string(initBodyBytes))
	if err != nil {
		t.Fatalf("failed to parse init response: %v", err)
	}
	resMap, _ := initParsed["result"].(map[string]any)
	serverInfo, _ := resMap["serverInfo"].(map[string]any)
	if serverInfo["name"] != "go-mcp-server" {
		t.Fatalf("expected server name go-mcp-server, got %v", serverInfo["name"])
	}

	// 2. notifications/initialized
	notifyPayload := map[string]any{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	}
	notifyBytes, _ := json.Marshal(notifyPayload)
	reqNotify, _ := http.NewRequest("POST", url, bytes.NewReader(notifyBytes))
	reqNotify.Header.Set("Content-Type", "application/json")
	reqNotify.Header.Set("Accept", "application/json, text/event-stream")
	reqNotify.Header.Set("mcp-session-id", sessionID)
	respNotify, err := client.Do(reqNotify)
	if err != nil {
		t.Fatalf("notification failed: %v", err)
	}
	respNotify.Body.Close()

	// 3. tools/list
	listPayload := map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/list",
		"params":  map[string]any{},
	}
	listBytes, _ := json.Marshal(listPayload)
	reqList, _ := http.NewRequest("POST", url, bytes.NewReader(listBytes))
	reqList.Header.Set("Content-Type", "application/json")
	reqList.Header.Set("Accept", "application/json, text/event-stream")
	reqList.Header.Set("mcp-session-id", sessionID)

	respList, err := client.Do(reqList)
	if err != nil {
		t.Fatalf("tools/list failed: %v", err)
	}
	defer respList.Body.Close()

	if respList.StatusCode != http.StatusOK {
		t.Fatalf("tools/list expected 200, got %d", respList.StatusCode)
	}

	listBodyBytes, _ := io.ReadAll(respList.Body)
	listParsed, err := parseMCPResponse(string(listBodyBytes))
	if err != nil {
		t.Fatalf("failed to parse list response: %v", err)
	}
	listRes, _ := listParsed["result"].(map[string]any)
	toolsList, _ := listRes["tools"].([]any)
	if len(toolsList) < 40 {
		t.Fatalf("expected at least 40 tools, got %d", len(toolsList))
	}

	// 4. tools/call (list_loaded_projects)
	callPayload := map[string]any{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "list_loaded_projects",
			"arguments": map[string]any{},
		},
	}
	callBytes, _ := json.Marshal(callPayload)
	reqCall, _ := http.NewRequest("POST", url, bytes.NewReader(callBytes))
	reqCall.Header.Set("Content-Type", "application/json")
	reqCall.Header.Set("Accept", "application/json, text/event-stream")
	reqCall.Header.Set("mcp-session-id", sessionID)

	respCall, err := client.Do(reqCall)
	if err != nil {
		t.Fatalf("tools/call failed: %v", err)
	}
	defer respCall.Body.Close()

	if respCall.StatusCode != http.StatusOK {
		t.Fatalf("tools/call expected 200, got %d", respCall.StatusCode)
	}

	callBodyBytes, _ := io.ReadAll(respCall.Body)
	callParsed, err := parseMCPResponse(string(callBodyBytes))
	if err != nil {
		t.Fatalf("failed to parse call response: %v", err)
	}
	callRes, _ := callParsed["result"].(map[string]any)
	contentArr, _ := callRes["content"].([]any)
	if len(contentArr) == 0 {
		t.Fatal("expected content in call result")
	}
}

func TestHttpTransportAuth(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen on port 0: %v", err)
	}

	manager := project.NewManager()
	srv := server.New(manager, "0.2.0")
	apiKeys := NewApiKeyStore([]string{"sk-secret-123", "sk-secret-456"})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	launcher := &HttpTransportLauncher{}
	go func() {
		_ = launcher.StartWithListener(ctx, listener, srv, apiKeys)
	}()

	client := &http.Client{Timeout: 10 * time.Second}
	url := fmt.Sprintf("http://%s/mcp", listener.Addr().String())

	initPayload := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2025-06-18",
			"capabilities":    map[string]any{},
			"clientInfo": map[string]any{
				"name":    "test",
				"version": "1",
			},
		},
	}
	initBytes, _ := json.Marshal(initPayload)

	// 1. Missing Authorization header -> 401
	reqAnon, _ := http.NewRequest("POST", url, bytes.NewReader(initBytes))
	reqAnon.Header.Set("Content-Type", "application/json")
	respAnon, err := client.Do(reqAnon)
	if err != nil {
		t.Fatal(err)
	}
	respAnon.Body.Close()
	if respAnon.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for anonymous, got %d", respAnon.StatusCode)
	}

	// 2. Wrong token -> 401
	reqWrong, _ := http.NewRequest("POST", url, bytes.NewReader(initBytes))
	reqWrong.Header.Set("Content-Type", "application/json")
	reqWrong.Header.Set("Authorization", "Bearer sk-invalid")
	respWrong, err := client.Do(reqWrong)
	if err != nil {
		t.Fatal(err)
	}
	respWrong.Body.Close()
	if respWrong.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for wrong token, got %d", respWrong.StatusCode)
	}

	// 3. Valid token -> 200
	reqValid, _ := http.NewRequest("POST", url, bytes.NewReader(initBytes))
	reqValid.Header.Set("Content-Type", "application/json")
	reqValid.Header.Set("Accept", "application/json, text/event-stream")
	reqValid.Header.Set("Authorization", "Bearer sk-secret-123")
	respValid, err := client.Do(reqValid)
	if err != nil {
		t.Fatal(err)
	}
	defer respValid.Body.Close()
	if respValid.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(respValid.Body)
		t.Fatalf("expected 200 for valid token, got %d (body: %s)", respValid.StatusCode, string(body))
	}
}
