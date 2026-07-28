#!/usr/bin/env bash
# smoke.sh — end-to-end smoke test for go-mcp-server.
#
# Pipes JSON-RPC newline-delimited to the binary via stdin and verifies:
#   1. initialize             → serverInfo go-mcp-server
#   2. notifications/initialized AS REQUEST (with id) — opencode quirk
#   3. tools/list             → contains tools
#   4. tools/call list_loaded_projects (empty)
#   5. tools/call load_go_project (real fixture)
#   6. tools/call list_functions (on the loaded project)
#
# Usage: ./scripts/smoke.sh
set -euo pipefail

cd "$(dirname "$0")/.."

BIN="${BIN:-./go-mcp-server}"
echo "== building =="
go build -o "$BIN" .

FIXTURE="$PWD/internal/tools/testdata/samplemodule"

# PHASE 1: independent requests. NOTE: the go-sdk dispatches each request in
# its own goroutine — responses may arrive OUT OF ORDER (id 4 before id 3).
# Dependent requests (list_functions needs load to finish) go to PHASE 2,
# after a pause.
PHASE1='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"smoke","version":"0"}}}
{"jsonrpc":"2.0","id":2,"method":"notifications/initialized"}
{"jsonrpc":"2.0","id":3,"method":"tools/list","params":{}}
{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"list_loaded_projects","arguments":{}}}
{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"load_go_project","arguments":{"source":"'"$FIXTURE"'","alias":"smoke"}}}'

PHASE2='{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"list_functions","arguments":{"name":"smoke"}}}'

echo "== sending requests (2 phases) =="
# The pipe stays open: the go-sdk closes the session as soon as it sees EOF
# on stdin and would discard pending responses. The pause between phases
# ensures load (id 5) completes before list_functions (id 6).
RESPONSES=$( { printf '%s\n' "$PHASE1"; sleep 2; printf '%s\n' "$PHASE2"; sleep 2; } | "$BIN" 2>/dev/null || true)

echo "$RESPONSES" | while IFS= read -r line; do
  echo "<<< $line"
done

FAIL=0
check() {
  local desc="$1" pattern="$2"
  if printf '%s\n' "$RESPONSES" | grep -q "$pattern"; then
    echo "PASS: $desc"
  else
    echo "FAIL: $desc (pattern: $pattern)"
    FAIL=1
  fi
}

check "initialize → serverInfo"       '"name":"go-mcp-server"'
check "initialize → instructions"     'Ground truth for AI agents'
check "tools/list → tool iter 1"      '"name":"list_loaded_projects"'
check "tools/list → tool iter 2"      '"name":"load_go_project"'
check "tools/list → tool análisis"    '"name":"inspect_struct"'
check "tools/call → empty markdown"   'No projects loaded'
check "load_go_project → loaded"      '# Project Loaded'
check "load_go_project → module path" 'example.com/samplemodule'
check "list_functions → 10 functions" '# Functions (10)'

if [ "$FAIL" -ne 0 ]; then
  echo "== SMOKE FAILED =="
  exit 1
fi
echo "== SMOKE OK =="
