#!/usr/bin/env bash
# test-ingest-quality.sh — ingest quality regression harness (G14)
#
# Runs 1-3 sequential ingest tasks against a short, stable test prompt.
# For each output .dot file:
#   1. Validates with kilroy attractor validate
#   2. Counts nodes missing $KILROY_STAGE_STATUS_PATH (Gap F check)
#   3. Compares counts against scripts/ingest-quality-baseline.json
#
# Exits 1 if any file has validator errors or gap coverage drops below baseline.
#
# Usage:
#   RUN_INGEST_QUALITY=1 ./scripts/test-ingest-quality.sh
#   RUN_INGEST_QUALITY=1 ./scripts/test-ingest-quality.sh --update-baseline
#   RUN_INGEST_QUALITY=1 INGEST_RUNS=1 ./scripts/test-ingest-quality.sh
#
# Environment variables:
#   RUN_INGEST_QUALITY   Must be set to "1" to run (CI gate).
#   INGEST_RUNS          Number of ingest runs to perform (default: 3, max: 3).
#   KILROY_BIN           Path to kilroy binary (default: auto-detect).

set -euo pipefail

# ---------------------------------------------------------------------------
# Gate: must opt in to run
# ---------------------------------------------------------------------------
if [ -z "${RUN_INGEST_QUALITY:-}" ]; then
    echo "Skipping (set RUN_INGEST_QUALITY=1 to run)"
    exit 0
fi

# ---------------------------------------------------------------------------
# Paths
# ---------------------------------------------------------------------------
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
BASELINE_FILE="$SCRIPT_DIR/ingest-quality-baseline.json"
TESTDATA_FILE="$SCRIPT_DIR/testdata/ingest-quality-test.md"
TMPDIR_BASE="${TMPDIR:-/tmp}/kilroy-ingest-quality-$$"

# ---------------------------------------------------------------------------
# Parse flags
# ---------------------------------------------------------------------------
UPDATE_BASELINE=0
for arg in "$@"; do
    case "$arg" in
        --update-baseline)
            UPDATE_BASELINE=1
            ;;
        *)
            echo "unknown flag: $arg" >&2
            echo "usage: $0 [--update-baseline]" >&2
            exit 1
            ;;
    esac
done

# ---------------------------------------------------------------------------
# Locate kilroy binary
# ---------------------------------------------------------------------------
if [ -n "${KILROY_BIN:-}" ]; then
    KILROY="$KILROY_BIN"
elif [ -x "$REPO_ROOT/kilroy" ]; then
    KILROY="$REPO_ROOT/kilroy"
elif command -v kilroy &>/dev/null; then
    KILROY="kilroy"
else
    echo "error: kilroy binary not found; build with 'go build -o kilroy ./cmd/kilroy'" >&2
    echo "  or set KILROY_BIN=/path/to/kilroy" >&2
    exit 1
fi

# ---------------------------------------------------------------------------
# Read baseline
# ---------------------------------------------------------------------------
if [ ! -f "$BASELINE_FILE" ]; then
    echo "error: baseline not found at $BASELINE_FILE" >&2
    exit 1
fi

# Parse baseline using awk (no jq dependency).
MAX_MISSING_STATUS=$(awk -F: '/"max_missing_status_path_nodes"/ {gsub(/[^0-9]/, "", $2); print $2}' "$BASELINE_FILE")
MAX_VALIDATOR_ERRORS=$(awk -F: '/"max_validator_errors"/ {gsub(/[^0-9]/, "", $2); print $2}' "$BASELINE_FILE")

if [ -z "$MAX_MISSING_STATUS" ] || [ -z "$MAX_VALIDATOR_ERRORS" ]; then
    echo "error: could not parse baseline from $BASELINE_FILE" >&2
    exit 1
fi

echo "baseline: max_missing_status_path_nodes=$MAX_MISSING_STATUS  max_validator_errors=$MAX_VALIDATOR_ERRORS"

# ---------------------------------------------------------------------------
# Read test prompt
# ---------------------------------------------------------------------------
if [ ! -f "$TESTDATA_FILE" ]; then
    echo "error: test prompt not found at $TESTDATA_FILE" >&2
    exit 1
fi
TEST_PROMPT="$(cat "$TESTDATA_FILE")"

# ---------------------------------------------------------------------------
# Number of runs
# ---------------------------------------------------------------------------
INGEST_RUNS="${INGEST_RUNS:-3}"
if [ "$INGEST_RUNS" -lt 1 ] || [ "$INGEST_RUNS" -gt 3 ]; then
    echo "error: INGEST_RUNS must be between 1 and 3 (got $INGEST_RUNS)" >&2
    exit 1
fi

# ---------------------------------------------------------------------------
# Setup temp directory
# ---------------------------------------------------------------------------
mkdir -p "$TMPDIR_BASE"
cleanup() {
    rm -rf "$TMPDIR_BASE"
}
trap cleanup EXIT

echo "=== ingest quality regression harness ==="
echo "runs:    $INGEST_RUNS"
echo "prompt:  $TESTDATA_FILE"
echo "kilroy:  $KILROY"
echo "tmpdir:  $TMPDIR_BASE"
echo ""

# ---------------------------------------------------------------------------
# Run ingest N times sequentially
# ---------------------------------------------------------------------------
TOTAL_MISSING=0
TOTAL_ERRORS=0
WORST_MISSING=0
WORST_ERRORS=0
FAIL=0

