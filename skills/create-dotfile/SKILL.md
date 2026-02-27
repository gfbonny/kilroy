---
name: create-dotfile
description: Use when authoring or repairing Kilroy Attractor DOT graphs from requirements, with template-first topology, routing guardrails, and validator-clean output.
---

# Create Dotfile

## Scope

This skill owns DOT graph authoring and repair for Attractor pipelines.

In scope:
- Turning requirements/spec/DoD into a runnable `.dot` graph.
- Defining topology, node prompts, routing, model assignments, and validation behavior.
- Enforcing DOT-specific guardrails and validator compatibility.

Out of scope:
- Run config (`run.yaml` / `run.json`) authoring and backend policy details. Use `create-runfile` for that.

## Overview

Core principle:
- Prefer validated template topology over ad-hoc graph design.
- Compose prompt text from project evidence; do not copy stale boilerplate.
- Optimize for reliable execution and recoverability, not novelty.

Default topology source:
- `skills/create-dotfile/reference_template.dot`

Model defaults source:
- `skills/create-dotfile/preferences.yaml`

## Workflow

0. Fetch the current model list (required before writing any model_stylesheet).

Run:
    kilroy attractor modeldb suggest

Capture the output. Use ONLY the model IDs listed in the output. Do not use
model IDs from memory — they go stale. If the command is unavailable, default
to: `claude-sonnet-4.6` (anthropic), `gemini-3-flash-preview` (google),
`gpt-4.1` (openai).

1. Determine mode and hard constraints.
- If non-interactive/programmatic, do not ask follow-up questions.
- Extract explicit constraints (`no fanout`, model/provider requirements, deliverable paths).

2. Gather repo evidence.
- Read the authoritative spec/DoD sources if provided.
- Use repo docs and files to resolve ambiguity before making assumptions.

3. Choose topology from template first.
- Start from `reference_template.dot` for node shapes, routing, and loop structure.
- If user says `no fanout` or `single path`, remove fan-out/fan-in branch families.
- **graph-level retry_target**: Set graph-level retry_target to the earliest node that
  preserves already-completed work on re-entry. For pipelines with an analysis or planning
  phase, point to plan_work or debate_consolidate so re-entry can reuse completed design
  docs and implementation files. Pointing retry_target at a specific implement_* node
  skips all sibling workers' output.

### Implementation Decomposition

- If the task involves implementing or porting a codebase estimated to exceed ~1,000 lines of new code, decompose the `implement` node into per-module fan-out nodes (e.g. `implement_core`, `implement_api`, `implement_data_layer`) with a `merge_implementation` synthesis node. Each module node targets a bounded deliverable (~200–500 lines). A single `implement` node for large codebases produces stub implementations that pass structural checks but deliver no functional behavior.
- Use parallel fan-out (multiple `implement_X` → `merge_implementation`) or sequential chain as appropriate. Each `implement_X` node writes to `.ai/module_X_impl.md` and commits the code. `merge_implementation` synthesizes integration points and resolves conflicts.
- Threshold: >1,000 estimated lines of new code → decompose. The cost of extra nodes is much lower than a stub implementation.

**Analyze-before-implement (required when porting or reading existing source):**
When the task involves translating, porting, or extending an existing codebase, insert a
dedicated analysis cluster BEFORE any implement_* nodes:

```
analyze_fanout (shape=component) → analyze_<module>×N (shape=box, auto_status=true)
  → merge_analysis (shape=box, auto_status=true) → [implement cluster]
```

Each analyze_<module> node reads specific source files and writes a compact design doc
to `.ai/design_<module>.md` (under 300 lines — spec only, no implementation code).
merge_analysis verifies all design docs exist and are cross-module consistent.
Only after merge_analysis succeeds does the pipeline proceed to implement_* nodes.
See `skills/create-dotfile/reference_template.dot` OPTIONAL comments for stub examples.

**Worker pool (for 5+ discrete deliverable files requiring idempotent resumability):**
When the task produces many independent files (e.g. 6 service modules, 8 API handlers), use a worker pool
instead of a simple fan-out to get per-file idempotency and stall detection:

