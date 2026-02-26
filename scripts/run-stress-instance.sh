#!/usr/bin/env bash
# Run a single kilroy instance as part of a parallel stress test harness.
# Called from tool nodes in parallel-run-harness.dot (run_1, run_2, run_3).
#
# Always exits 0 — results are written to numbered files in the worktree
# for the downstream analysis LLM to read.
#
# Usage: run-stress-instance.sh <run-number>
#   run-number: 1, 2, or 3
#
# Reads:  $KILROY_WORKTREE_DIR/.kilroy/target-dot-path.txt
# Writes: $KILROY_WORKTREE_DIR/.kilroy/run_N_final.json
#         $KILROY_WORKTREE_DIR/.kilroy/run_N_output.txt
#         $KILROY_WORKTREE_DIR/.kilroy/run_N_meta.json
#
# NOTE: $KILROY_STAGE_STATUS_PATH is NOT set for tool nodes — status is exit-code only.
# We always exit 0 so the parent pipeline proceeds to analysis even if a child run fails.
set -uo pipefail

RUN_NUM="${1:?Usage: run-stress-instance.sh <run-number>}"
WORKTREE="${KILROY_WORKTREE_DIR:-$(pwd)}"

# Locate the modeldb relative to this script's repo root, with env var override.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
KILROY_REPO_ROOT="$(dirname "$SCRIPT_DIR")"
MODELDB="${KILROY_MODELDB_PATH:-$KILROY_REPO_ROOT/internal/attractor/modeldb/pinned/openrouter_models.json}"

TARGET_PATH_FILE="$WORKTREE/.kilroy/target-dot-path.txt"
RESULT_FINAL="$WORKTREE/.kilroy/run_${RUN_NUM}_final.json"
RESULT_OUTPUT="$WORKTREE/.kilroy/run_${RUN_NUM}_output.txt"
RESULT_META="$WORKTREE/.kilroy/run_${RUN_NUM}_meta.json"

echo "=== Stress test run $RUN_NUM starting ==="

# Helper: write a failure sentinel and exit cleanly
write_error() {
  local reason="$1"
  local ts
  ts="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  echo "ERROR: $reason"
  jq -n \
    --arg run_num "$RUN_NUM" \
    --arg reason "$reason" \
    --arg ts "$ts" \
    '{run_num: $run_num, status: "error", failure_reason: $reason, started_at: $ts, ended_at: $ts}' \
    > "$RESULT_META"
  jq -n \
    --arg reason "$reason" \
    '{status: "error", failure_reason: $reason}' \
    > "$RESULT_FINAL"
  exit 0
}

# Read target DOT path written by the setup LLM node
if [ ! -f "$TARGET_PATH_FILE" ]; then
  write_error "target-dot-path.txt not found at $TARGET_PATH_FILE"
fi

DOT_FILE=$(tr -d '[:space:]' < "$TARGET_PATH_FILE")
echo "Target DOT: $DOT_FILE"

if [ -z "$DOT_FILE" ]; then
  write_error "target-dot-path.txt is empty"
fi

if [ ! -f "$DOT_FILE" ]; then
  write_error "DOT file not found: $DOT_FILE"
fi

# Build per-instance config pointing at the parent's worktree.
# The attractor will create its own branch (attractor/stress-N-<runid>) and isolated
# worktree from this repo, so concurrent runs do not share file state.
CHILD_CONFIG=$(mktemp /tmp/stress-config-XXXX.yaml)
cat > "$CHILD_CONFIG" <<EOF
version: 1
repo:
  path: $WORKTREE
cxdb:
  binary_addr: cxdb:9009
  http_base_url: http://cxdb:9010
  autostart:
    enabled: false
llm:
  cli_profile: real
  providers:
    anthropic:
      backend: cli
modeldb:
  openrouter_model_info_path: $MODELDB
  openrouter_model_info_update_policy: pinned
git:
  require_clean: false
  run_branch_prefix: attractor/stress-$RUN_NUM
  commit_per_node: true
runtime_policy:
  stage_timeout_ms: 0
  stall_timeout_ms: 600000
  stall_check_interval_ms: 5000
  max_llm_retries: 6
preflight:
  prompt_probes:
    enabled: false
EOF

# Heartbeat for log visibility while child run executes.
# Does NOT prevent stall watchdog in the parent config — that checks progress events.
(while true; do sleep 60; echo "[stress-run-$RUN_NUM heartbeat] still running..."; done) &
HEARTBEAT_PID=$!

cleanup() {
  kill "$HEARTBEAT_PID" 2>/dev/null || true
  wait "$HEARTBEAT_PID" 2>/dev/null || true
  rm -f "$CHILD_CONFIG"
}
trap cleanup EXIT

START_TIME=$(date -u +%Y-%m-%dT%H:%M:%SZ)
echo "Run $RUN_NUM started at $START_TIME"

# Run kilroy — capture all output, never fail the parent node
kilroy attractor run --skip-cli-headless-warning \
  --graph "$DOT_FILE" \
  --config "$CHILD_CONFIG" \
  2>&1 | tee "$RESULT_OUTPUT" || true

END_TIME=$(date -u +%Y-%m-%dT%H:%M:%SZ)
echo "Run $RUN_NUM finished at $END_TIME"

# Find the final.json from this child run (most recent in this container's home dir).
# Each tool node runs in its own container so there is no cross-run conflict.
RUNS_DIR="$HOME/.local/state/kilroy/attractor/runs"
CHILD_FINAL=$(ls -t "$RUNS_DIR"/*/final.json 2>/dev/null | head -1 || true)

CHILD_STATUS="no_output"
CHILD_RUN_ID="none"

if [ -n "$CHILD_FINAL" ] && [ -f "$CHILD_FINAL" ]; then
  cp "$CHILD_FINAL" "$RESULT_FINAL"
  CHILD_STATUS=$(jq -r '.status // "unknown"' "$RESULT_FINAL" 2>/dev/null || echo "unknown")
  CHILD_RUN_ID=$(jq -r '.run_id // "unknown"' "$RESULT_FINAL" 2>/dev/null || echo "unknown")
  echo "Run $RUN_NUM: status=$CHILD_STATUS run_id=$CHILD_RUN_ID"
else
  echo "WARNING: No final.json found for run $RUN_NUM — child produced no output"
  jq -n '{status: "error", failure_reason: "no final.json produced by child run"}' \
    > "$RESULT_FINAL"
fi

# Write per-run metadata for the analysis agent
jq -n \
  --arg run_num "$RUN_NUM" \
  --arg dot_file "$DOT_FILE" \
  --arg status "$CHILD_STATUS" \
  --arg run_id "$CHILD_RUN_ID" \
  --arg started "$START_TIME" \
  --arg ended "$END_TIME" \
  '{
    run_num: $run_num,
    dot_file: $dot_file,
    status: $status,
    run_id: $run_id,
    started_at: $started,
    ended_at: $ended
  }' > "$RESULT_META"

echo "=== Stress test run $RUN_NUM complete: $CHILD_STATUS ==="
exit 0
