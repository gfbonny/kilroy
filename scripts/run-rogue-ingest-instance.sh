#!/usr/bin/env bash
# run-rogue-ingest-instance.sh N
#
# For instance N:
#   1. Reads demo/rogue/rogue-prompt.txt from the kilroy repo.
#   2. Runs `kilroy attractor ingest` to generate a DOT pipeline from that prompt.
#   3. Runs the generated DOT via `kilroy attractor run`.
#
# Writes output artifacts to .kilroy/ relative to KILROY_WORKTREE_DIR (or CWD):
#   .kilroy/run_N_generated.dot  — the ingest-produced DOT file
#   .kilroy/run_N_output.txt     — combined stdout from both phases
#   .kilroy/run_N_meta.json      — run metadata (times, status, exit code)
set -euo pipefail

N="${1:-1}"

# Resolve the kilroy repo root from KILROY_SCRIPTS_DIR (set by attractor) or hardcoded fallback.
SCRIPTS_DIR="${KILROY_SCRIPTS_DIR:-/workspace/project/.kilroy/repos/kilroy/scripts}"
KILROY_REPO_ROOT="$(cd "$SCRIPTS_DIR/.." && pwd)"

PROMPT_FILE="$KILROY_REPO_ROOT/demo/rogue/rogue-prompt.txt"
SKILL_PATH="$KILROY_REPO_ROOT/skills/create-dotfile/SKILL.md"
CONFIG="/workspace/project/run-kilroy-compose.yaml"

# Write artifacts into .kilroy/ in the worktree (or CWD if env not set).
OUTDIR="${KILROY_WORKTREE_DIR:-.}/.kilroy"
mkdir -p "$OUTDIR"

GENERATED_DOT="$OUTDIR/run_${N}_generated.dot"
OUTPUT_FILE="$OUTDIR/run_${N}_output.txt"
META_FILE="$OUTDIR/run_${N}_meta.json"

: > "$OUTPUT_FILE"

echo "=== Instance $N: Ingesting rogue prompt ===" | tee -a "$OUTPUT_FILE"
echo "Prompt file: $PROMPT_FILE" | tee -a "$OUTPUT_FILE"
echo "Skill:       $SKILL_PATH" | tee -a "$OUTPUT_FILE"
echo "Config:      $CONFIG" | tee -a "$OUTPUT_FILE"
echo "" | tee -a "$OUTPUT_FILE"

if [ ! -f "$PROMPT_FILE" ]; then
  echo "ERROR: prompt file not found: $PROMPT_FILE" | tee -a "$OUTPUT_FILE"
  exit 1
fi

PROMPT=$(cat "$PROMPT_FILE")

kilroy attractor ingest \
  --output "$GENERATED_DOT" \
  --skill "$SKILL_PATH" \
  --repo "$KILROY_REPO_ROOT" \
  "$PROMPT" 2>&1 | tee -a "$OUTPUT_FILE"

echo "" | tee -a "$OUTPUT_FILE"
echo "=== Normalizing provider references (all -> google) ===" | tee -a "$OUTPUT_FILE"
# run-kilroy-compose.yaml uses google (Gemini) as the primary backend.
# Step 1: targeted sed for common providers/models (fast path)
sed -i \
  -e 's/llm_model: [a-zA-Z0-9_-]*\/[a-zA-Z0-9._-]*/llm_model: gemini-2.0-flash/g' \
  -e 's/llm_model="[a-zA-Z0-9_-]*\/[a-zA-Z0-9._-]*"/llm_model="gemini-2.0-flash"/g' \
  "$GENERATED_DOT"
# Step 2: Python catch-all — replace ANY remaining non-google provider and
# ANY remaining non-gemini-2.0-flash model. Handles deepseek, mistral, llama,
# qwen, and any other model family the ingestor may hallucinate.
python3 - "$GENERATED_DOT" << 'PYEOF'
import re, sys
path = sys.argv[1]
with open(path) as f:
    c = f.read()
# All providers → google
c = re.sub(r'llm_provider:\s+\S+', 'llm_provider: google', c)
c = re.sub(r'llm_provider\s*=\s*"[^"]*"', 'llm_provider="google"', c)
# All models → gemini-2.0-flash
c = re.sub(r'llm_model:\s+\S+', 'llm_model: gemini-2.0-flash', c)
c = re.sub(r'llm_model\s*=\s*"[^"]*"', 'llm_model="gemini-2.0-flash"', c)
with open(path, 'w') as f:
    f.write(c)
PYEOF
REMAINING_PROVIDERS=$(grep -cP 'llm_provider[: ="]+(?!google)[a-zA-Z]' "$GENERATED_DOT" 2>/dev/null || echo "0")
REMAINING_MODELS=$(grep -cP 'llm_model[: ="]+(?!gemini-2\.0-flash)[a-zA-Z]' "$GENERATED_DOT" 2>/dev/null || echo "0")
echo "Normalized: $(grep -c 'google' "$GENERATED_DOT") google refs, $REMAINING_PROVIDERS non-google provider refs, $REMAINING_MODELS non-gemini-2.0-flash model refs remaining" | tee -a "$OUTPUT_FILE"

echo "" | tee -a "$OUTPUT_FILE"
echo "=== Generated DOT ($GENERATED_DOT) ===" | tee -a "$OUTPUT_FILE"
cat "$GENERATED_DOT" | tee -a "$OUTPUT_FILE"
echo "" | tee -a "$OUTPUT_FILE"

START=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
printf '{"instance":%s,"started_at":"%s","generated_dot":"%s"}\n' \
  "$N" "$START" "$GENERATED_DOT" > "$META_FILE"

echo "=== Instance $N: Running generated pipeline ===" | tee -a "$OUTPUT_FILE"
set +e
kilroy attractor run --skip-cli-headless-warning \
  --graph "$GENERATED_DOT" \
  --config "$CONFIG" 2>&1 | tee -a "$OUTPUT_FILE"
RUN_EXIT=$?
set -e

END=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
STATUS="success"
[ "$RUN_EXIT" != "0" ] && STATUS="fail"

printf '{"instance":%s,"started_at":"%s","ended_at":"%s","status":"%s","exit_code":%s,"generated_dot":"%s"}\n' \
  "$N" "$START" "$END" "$STATUS" "$RUN_EXIT" "$GENERATED_DOT" > "$META_FILE"

echo "" | tee -a "$OUTPUT_FILE"
echo "=== Instance $N done: status=$STATUS exit_code=$RUN_EXIT ===" | tee -a "$OUTPUT_FILE"

exit "$RUN_EXIT"
