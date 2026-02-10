# Kimi Tool-Call Investigation Notes (Task 1)

Date: 2026-02-10
Scope: Investigation only (reproduction + localization), no adapter fix yet.

## What Was Added

- Test fixture:
  - `internal/llm/providers/anthropic/testdata/kimi_tool_call_sequences.ndjson`
- Regression tests:
  - `TestAdapter_Stream_ToolUse_StartInputPlusDelta_NoDuplicateJSON`
  - `TestAdapter_Stream_ToolUse_DeltaOnly_ValidJSON`
  - File: `internal/llm/providers/anthropic/adapter_test.go`

## Reproduction Command and Results

Command:

```bash
go test ./internal/llm/providers/anthropic -run 'ToolUse_StartInputPlusDelta_NoDuplicateJSON|ToolUse_DeltaOnly_ValidJSON' -count=1
```

Result:

- `TestAdapter_Stream_ToolUse_StartInputPlusDelta_NoDuplicateJSON`: **fails**
- `TestAdapter_Stream_ToolUse_DeltaOnly_ValidJSON`: **passes**

Focused failure output:

```text
tool call arguments must be valid JSON: "{\"command\":\"rg --files demo/rogue/original-rogue/*.c\"}{\"command\":\"rg --files demo/rogue/original-rogue/*.c\"}"
```

## Failing Sequence

From fixture sequence `start_input_plus_delta_duplicate`:

1. `content_block_start.tool_use` includes `input` object
2. `content_block_delta.input_json_delta.partial_json` provides the same JSON object payload
3. Adapter currently concatenates both payloads into a single args buffer

Observed assembled arguments:

```json
{"command":"rg --files demo/rogue/original-rogue/*.c"}{"command":"rg --files demo/rogue/original-rogue/*.c"}
```

This is invalid JSON and matches field errors seen in run artifacts (e.g. `invalid character '{' after top-level value`).

## Current Adapter Behavior (Localized)

In `internal/llm/providers/anthropic/adapter.go` stream handling:

- On `content_block_start` for `tool_use`, it marshals `content_block.input` and writes it to `st.toolArgs`.
- On `content_block_delta` with `input_json_delta`, it appends `partial_json` to the same `st.toolArgs` buffer.
- On `content_block_stop` and `message_stop`, it forwards this combined buffer as tool arguments.

This creates corruption when both start-input and delta represent complete JSON payloads.

## Expected Behavior

- Tool args should remain valid JSON regardless of whether provider emits:
  - start-input only
  - delta-only
  - start-input plus delta
- Tool-call IDs must remain stable across start/delta/end events.
- Finish reason should resolve to `tool_calls` when tool calls exist.

The new tests explicitly assert ID stability and finish reason behavior.

## Minimal Fix Options

### Option A: Source Precedence (recommended)

Track per-tool-call arg source (`start_input` and `delta_stream`) separately, and decide at finalize:

- if any delta was seen, prefer delta buffer
- else use start-input buffer

Tradeoffs:

- Pros: simple, deterministic, prevents direct concatenation corruption.
- Cons: assumes delta stream is complete when present; if a provider emits continuation-only deltas after start-input, this may need follow-up heuristics.

### Option B: Heuristic Merge on First Delta

When first delta arrives after start-input:

- if delta appears to start a full JSON document, replace existing start-input buffer
- otherwise append as continuation

Tradeoffs:

- Pros: handles both replay and continuation patterns.
- Cons: more branching/heuristics; higher risk of edge-case misclassification.

## External Corroboration

Existing run artifacts show downstream symptom consistent with this bug:

- `~/.local/state/kilroy/attractor/runs/rogue-fast-20260210T040700Z-timeout60s/impl_analysis/events.ndjson`
- repeated: `invalid tool arguments JSON: invalid character '{' after top-level value`