```
plan_work (shape=box, auto_status=true)
  → work_pool (shape=component)
  → worker_0 / worker_1 / worker_2 (shape=box, auto_status=true)
  → check_work_complete (shape=box, auto_status=true)
  → [loop back to work_pool if outcome=more_work, else merge_implementation]
```

plan_work writes `.ai/work_queue.json` once (skips if already exists).
Each worker_N claims items where `item.id % N == N_index`, skips files already on disk.
check_work_complete reads `.ai/work_pass.txt` (pass counter); routes:
- `outcome=more_work` → work_pool (files remain)
- `outcome=all_done`  → merge_implementation
- `outcome=fail`      → postmortem (pass count > 5, stall guard)

Required properties: (1) idempotent — skip existing files; (2) modulo item assignment;
(3) pass-counter stall guard capping retries; (4) explicit all_done completion edge.

**auto_status=true assignment rules:**
- MUST: synthesis nodes — consolidate_*, merge_*, debate_*, review_consensus, postmortem
- SHOULD: all implement_*, worker_N, analyze_* nodes in fan-outs — their primary
  completion signal is the deliverable file; auto_status removes redundant success writes
- NEVER on shape=diamond routing nodes or shape=parallelogram tool nodes

4. Set model/provider resolution in `model_stylesheet`.
- Ensure every `shape=box` node resolves provider + model via attrs or stylesheet.
- Keep backend choice (`cli` vs `api`) out of DOT; backend belongs in run config.
- `model_stylesheet` declarations **MUST** use semicolons to separate property-value pairs within each selector block. Omitting semicolons causes silent parsing failures where nodes resolve to no provider.
  - Correct: `* { llm_model: gemini-2.0-flash; llm_provider: google; }`
  - Wrong: `* { llm_model: gemini-2.0-flash llm_provider: google }` — space-separated declarations silently fail.
- After writing `model_stylesheet`: verify each `{}` block uses only semicolon-terminated declarations; no two property names appear adjacent without a semicolon separator.
- **Anthropic model IDs use dot-separated version numbers**: `claude-opus-4.6`, `claude-sonnet-4.6`, `claude-haiku-4.5`. Never use dashes in the version suffix — `claude-opus-4-6` is wrong and will cause a validation ERROR. The three current canonical Anthropic IDs are: `claude-opus-4.6`, `claude-sonnet-4.6`, `claude-haiku-4.5`.

## Model Constraint Contract (Required)

- Treat explicit user model/provider directives as hard constraints.
- For explicit fan-out mappings, keep branch-to-model assignments one-to-one; do not reorder branches or merge assignments.
- Canonicalize provider aliases for DOT keys: `gemini`/`google_ai_studio` -> `google`, `z-ai`/`z.ai` -> `zai`, `moonshot`/`moonshotai` -> `kimi`, `minimax-ai` -> `minimax`.
- Resolve explicit model IDs against local evidence in this order:
1. exact user-provided ID (if already canonical),
2. `internal/attractor/modeldb/pinned/openrouter_models.json`,
3. `internal/attractor/modeldb/manual_models.yaml` (if present),
4. `skills/shared/model_fallbacks.yaml` (backup only when other sources fail).
- Never silently downgrade or substitute an explicit model request with a different major/minor family (example: requested `glm-5` must not become `glm-4.5`).
- If exact canonical resolution is unavailable, preserve the user-requested model literal in `llm_model` (normalize whitespace only) instead of guessing a nearby model.
- Apply known alias normalization from the fallback file before deciding unresolved status (for example: `glm-5.0` -> `glm-5` for provider `zai`).
- Explicit user model/provider directives override `skills/create-dotfile/preferences.yaml` defaults.

5. Compose node prompts and handoffs.
- Every `shape=box` prompt must include both `$KILROY_STAGE_STATUS_PATH` and `$KILROY_STAGE_STATUS_FALLBACK_PATH`.
- IMPORTANT: `auto_status=true` suppresses writing `status=success` on normal completion. It does NOT remove the requirement to include `$KILROY_STAGE_STATUS_PATH` and `$KILROY_STAGE_STATUS_FALLBACK_PATH` in the prompt — those paths are still required for failure-case writes. For auto_status nodes, phrase it as:
  ```
  "Write to $KILROY_STAGE_STATUS_PATH (fallback: $KILROY_STAGE_STATUS_FALLBACK_PATH)
   ONLY on failure: {"status":"fail","failure_reason":"...","failure_class":"..."}"
  ```
  Omitting the paths from auto_status node prompts causes `status_contract_in_prompt` warnings.
