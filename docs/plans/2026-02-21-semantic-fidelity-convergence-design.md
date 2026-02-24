# Semantic Fidelity Convergence Fix

**Date:** 2026-02-21
**Branch:** fix/semantic-fidelity-convergence

## Problem

Kilroy hill-climbing runs die at semantic review gates (verify_fidelity) even
when making real progress. Two root causes:

### Root Cause 1: Generic failure_signature blinds the cycle breaker

verify_fidelity produces a fixed `failure_reason` like `"semantic_fidelity_gap"`
regardless of how many criteria pass or fail. The cycle breaker signature is
`nodeID|failureClass|normalizedReason`, so it sees:

```
verify_fidelity|deterministic|semantic_fidelity_gap   (iteration 1, 6 gaps)
verify_fidelity|deterministic|semantic_fidelity_gap   (iteration 2, 3 gaps)
verify_fidelity|deterministic|semantic_fidelity_gap   (iteration 3, 1 gap)
→ KILLED (3x limit reached)
```

The engine already supports `meta.failure_signature` via
`readFailureSignatureHint()` (tested in
`TestRestartFailureSignature_UsesFailureSignatureHint`), which takes priority
over `failure_reason` when building the cycle key. But no prompt instructs
verify_fidelity to emit it.

### Root Cause 2: implement prompt doesn't distinguish repair vs fresh

The implement prompt says "Execute .ai/plan_final.md" (primary) and "Read
.ai/postmortem_latest.md if present" (secondary, optional). LLMs interpret this
as "build from the plan" rather than "surgically fix the gaps." Each iteration
produces another "complete but shallow" pass instead of targeted repairs.

## Solution

Prompt-only changes. No engine code modifications needed.

### Change 1: verify_fidelity emits content-addressable failure_signature

Add to the verify_fidelity prompt's status contract in reference_template.dot,
rogue.dot, and the english-to-dotfile skill:

```
If status=fail, set failure_signature in meta to a sorted comma-separated list
of failed criteria identifiers (e.g. "AC-6,AC-7,AC-13" or
"monster_stats,dungeon_gen,wizard_mode"). This lets the cycle breaker
distinguish "same failures" from "different/fewer failures" across iterations.
```

As gaps get fixed, the signature changes → counter doesn't accumulate → run
survives to keep iterating.

### Change 2: implement prompt is repair-aware

Restructure the implement prompt to check for postmortem first:

```
If .ai/postmortem_latest.md exists, this is a REPAIR iteration:
  1. Read .ai/postmortem_latest.md FIRST.
  2. Fix ONLY the gaps it identifies.
  3. Do NOT regenerate or rewrite systems already marked as working.
  4. Preserve all passing code and tests.
Otherwise, execute .ai/plan_final.md as a fresh implementation.
```

This aligns with the postmortem contract which already says "The next iteration
must NOT start from scratch — preserve working code and fix gaps."

### Change 3: english-to-dotfile skill documents the pattern

Update SKILL.md to add anti-patterns and guidance for:
- verify_fidelity must emit failure_signature with specific gap identifiers
- implement must condition on postmortem existence for repair-mode behavior

## Files to Change

1. `skills/english-to-dotfile/reference_template.dot`
   - verify_fidelity prompt: add failure_signature instruction
   - implement prompt: add repair-mode conditioning

2. `skills/english-to-dotfile/SKILL.md`
   - Add anti-pattern: "Generic failure_reason on semantic gates"
   - Add guidance on failure_signature for verify nodes

3. `demo/rogue/rogue.dot`
   - verify_fidelity prompt: add failure_signature instruction
   - implement prompt: add repair-mode conditioning

## Out of Scope

- Engine code changes (the mechanism already exists)
- DND graph (runtime-generated, will pick up template changes on next ingest)
- solitaire-fast.dot (check if it has verify_fidelity; update if so)

## Verification

- `go run ./cmd/kilroy attractor validate --graph demo/rogue/rogue.dot` passes
- `go run ./cmd/kilroy attractor validate --graph skills/english-to-dotfile/reference_template.dot` passes
- Existing engine tests pass (no engine changes)
- Manual review: verify_fidelity prompt includes failure_signature guidance
- Manual review: implement prompt conditions on postmortem existence
