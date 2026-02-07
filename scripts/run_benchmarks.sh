#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

# Full agent capability benchmarks (current: refactor trio).
# Runs with real LLM providers via API backend and a local fake CXDB.

BENCH_DOTS=(
  "research/refactor-test-vague.dot"
  "research/refactor-test-moderate.dot"
  "research/refactor-test-complex.dot"
)

ROOT_OUT="${KILROY_BENCH_OUT:-$PWD/.ai/benchmarks}"
mkdir -p "$ROOT_OUT"
STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
OUT_DIR="$ROOT_OUT/$STAMP"
mkdir -p "$OUT_DIR"

echo "bench_out=$OUT_DIR"

# Build Kilroy once.
go build -o ./kilroy ./cmd/kilroy

# Start a fake CXDB server (Go) in the background.
CXDB_BIN="$OUT_DIR/cxdb_fake"
cat > "$OUT_DIR/cxdb_fake.go" <<'GO'
package main

import (
  "fmt"
  "encoding/json"
  "io"
  "log"
  "net"
  "net/http"
  "strconv"
  "strings"
  "sync"
)

type srvState struct {
  mu sync.Mutex
  nextContextID int
  nextTurnID int
  contexts map[string]*ctxState
}

type ctxState struct {
  ContextID string
  HeadTurnID string
  HeadDepth int
}

func main() {
  st := &srvState{nextContextID: 1, nextTurnID: 1, contexts: map[string]*ctxState{}}

  mux := http.NewServeMux()
  mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
  mux.HandleFunc("/v1/registry/bundles/", func(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPut { w.WriteHeader(http.StatusMethodNotAllowed); return }
    w.WriteHeader(http.StatusCreated)
  })
  mux.HandleFunc("/v1/contexts", func(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost { w.WriteHeader(http.StatusMethodNotAllowed); return }
    baseTurnID := "0"
    b, _ := io.ReadAll(r.Body)
    _ = r.Body.Close()
    if strings.TrimSpace(string(b)) != "" {
      var req map[string]any
      _ = json.Unmarshal(b, &req)
      if v, ok := req["base_turn_id"]; ok {
        if s, ok := v.(string); ok {
          s = strings.TrimSpace(s)
          if s != "" { baseTurnID = s }
        }
      }
    }

    st.mu.Lock()
    id := strconv.Itoa(st.nextContextID)
    st.nextContextID++
    st.contexts[id] = &ctxState{ContextID: id, HeadTurnID: baseTurnID, HeadDepth: 0}
    ci := *st.contexts[id]
    st.mu.Unlock()

    _ = json.NewEncoder(w).Encode(map[string]any{
      "context_id": ci.ContextID,
      "head_turn_id": ci.HeadTurnID,
      "head_depth": ci.HeadDepth,
    })
  })
  mux.HandleFunc("/v1/contexts/", func(w http.ResponseWriter, r *http.Request) {
    rest := strings.TrimPrefix(r.URL.Path, "/v1/contexts/")
    parts := strings.Split(rest, "/")
    if len(parts) < 2 || parts[1] != "turns" { w.WriteHeader(http.StatusNotFound); return }
    ctxID := strings.TrimSpace(parts[0])
    if ctxID == "" { w.WriteHeader(http.StatusNotFound); return }
    if r.Method != http.MethodPost { w.WriteHeader(http.StatusMethodNotAllowed); return }

    st.mu.Lock()
    ci := st.contexts[ctxID]
    if ci == nil {
      st.mu.Unlock(); w.WriteHeader(http.StatusNotFound); return
    }
    turnID := strconv.Itoa(st.nextTurnID)
    st.nextTurnID++
    ci.HeadDepth++
    ci.HeadTurnID = turnID
    depth := ci.HeadDepth
    st.mu.Unlock()

    _ = json.NewEncoder(w).Encode(map[string]any{
      "context_id": ctxID,
      "turn_id": turnID,
      "depth": depth,
      "payload_hash": "h" + turnID,
      "content_hash": "h" + turnID,
    })
  })

  l, err := net.Listen("tcp", "127.0.0.1:0")
  if err != nil { log.Fatal(err) }
  addr := l.Addr().String()
  log.Printf("listening=%s", addr)
  // Print base URL for shell parsing.
  fmt.Println("http://" + addr)

  if err := http.Serve(l, mux); err != nil {
    log.Fatal(err)
  }
}
GO

go build -o "$CXDB_BIN" "$OUT_DIR/cxdb_fake.go"

