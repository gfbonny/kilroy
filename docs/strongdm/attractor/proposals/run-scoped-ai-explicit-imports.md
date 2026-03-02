# Proposal: Run-Scoped `.ai` and Explicit Imports

## Problem

The March 2, 2026 incident (`run_id=01KJPDK649C65Y07TBX1041C73`) exposed two
separate problems:

1. **Immediate failure mode (confirmed):** a stale local `./kilroy` binary
   (built from `cee6fe8e...`) used old `copyInputFile` logic that truncated
   same-path copies to zero bytes.
2. **Design gap (still real):** startup input materialization implicitly copied
   gitignored repo-local `.ai/*.md` files from the developer checkout. That is
   how a stale **Solitaire** spec entered this run.

The solitaire provenance is confirmed:

- Source file: `/home/user/code/kilroy/.ai/spec.md` (local, gitignored).
- Copied at run startup into:
  - `logs_root/input_snapshot/files/.ai/spec.md`
  - `logs_root/worktree/.ai/spec.md`
- Snapshot file hash matched source hash exactly.

This proves we need stronger input boundaries, not just a stale-binary fix.

## Decision

Adopt a strict runtime model:

1. **`.ai` is engine-owned run state only.**
   - All runtime files live under `./.ai/runs/<run_id>/...`.
   - No repo-level `.ai/*` input ingestion.
2. **Shared project inputs live outside `.ai`.**
   - Examples: `docs/`, `specs/`, `policies/`, `requirements/`.
3. **Runs use explicit imports only.**
   - Operator declares imports in run config.
   - Engine snapshots imports into run-scoped storage.
4. **Execution reads from run snapshots, not live repo files.**
   - Fan-out branches and resume hydrate from the same snapshot state.

## Why Shared Sources Still Matter

Run-scoped execution does not eliminate the need for shared project sources.

Shared sources are still needed for:

1. Reviewed, versioned canonical docs (PR workflow, ownership, audit trail).
2. Stable baselines used across many runs.
3. Persistent policy files (security gates, coding standards, waivers).
4. Reuse of expensive precomputed metadata.

The correct split is:

- **Authoritative source:** normal repo paths (outside `.ai`).
- **Runtime working set:** copied snapshot under `./.ai/runs/<run_id>/...`.

## Proposed Design

### 1. Input Declaration

- Replace implicit `.ai/*.md` startup ingestion with explicit run-config imports.
- Fail closed by default:
  - no implicit repo-root `.ai/*` matches
  - no broad default globs that sweep local scratch files

### 2. Snapshot Materialization

At run start:

1. Resolve declared imports from source roots.
2. Copy imported files into `logs_root/input_snapshot/files/...`.
3. Hydrate active worktree into `./.ai/runs/<run_id>/inputs/...`.
4. Record source path + digest in manifest for provenance and replay.

During execution:

1. Agent outputs and scratch files are written only under
   `./.ai/runs/<run_id>/...`.
2. After each node, update snapshot from run-scoped runtime paths so branch
   worktrees and resume see current state.
3. Keep same-file copy guard to prevent truncation regressions.

### 3. Read/Write Contract

- Nodes read inputs from run-scoped paths.
- Nodes write outputs/scratch to run-scoped paths.
- Engine can expose `KILROY_RUN_DIR` to avoid hardcoding run IDs in prompts.

### 4. Optional Publish Step

If a run output should become shared project truth, require explicit promotion
(for example `publish` action or follow-up commit), never implicit leakage from
runtime `.ai` state.

## Why This Is Better Than Copying "Everything" Per Run

Copying everything per run without explicit shared-source boundaries still
leaves one unresolved question: **everything from where**.

This design answers it safely:

1. Shared sources are explicit and reviewed.
2. Runtime copies are isolated and reproducible.
3. Random local scratch files cannot silently enter runs.
4. Re-run fidelity is enforceable with input digests.

## Implementation Plan

1. **Immediate guardrail**
   - Remove/disable implicit repo-level `.ai/*.md` ingestion.
   - Ensure startup source-root scans skip repo `.ai/**` by default.
2. **Run-scoped runtime paths**
   - Standardize on `./.ai/runs/<run_id>/...`.
   - Provide `KILROY_RUN_DIR` to handlers/agents.
3. **Explicit import schema**
   - Add/normalize config field for imports.
   - Enforce explicit include semantics.
4. **Snapshot sync**
   - Update the run snapshot after each node, but only for run-scoped runtime files.
   - Preserve same-file copy protections.
5. **Provenance + diagnostics**
   - Persist per-import source and digest metadata.
   - Record binary revision in run artifacts for forensics.
6. **Migration**
   - Update built-in templates and docs to use run-scoped paths.
   - Add compatibility guidance for existing graphs that reference `.ai/*.md`.

## Test Plan

1. Startup materialization does not ingest repo `.ai/*` unless explicitly
   imported.
2. Explicit import from `docs/...` appears in run-scoped input snapshot.
3. Node output in `./.ai/runs/<run_id>/...` persists across linear nodes.
4. Parallel branch worktrees hydrate latest run-scoped snapshot state.
5. Resume hydrates same run-scoped state from snapshot.
6. Same-file copy path never truncates content.
7. Input manifest digests are stable and auditable.

## Incident-Specific Cleanup (Completed)

- Removed stale local scratch file:
  `/home/user/code/kilroy/.ai/spec.md`

This eliminates the immediate local solitaire source file, but the durable fix
is the architecture above.
