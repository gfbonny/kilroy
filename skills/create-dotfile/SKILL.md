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

1. Determine mode and hard constraints.
- If non-interactive/programmatic, do not ask follow-up questions.
- Extract explicit constraints (`no fanout`, model/provider requirements, deliverable paths).

2. Gather repo evidence.
- Read the authoritative spec/DoD sources if provided.
- Use repo docs and files to resolve ambiguity before making assumptions.

3. Choose topology from template first.
- Start from `reference_template.dot` for node shapes, routing, and loop structure.
- If user says `no fanout` or `single path`, remove fan-out/fan-in branch families.

### Implementation Decomposition

- If the task involves implementing or porting a codebase estimated to exceed ~1,000 lines of new code, decompose the `implement` node into per-module fan-out nodes (e.g. `implement_core`, `implement_api`, `implement_data_layer`) with a `merge_implementation` synthesis node. Each module node targets a bounded deliverable (~200–500 lines). A single `implement` node for large codebases produces stub implementations that pass structural checks but deliver no functional behavior.
- Use parallel fan-out (multiple `implement_X` → `merge_implementation`) or sequential chain as appropriate. Each `implement_X` node writes to `.ai/module_X_impl.md` and commits the code. `merge_implementation` synthesizes integration points and resolves conflicts.
- Threshold: >1,000 estimated lines of new code → decompose. The cost of extra nodes is much lower than a stub implementation.

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
- Require explicit success/fail/retry behavior. For fail/retry include `failure_reason` and `details` (and `failure_class` where applicable).
- Keep `.ai/*` producer/consumer paths exact; no filename drift.
- `shape=parallelogram` nodes must use `tool_command`.
- For compiled or packaged deliverables (executables, libraries, modules, services, containers, bundles): the verification node MUST validate the expected runtime behavior or interface contract — not just file existence or a successful build exit code.
- Add a domain-specific runtime validation node when needed (for example `verify_runtime`, `verify_api_contract`, `verify_cli_behavior`, `verify_ui_smoke`). Use checks that prove the deliverable actually works for the intended use case.
- A stub artifact can compile and still be functionally empty; require contract-level verification (exports, endpoints, CLI behavior, or observable outputs) to catch this.

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
- **Never** instruct any `shape=box` node to write `status: retry`. It is reserved by the attractor and triggers `deterministic_failure_cycle_check`, which downgrades to `fail` after N attempts. For iteration/revision loops, use a custom outcome: e.g. `{"status": "success", "outcome": "needs_revision"}` routed via `condition="outcome=needs_revision"` edge.
- **Never** instruct `review_consensus` (or any review/gate node) to write `status: fail` for a rejection verdict. Write a custom outcome instead: e.g. `{"status": "success", "outcome": "rejected"}`. `status: fail` triggers failure processing and blocks `goal_gate=true` re-execution. Route rejection via `condition="outcome=rejected"`.
- **Never use DOT/Graphviz reserved keywords as node IDs**: `if`, `node`, `edge`, `graph`, `digraph`, `subgraph`, `strict`. These cause routing failures — the DOT parser interprets them as language keywords rather than node names.
- **Every `goal_gate=true` node must declare its own `retry_target`** pointing to the appropriate recovery node (typically `postmortem`). The graph-level `retry_target` is for transient node failures and is not an appropriate retry path for a failed review/gate consensus. Example: `review_consensus [auto_status=true, goal_gate=true, retry_target="postmortem"]`.

## References

- `docs/strongdm/attractor/ingestor-spec.md`
- `docs/strongdm/attractor/attractor-spec.md`
- `docs/strongdm/attractor/coding-agent-loop-spec.md`
- `skills/create-dotfile/reference_template.dot`
- `skills/create-dotfile/preferences.yaml`
- `skills/shared/model_fallbacks.yaml`
- `internal/attractor/modeldb/pinned/openrouter_models.json`
- `internal/attractor/modeldb/manual_models.yaml`
