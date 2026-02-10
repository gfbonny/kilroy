# Rogue Runs: Best-of-Both Fix Plan

## Core invariants (must hold after fixes)

- Active branch work must always count as parent-run progress.
- No stage may emit progress after its attempt has ended.
- Canceled runs must converge quickly to terminal state.
- Fail outcomes must preserve causal `failure_reason` across routing nodes.
- Stage status must be owned by the same stage/attempt that consumes it.
- Every run that terminates via controllable engine paths must persist an explicit top-level terminal artifact (`final.json`).

## Spec alignment guardrails (idiomatic Attractor/Kilroy path)

- Keep the canonical stage outcome contract from metaspec: `{logs_root}/{node_id}/status.json` is authoritative, statuses remain canonical lowercase, and `failure_reason` stays non-empty for `fail`/`retry`.
- Treat `.ai/status.json` as a legacy compatibility input only; never replace canonical stage-directory `status.json` as the source of routing truth.
- Preserve edge-routing semantics (condition evaluation, retry-target/fallback behavior, and deterministic tie-break order); do not introduce special-case routing shortcuts.
- Preserve parallel isolation semantics: branch-local git branch/worktree + CXDB fork, with fan-in selecting one winner via ff-only integration.
- Preserve artifact persistence expectations: `checkpoint.json`, stage artifacts, and top-level `final.json` remain first-class persisted outputs.
- Keep provider behavior config-driven via runtime metadata (backend/profile/failover from run config), not hardcoded provider-specific branches.

## P0 (must fix first)

- Make watchdog liveness fanout-aware: parent watchdog must treat child branch activity as progress.
  - Done when: active fanout branches cannot trigger false `stall_watchdog_timeout`.
- Add API `agent_loop` progress plumbing (`appendProgress`) so active API stages are visible to watchdog/observers.
  - Done when: long-running API stages produce periodic progress events comparable to CLI visibility.
  - Note: this alone is insufficient; parent watchdog must consume branch activity, not only branch-local logs.
- Stop CLI heartbeat leaks: end heartbeat goroutine exactly when the stage process exits and key heartbeat events by attempt ID.
  - Done when: zero heartbeats appear after `stage_attempt_end` for that `(node_id, attempt_id)`.
- Add cancellation guards in `runSubgraphUntil` at loop top and post-stage execution.
  - Done when: canceled contexts terminate branch loops immediately (no high-frequency fail cycling).
- Port deterministic failure-cycle breaker into subgraph execution (parity with main `runLoop`).
  - Done when: repeated deterministic branch loops abort within configured signature limits.
- Propagate `failure_reason` and `failure_class` in subgraph context (parity with main path).
  - Done when: check/conditional nodes retain upstream causal failures (no validator-only generic reasons).
- Harden status ingestion contract (`status.json` vs `.ai/status.json`) with deterministic precedence, atomic copy, and explicit diagnostics.
  - Done when: status discovery decisions are traceable, canonical stage status remains authoritative, status ownership is validated (`node` matches stage when present), and there are no false "missing status.json" failures when valid status was produced.
- Guarantee top-level terminalization on all controllable exit paths.
  - Done when: success/fail/watchdog cancel/context cancel/internal fatal errors all persist top-level `final.json` (best-effort; excludes uncatchable hard kill like `SIGKILL`).

## P1 (high-value hardening)

- Reclassify canceled-context/stall-timeout failures separately from deterministic provider/API failures.
- Normalize failure signatures so semantically identical reasons collapse and cycle breakers trigger reliably.
- Enforce strict model/failover policy: if config pins provider/model (especially production/no-failover), block implicit fallback.
- Improve provider/tool adaptation (especially `apply_patch` contract handling for openai-family API profiles).
- Add parent rollup telemetry for branch status summaries to improve live triage without drilling into branch dirs.

## Required observability (for proving fixes)

- Emit explicit branch-to-parent liveness events (or equivalent parent rollup counters).
- Emit attempt IDs on all stage lifecycle/progress events (`stage_attempt_start`, `stage_attempt_end`, `stage_heartbeat`) and include monotonic elapsed in heartbeats.
- Emit status-ingestion decision events: searched paths, selected path, parse result, ownership check result.
- Emit subgraph cancellation exit event with terminal reason and node where loop stopped.
- Emit deterministic-cycle breaker event in subgraph path with signature count and configured limit.

## P2 (validation and prevention)

- Add regression test: fanout watchdog with active child progress must not false-timeout.
- Add regression test: no stale heartbeats after attempt completion.
- Add regression test: subgraph cancellation terminates immediately.
- Add regression test: subgraph deterministic cycle breaker trips as configured.
- Add regression test: status ingestion correctness for `status.json` and `.ai/status.json`.
- Add regression test: failure propagation through check/conditional nodes preserves causal reason/class.
- Add regression test: top-level `final.json` persistence on all terminal paths.
- Add regression test: model pin/failover enforcement obeys run config exactly.
- Add regression test: status ownership enforcement prevents cross-stage status reuse.
- Add regression test: true-positive watchdog timeout still fires when no top-level or branch activity exists.

## Suggested implementation order

- Land observability events first (minimal behavior change, maximal diagnostic value).
- Ship subgraph cancellation + subgraph cycle breaker + subgraph failure propagation together.
- Ship watchdog liveness + API progress plumbing + heartbeat attempt-scoping together.
- Ship terminalization guarantee and status-ingestion hardening.
- Land classification/signature/tooling hardening.
- Gate completion on the full regression suite above.

## Primary touchpoints

- `internal/attractor/engine/parallel_handlers.go` (fanout orchestration, parent/branch liveness integration).
- `internal/attractor/engine/subgraph.go` (cancellation guards, deterministic-cycle logic parity, context propagation parity).
- `internal/attractor/engine/engine.go` (watchdog, terminalization/final artifact guarantees, failure signature normalization hooks).
- `internal/attractor/engine/codergen_router.go` (CLI heartbeat lifecycle, API agent-loop progress emission).
- `internal/attractor/engine/handlers.go` (status discovery/ingestion contract and status ownership validation).
- `internal/attractor/runtime/status.go` (status validation constraints and compatibility behavior).

## Release gates

- No new stale-heartbeat events in canary runs.
- No fanout false-timeout in canary runs with active branches.
- Deterministic branch loops terminate within configured limits.
- Every terminated run has top-level `final.json` with correct terminal status and reason.
- No regressions in canonical status/routing semantics (`status.json` contract, lowercase outcomes, fail/retry `failure_reason`).