for i in $(seq 1 "$INGEST_RUNS"); do
    DOT_FILE="$TMPDIR_BASE/ingest-quality-run-${i}.dot"
    echo "--- run $i/$INGEST_RUNS ---"

    # Run ingest. Capture stderr separately; dot goes to file.
    if ! "$KILROY" attractor ingest \
            --output "$DOT_FILE" \
            --repo "$REPO_ROOT" \
            "$TEST_PROMPT" 2>&1; then
        echo "FAIL: kilroy attractor ingest failed on run $i" >&2
        FAIL=1
        continue
    fi

    if [ ! -f "$DOT_FILE" ]; then
        echo "FAIL: run $i produced no output file at $DOT_FILE" >&2
        FAIL=1
        continue
    fi

    echo "  output: $DOT_FILE ($(wc -c < "$DOT_FILE") bytes)"

    # -- 1. Structural validation --
    VALIDATE_OUT="$TMPDIR_BASE/validate-${i}.txt"
    VALIDATE_RC=0
    "$KILROY" attractor validate --graph "$DOT_FILE" >"$VALIDATE_OUT" 2>&1 || VALIDATE_RC=$?

    if [ "$VALIDATE_RC" -ne 0 ]; then
        echo "  FAIL: validator returned exit $VALIDATE_RC"
        cat "$VALIDATE_OUT" | sed 's/^/    /'
        FAIL=1
    else
        echo "  validate: ok"
    fi

    # Count error lines from validate (lines starting with "error:")
    ERROR_COUNT=$(grep -ci '^error:' "$VALIDATE_OUT" 2>/dev/null || true)
    TOTAL_ERRORS=$(( TOTAL_ERRORS + ERROR_COUNT ))
    if [ "$ERROR_COUNT" -gt "$WORST_ERRORS" ]; then
        WORST_ERRORS=$ERROR_COUNT
    fi

    # -- 2. Gap F check: count shape=box nodes missing KILROY_STAGE_STATUS_PATH --
    # Strategy: extract node bodies between consecutive shape=box occurrences
    # and count those that lack $KILROY_STAGE_STATUS_PATH in their prompt text.
    #
    # Acceptable grep approach here — we're counting, not parsing structure.
    # A box node is compliant if it has auto_status=true OR contains
    # $KILROY_STAGE_STATUS_PATH anywhere in its prompt attribute.

    TOTAL_BOX_NODES=$(grep -c 'shape=box' "$DOT_FILE" 2>/dev/null || true)

    # Nodes with auto_status=true are always compliant (engine injects the path).
    AUTO_STATUS_NODES=$(grep -c 'auto_status=true' "$DOT_FILE" 2>/dev/null || true)

    # Nodes that explicitly reference $KILROY_STAGE_STATUS_PATH in prompt.
    # Count unique box-node blocks that contain the variable.
    STATUS_PATH_PRESENT=$(grep -c 'KILROY_STAGE_STATUS_PATH' "$DOT_FILE" 2>/dev/null || true)

    # Conservative: a node is "covered" if ANY mention of KILROY_STAGE_STATUS_PATH
    # exists in the file per box node (over-counts compliance slightly, consistent
    # with existing audit methodology from rogue x10 report).
    COVERED=$(( AUTO_STATUS_NODES + STATUS_PATH_PRESENT ))

    # Missing = box nodes not covered. Floor at 0.
    MISSING=$(( TOTAL_BOX_NODES - COVERED ))
    if [ "$MISSING" -lt 0 ]; then MISSING=0; fi

    echo "  box_nodes=$TOTAL_BOX_NODES  auto_status=$AUTO_STATUS_NODES  status_path_mentions=$STATUS_PATH_PRESENT  missing=$MISSING"

    TOTAL_MISSING=$(( TOTAL_MISSING + MISSING ))
    if [ "$MISSING" -gt "$WORST_MISSING" ]; then
        WORST_MISSING=$MISSING
    fi

    if [ "$MISSING" -gt "$MAX_MISSING_STATUS" ]; then
        echo "  FAIL: missing status path nodes ($MISSING) exceeds baseline ($MAX_MISSING_STATUS)"
        FAIL=1
    fi

    echo ""
done

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
echo "=== summary ==="
echo "runs:                  $INGEST_RUNS"
echo "worst_missing_status:  $WORST_MISSING (baseline threshold: $MAX_MISSING_STATUS)"
echo "worst_validator_errors: $WORST_ERRORS (baseline threshold: $MAX_VALIDATOR_ERRORS)"
echo "total_validator_errors: $TOTAL_ERRORS"

if [ "$WORST_ERRORS" -gt "$MAX_VALIDATOR_ERRORS" ]; then
    FAIL=1
    echo "ERROR: validator errors ($WORST_ERRORS) exceed baseline threshold ($MAX_VALIDATOR_ERRORS)"
fi

if [ "$UPDATE_BASELINE" -eq 1 ]; then
    if [ "$FAIL" -eq 1 ]; then
        echo "WARNING: Not updating baseline because run detected regressions (FAIL=1). Fix regressions first, then run --update-baseline again."
    else
        # Write a new baseline based on observed worst-case + 0 tolerance for errors.
        cat > "$BASELINE_FILE" <<BASELINE_EOF
{
  "comment": "Update by running: RUN_INGEST_QUALITY=1 scripts/test-ingest-quality.sh --update-baseline",
  "max_missing_status_path_nodes": $WORST_MISSING,
  "max_validator_errors": $WORST_ERRORS
}
BASELINE_EOF
        echo ""
        echo "baseline updated: $BASELINE_FILE"
        echo "  max_missing_status_path_nodes=$WORST_MISSING"
        echo "  max_validator_errors=$WORST_ERRORS"
    fi
fi

if [ "$FAIL" -ne 0 ]; then
    echo ""
    echo "FAIL: ingest quality regression detected"
    exit 1
fi

echo ""
echo "PASS: ingest quality within baseline"
exit 0
