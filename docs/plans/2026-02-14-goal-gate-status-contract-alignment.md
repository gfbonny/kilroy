# Goal-Gate Status Contract Alignment Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Eliminate the control-flow mismatch where goal-gate nodes emit `outcome=pass` while Attractor exit gating only accepts `success|partial_success`, and add guardrails so future graphs cannot silently reintroduce the issue.

**Architecture:** Keep the engine contract unchanged (spec-canonical). Fix the source of mismatch in the english-to-dotfile reference artifacts, and add validator diagnostics that flag goal-gate-to-exit conditions using non-success statuses. This preserves strict runtime semantics while improving authoring ergonomics and pre-run safety.

**Tech Stack:** Go (`internal/attractor/validate`), DOT templates (`skills/english-to-dotfile/reference_template.dot`), markdown skill/docs (`skills/english-to-dotfile/SKILL.md`), Go tests (`go test`).

---

### Task 1: Lock in Existing Contract With a Focused Regression Test

**Files:**
- Modify: `internal/attractor/validate/validate_test.go`

**Step 1: Add a failing test case for goal_gate + exit edge condition mismatch**

Add a new test that builds a small DOT graph where:
- node `review_consensus` has `goal_gate=true`
- edge `review_consensus -> exit` uses `condition="outcome=pass"`

Assert validator emits a warning diagnostic for this pattern.

**Step 2: Run targeted test to verify failure before implementation**

Run:
```bash
go test ./internal/attractor/validate -run GoalGate -v
```

Expected: FAIL because the new rule does not exist yet.

**Step 3: Commit test scaffold**

Run:
```bash
git add internal/attractor/validate/validate_test.go
git commit -m "test(validate): add regression for goal-gate exit condition using non-success status"
```

---

### Task 2: Add Validator Rule for Goal-Gate Exit Status Compatibility

**Files:**
- Modify: `internal/attractor/validate/validate.go`
- Modify: `internal/attractor/validate/validate_test.go`

**Step 1: Implement lint rule in validator**

Add a new lint function (warning severity) that:
- finds nodes with `goal_gate=true`
- inspects outgoing edges from those nodes to terminal nodes (`Msquare`, `doublecircle`, or id `exit`/`end`)
- checks each such edge condition
- warns when condition compares `outcome` to values outside `success|partial_success` (for example `outcome=pass`)

Recommended rule id: `goal_gate_exit_status_contract`.

Recommended message:
- "goal_gate node routes to terminal on non-success outcome; use outcome=success (or partial_success) to satisfy goal-gate contract"

**Step 2: Register the rule in `Validate()`**

Wire the new lint into the existing pipeline after `lintGoalGateHasRetry`.

**Step 3: Add/expand tests**

In `validate_test.go`, cover:
- warning emitted for `outcome=pass`
- no warning for `outcome=success`
- no warning for `outcome=partial_success`
- no warning when goal-gate node does not directly route to terminal

**Step 4: Run validate package tests**

Run:
```bash
go test ./internal/attractor/validate -v
```

Expected: PASS.

**Step 5: Commit validator implementation**

Run:
```bash
git add internal/attractor/validate/validate.go internal/attractor/validate/validate_test.go
git commit -m "feat(validate): warn when goal-gate terminal routing uses non-success status conditions"
```

---

### Task 3: Fix Reference Template to Use Canonical Goal-Gate Success Status

**Files:**
- Modify: `skills/english-to-dotfile/reference_template.dot`

**Step 1: Update review prompts to canonical status vocabulary**

Change review-related status instructions from `outcome=pass` to `outcome=success` where the branch means "approved".

At minimum update:
- `review_a`, `review_b`, `review_c` prompt outcome instructions
- `review_consensus` prompt outcome instructions

**Step 2: Update consensus routing edge**

Change:
- `review_consensus -> exit [condition="outcome=pass"]`

to:
- `review_consensus -> exit [condition="outcome=success"]`

Retain retry/failure path semantics.

**Step 3: Validate the template graph**

Run one of:
```bash
./kilroy attractor validate --graph skills/english-to-dotfile/reference_template.dot
```
or
```bash
go run ./cmd/kilroy attractor validate --graph skills/english-to-dotfile/reference_template.dot
```

Expected: no errors; no warning from new goal-gate status rule.

**Step 4: Commit template fix**

Run:
```bash
git add skills/english-to-dotfile/reference_template.dot
git commit -m "fix(template): align goal-gate success routing with canonical outcome statuses"
```

---

### Task 4: Align Skill Guidance to Prevent Future Drift

**Files:**
- Modify: `skills/english-to-dotfile/SKILL.md`

**Step 1: Update routing guidance where it currently normalizes `pass`**

Adjust sections that imply `pass` is standard in the core loop. Make explicit:
- custom outcomes are allowed for steering nodes
- goal-gate satisfaction must use `outcome=success` or `outcome=partial_success`

**Step 2: Add explicit anti-pattern entry**

Add a concise anti-pattern similar to:
- "Do not use `outcome=pass` as success signal on `goal_gate=true` nodes that route to terminal; use canonical success statuses."

**Step 3: Keep examples consistent with updated template**

Ensure all review/consensus examples in skill prose reflect `success/retry/fail` status routing for goal-gate flow.

**Step 4: Commit skill-doc changes**

Run:
```bash
git add skills/english-to-dotfile/SKILL.md
git commit -m "docs(skill): codify canonical goal-gate success statuses and remove pass-based drift"
```

---

### Task 5: End-to-End Validation of Behavior and Tooling

**Files:**
- No new files required

**Step 1: Run targeted engine and validator tests**

Run:
```bash
go test ./internal/attractor/validate ./internal/attractor/engine -run GoalGate -v
```

Expected: PASS.

**Step 2: Run full repository test suite**

Run:
```bash
go test ./...
```

Expected: PASS.

**Step 3: Build CLI and validate template again**

Run:
```bash
go build -o ./kilroy ./cmd/kilroy
./kilroy attractor validate --graph skills/english-to-dotfile/reference_template.dot
```

Expected: successful build and clean validation.

**Step 4: Commit final verification note (if any test fixtures/log snapshots changed)**

Run:
```bash
git add -A
git commit -m "chore: run full validation for goal-gate status contract alignment"
```

Only commit if there are intentional tracked changes.

---

### Task 6: PR Assembly and Reviewer Checklist

**Files:**
- No source changes required

**Step 1: Produce concise change summary for PR description**

Include:
- root cause (goal_gate contract vs template `pass` vocabulary)
- why engine semantics were kept unchanged
- validator guardrail added
- template + skill/docs aligned

**Step 2: Include reviewer verification commands**

```bash
go test ./internal/attractor/validate -v
go test ./internal/attractor/engine -run GoalGate -v
./kilroy attractor validate --graph skills/english-to-dotfile/reference_template.dot
```

**Step 3: Ensure commit history is narrow and ordered**

Recommended commit order:
1. test regression
2. validator rule
3. template alignment
4. skill/docs alignment
5. verification/meta (if needed)