- Require explicit success/fail/retry behavior. For fail/retry include `failure_reason` and `details` (and `failure_class` where applicable).
- Keep `.ai/*` producer/consumer paths exact; no filename drift.
- `shape=parallelogram` nodes must use `tool_command`.
- For compiled or packaged deliverables (executables, libraries, modules, services, containers, bundles): the verification node MUST validate the expected runtime behavior or interface contract — not just file existence or a successful build exit code.
- Add a domain-specific runtime validation node when needed (for example `verify_runtime`, `verify_api_contract`, `verify_cli_behavior`, `verify_ui_smoke`). Use checks that prove the deliverable actually works for the intended use case.
- A stub artifact can compile and still be functionally empty; require contract-level verification (exports, endpoints, CLI behavior, or observable outputs) to catch this.
- The verify_fidelity prompt MUST enumerate acceptance criteria by numbered ID (AC1, AC2, ...), map each to the specific output file(s) that implement it:
  ```
  AC1: src/auth.py   — verify token issuance and expiry behaviour
  AC2: src/storage.py — verify data persistence across restart
  ```
  Use these IDs in the failure_signature field so postmortem can reference failing ACs precisely and the next verify pass can re-check only those criteria.
- **Verify nodes must call runtime-authored scripts, not hardcode tool invocations.** Any `shape=parallelogram` verify node whose pass/fail depends on decisions made by implementation nodes — package manager, build system, directory layout, language runtime — MUST use `tool_command="sh scripts/validate-{stage}.sh"` rather than inline tool calls. The implementation node responsible for project scaffolding MUST write `scripts/validate-build.sh`, `scripts/validate-fmt.sh`, and `scripts/validate-test.sh` as committed deliverables. Scripts MUST open with `#!/bin/sh` and MUST NOT assume any runtime beyond what `check_toolchain` confirmed. Rationale: ingest-time `tool_command` strings embed assumptions about structure the implementation loop has not yet made; when the loop's choices diverge — different directory names, package managers, or build systems — the verify node fails deterministically and no postmortem routing can repair it.

6. Enforce routing guardrails.
- Do not bypass actionable outcomes with unconditional pass-through edges.
- For nodes with conditional edges, include one unconditional fallback edge.
- Use only supported condition operators: `=`, `!=`, `&&`.
- Use `loop_restart=true` only for `context.failure_class=transient_infra`.
- The `postmortem` node **MUST** have at least three condition-keyed outbound edges covering distinct outcome classes (e.g. `impl_repair`, `needs_replan`, `needs_toolchain` or equivalents for the task domain) **before** the unconditional fallback. A `postmortem` with only one unconditional edge is invalid — it prevents recovery classification from routing differently and collapses all failure modes into a single path.
- The unconditional fallback from `postmortem` MUST come last among its outbound edges.

7. Preserve authoritative text contracts.
- If user explicitly provides goal/spec/DoD text, keep it verbatim (DOT-escape only).
- `expand_spec` must include the full user input verbatim in a delimited block.

8. Validate and repair before emit.
- Verify no unresolved placeholders (`DEFAULT_MODEL`, etc.).
- Run syntax + semantic validation loops, applying minimal fixes until clean.
- A PostToolUse hook (`skills/create-dotfile/hooks/validate-dot.sh`) runs automatically
  after every Write, Edit, or MultiEdit to a `.dot` file. It calls
  `kilroy attractor validate --graph` and, if issues are found, signals Claude Code
  via exit 2 + stderr so the feedback is injected into your context. If feedback
  appears, repair the reported issues immediately and re-write the file. No manual
  validate invocation is needed during ingest sessions.
- The hook requires `kilroy` in PATH. The `KILROY_CLAUDE_PATH` environment variable
  can override the binary location (full path or directory containing `kilroy`).

## Non-Negotiable Guardrails