CXDB_URL_FILE="$OUT_DIR/cxdb_url.txt"
"$CXDB_BIN" >"$CXDB_URL_FILE" 2>"$OUT_DIR/cxdb.log" &
CXDB_PID=$!

cleanup() {
  kill "$CXDB_PID" >/dev/null 2>&1 || true
}
trap cleanup EXIT

# Wait for URL to appear.
for _ in {1..50}; do
  if [[ -s "$CXDB_URL_FILE" ]]; then
    break
  fi
  sleep 0.1
done

CXDB_URL="$(tail -n 1 "$CXDB_URL_FILE" | tr -d '\r' | tr -d '\n' | xargs)"
if [[ -z "$CXDB_URL" ]]; then
  echo "failed to start fake cxdb" >&2
  exit 1
fi

echo "cxdb_url=$CXDB_URL"

# Minimal pinned catalog.
CATALOG="$OUT_DIR/model_prices_and_context_window.json"
cat > "$CATALOG" <<'JSON'
{
  "gpt-5.2": {
    "litellm_provider": "openai",
    "mode": "chat",
    "max_input_tokens": 1000,
    "max_output_tokens": 1000
  }
}
JSON

run_one() {
  local dot="$1"
  local name
  name="$(basename "$dot" .dot)"
  local workdir
  workdir="$OUT_DIR/$name"
  mkdir -p "$workdir"

  # Transform the benchmark DOT to run on OpenAI (API backend) by swapping the model stylesheet.
  # This avoids hard-depending on Anthropic/Gemini credentials being valid on first run.
  local graph
  graph="$workdir/graph.dot"
  python3 - "$dot" "$graph" <<'PY'
import re, sys, pathlib
src_path = pathlib.Path(sys.argv[1])
dst_path = pathlib.Path(sys.argv[2])
src = src_path.read_text()

replacement = (
    'model_stylesheet="\\n'
    '            * { llm_model: gpt-5-codex; llm_provider: openai; reasoning_effort: medium; }\\n'
    '            .hard { llm_model: gpt-5-codex; llm_provider: openai; reasoning_effort: high; }\\n'
    '            .verify { llm_model: gpt-5-mini; llm_provider: openai; reasoning_effort: medium; }\\n'
    '            .review { llm_model: gpt-5-codex; llm_provider: openai; reasoning_effort: high; }\\n'
    '        "'
)

out, n = re.subn(r'model_stylesheet\s*=\s*\".*?\"', replacement, src, count=1, flags=re.S)
if n != 1:
    raise SystemExit(f"expected to replace exactly 1 model_stylesheet, replaced {n}")
dst_path.write_text(out)
PY

  # Fresh git repo to operate on.
  local repo="$workdir/repo"
  mkdir -p "$repo"
  (cd "$repo" && git init -q && git config user.name tester && git config user.email tester@example.com && echo "hello" > README.md && git add -A && git commit -qm init)

  local cfg="$workdir/run.yaml"
  cat > "$cfg" <<YAML
version: 1
repo:
  path: $repo
cxdb:
  binary_addr: 127.0.0.1:9009
  http_base_url: $CXDB_URL
modeldb:
  litellm_catalog_path: $CATALOG
  litellm_catalog_update_policy: pinned
git:
  run_branch_prefix: attractor/run
llm:
  providers:
    openai:
      backend: api
YAML

  local run_id="bench-$name-$STAMP"
  local logs_root="$workdir/logs"
  mkdir -p "$logs_root"

  echo "== RUN $dot =="
  echo "graph=$graph"
  echo "run_id=$run_id"
  echo "logs_root=$logs_root"

  # Long runs: leave overall timeout to caller; per-node timeout is handled by the engine.
  set +e
  ./kilroy attractor run --graph "$graph" --config "$cfg" --run-id "$run_id" --logs-root "$logs_root" | tee "$workdir/run.out"
  local ec=${PIPESTATUS[0]}
  set -e

  echo "exit_code=$ec" | tee "$workdir/exit_code.txt"
  echo
  return $ec
}

fail=0
for dot in "${BENCH_DOTS[@]}"; do
  if run_one "$dot"; then
    :
  else
    echo "BENCH FAIL: $dot" >&2
    fail=1
  fi
  # Keep the system responsive.
  sleep 0.5
done

if [[ $fail -ne 0 ]]; then
  echo "One or more benchmarks failed" >&2
  exit 1
fi

echo "All benchmarks finished successfully"
