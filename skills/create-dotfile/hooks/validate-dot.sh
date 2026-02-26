#!/usr/bin/env bash
# validate-dot.sh — PostToolUse hook for kilroy attractor .dot file validation
#
# Triggered by Claude Code after any Write or Edit tool call.
# Reads JSON from stdin (Claude Code hook protocol), extracts file_path,
# and if the file ends in .dot, runs kilroy attractor validate --graph.
#
# Exit codes (PostToolUse semantics):
#   0 — clean or no-op; Claude continues normally
#   2 — feedback written to stderr; Claude Code injects it into agent context
#
# Env vars:
#   KILROY_CLAUDE_PATH  — if set, directory or full path used to locate kilroy binary
#                         (mirrors the ingestor env var for the claude executable)

set -euo pipefail

# Read the full JSON payload from stdin.
HOOK_INPUT=$(cat)

# Extract tool_name and file_path from the JSON payload.
TOOL_NAME=$(printf '%s' "$HOOK_INPUT" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('tool_name',''))" 2>/dev/null || true)
FILE_PATH=$(printf '%s' "$HOOK_INPUT" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('tool_input',{}).get('file_path',''))" 2>/dev/null || true)

# Only act on Write or Edit tool calls.
case "$TOOL_NAME" in
    Write|Edit|MultiEdit) ;;
    *) exit 0 ;;
esac

# Only act on .dot files.
case "$FILE_PATH" in
    *.dot) ;;
    *) exit 0 ;;
esac

# Locate the kilroy binary.
# Honour KILROY_CLAUDE_PATH as a PATH hint (same env var the ingestor uses for
# the claude binary, repurposed here to override the kilroy binary location).
KILROY_BIN="kilroy"
if [[ -n "${KILROY_CLAUDE_PATH:-}" ]]; then
    if [[ -x "$KILROY_CLAUDE_PATH" ]]; then
        # Full path to the kilroy binary was provided.
        KILROY_BIN="$KILROY_CLAUDE_PATH"
    elif [[ -d "$KILROY_CLAUDE_PATH" ]]; then
        # A directory was provided; look for kilroy inside it.
        KILROY_BIN="$KILROY_CLAUDE_PATH/kilroy"
    fi
fi

# Verify kilroy is available.
if ! command -v "$KILROY_BIN" &>/dev/null && [[ ! -x "$KILROY_BIN" ]]; then
    # kilroy not in PATH — skip silently so the hook does not block writes
    # when the binary is absent (e.g., first-time bootstrap).
    exit 0
fi

# Run validation. Capture both stdout and stderr.
# kilroy attractor validate prints:
#   stdout: "ok: <file>" on success; "WARNING/ERROR: <msg> (<rule>)" for diagnostics
#   stderr: error details on fatal failure; exits non-zero
COMBINED_OUTPUT=$("$KILROY_BIN" attractor validate --graph "$FILE_PATH" 2>&1) || true

# Strip the "ok: <file>" success line — that is expected and not actionable.
FEEDBACK=$(printf '%s\n' "$COMBINED_OUTPUT" | grep -v '^ok: ' || true)

# If there is anything remaining (warnings or errors), return it as feedback.
# PostToolUse hooks must use exit 2 + stderr; stdout on exit 0 is not injected
# into the agent context.
if [ -n "$FEEDBACK" ]; then
    printf 'kilroy attractor validate found issues in %s — please repair before continuing:\n\n%s\n' \
        "$FILE_PATH" "$FEEDBACK" >&2
    exit 2
fi
exit 0