- Programmatic output is DOT only (`digraph ... }`), no markdown fences or sentinel text.
- `shape=diamond` nodes route outcomes only; do not attach execution prompts.
- Keep prerequisite/tool gates real: route success/failure explicitly.
- Add deterministic checks for explicit deliverable paths named in requirements.
- For semantic verify stages, include a content-addressable `failure_signature` when failing repeated acceptance checks.
- **Never** instruct any `shape=box` node to write `status: retry`. It is reserved by the attractor and triggers `deterministic_failure_cycle_check`, which downgrades to `fail` after N attempts. For iteration/revision loops, use a custom outcome: e.g. `{"status": "needs_revision"}` routed via `condition="outcome=needs_revision"` edge.
- **Never** instruct `review_consensus` (or any review/gate node) to write `status: fail` for a rejection verdict. Write a custom outcome instead: e.g. `{"status": "rejected"}`. `status: fail` triggers failure processing and blocks `goal_gate=true` re-execution. Route rejection via `condition="outcome=rejected"`.
- **Never** write `{"status":"success","outcome":"..."}` in a status JSON — the `outcome` key is silently ignored by the runtime decoder; only `status` drives edge condition matching. Custom routing always uses the `status` field: `{"status":"more_work"}` matches `condition="outcome=more_work"`, not `{"status":"success","outcome":"more_work"}`.
- **Never use DOT/Graphviz reserved keywords as node IDs**: `if`, `node`, `edge`, `graph`, `digraph`, `subgraph`, `strict`. These cause routing failures — the DOT parser interprets them as language keywords rather than node names.
- **Every `goal_gate=true` node must declare its own `retry_target`** pointing to the appropriate recovery node (typically `postmortem`). The graph-level `retry_target` is for transient node failures and is not an appropriate retry path for a failed review/gate consensus. Example: `review_consensus [auto_status=true, goal_gate=true, retry_target="postmortem"]`.
- **Never embed runtime assumptions in verify `tool_command` fields.** Package manager invocations (`npm`, `cargo`, `pip`, `go build`), directory paths (`server/`, `client/`, `backend/`), and build tool flags MUST NOT appear directly in a verify node's `tool_command`. The only permitted form is `tool_command="sh scripts/validate-{stage}.sh"`. Violation causes deterministic cycle failures that postmortem cannot route out of: the script is hardcoded at ingest time, the implementation loop makes different structural choices at runtime, and every retry re-runs the same broken command against the same wrong structure until the cycle limit aborts the run.
- **Every `sh scripts/validate-*.sh` call MUST include a `KILROY_VALIDATE_FAILURE` fallback.** When a validate script is missing, crashes before printing output, or returns non-zero without diagnostic text, postmortem receives no repair signal and cannot distinguish a missing script from a genuine build failure. The canonical `tool_command` form is:
  ```
  tool_command="sh scripts/validate-{stage}.sh || { echo 'KILROY_VALIDATE_FAILURE: validate-{stage}.sh missing or failed — postmortem must write scripts/validate-{stage}.sh'; exit 1; }"
  ```
  Within each `scripts/validate-{stage}.sh` authored by implementation nodes, include a POSIX sh failure trap as the first executable line:
  ```sh
  #!/bin/sh
  set -e
  trap 'echo "KILROY_VALIDATE_FAILURE: validate-{stage}.sh crashed at line $LINENO — postmortem must repair scripts/validate-{stage}.sh"' EXIT
  # ... stage-specific checks ...
  trap - EXIT
  ```
  The `KILROY_VALIDATE_FAILURE:` prefix is the linter-enforced token. The validator rule `validate_script_failure_contract` fires a warning when `sh scripts/validate-*.sh` appears in `tool_command` without this token.

## References

- `docs/strongdm/attractor/ingestor-spec.md`
- `docs/strongdm/attractor/attractor-spec.md`
- `docs/strongdm/attractor/coding-agent-loop-spec.md`
- `skills/create-dotfile/reference_template.dot`
- `skills/create-dotfile/preferences.yaml`
- `skills/shared/model_fallbacks.yaml`
- `internal/attractor/modeldb/pinned/openrouter_models.json`
- `internal/attractor/modeldb/manual_models.yaml`
