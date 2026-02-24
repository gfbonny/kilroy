# Attractor Run Root Cause Fixes

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix three Kilroy-level root causes that caused the rogue and D&D attractor pipeline runs to fail.

**Architecture:** Three independent fixes: (1) harden the `loop_restart_failure_class_guard` validator lint rule to catch unconditional edges and enforce paired deterministic fallback edges per spec, (2) a `fix_fmt` auto-fix node in the reference template with `max_retries=0`, (3) a postmortem prompt clarification in the reference template. Each fix has a corresponding guardrail test.

**Tech Stack:** Go (validator), DOT (reference template), Go tests (guardrails)

---

### Task 1: Validator — harden `loop_restart_failure_class_guard` lint rule

The existing `lintLoopRestartFailureClassGuard` (validate.go:731-757) has two gaps relative to the spec (attractor-spec.md:1509):

1. **Unconditional edges missed:** The `conditionMentionsFailureOutcome` early-return skips unconditional edges (empty condition). An unconditional `loop_restart=true` edge is a de facto failure path — the engine's `loopRestart()` guard blocks it at runtime when the preceding outcome is a failure. This killed the rogue run.

2. **Paired-edge check missing:** The spec says a `loop_restart=true` edge "on a failure path should be guarded by context.failure_class=transient_infra **and paired with a non-restart deterministic fail edge**." The current rule checks the guard but not the pairing. Without a companion non-restart edge for deterministic failures, the node has no outgoing fail edge for non-transient failures.

**Files:**
- Modify: `internal/attractor/validate/validate.go:731-757`
- Test: `internal/attractor/validate/validate_test.go` (after line 231, and update existing test at line 210)

**Step 1: Write failing tests**

Add two tests in validate_test.go after the existing loop_restart tests (line 231):

a) `TestValidate_LoopRestartOnUnconditionalEdge_Warns` — unconditional edge with `loop_restart=true`:

```go
func TestValidate_LoopRestartOnUnconditionalEdge_Warns(t *testing.T) {
	g, err := dot.Parse([]byte(`
digraph G {
  start [shape=Mdiamond]
  exit [shape=Msquare]
  a [shape=box, llm_provider=openai, llm_model=gpt-5.2, prompt="x"]
  b [shape=box, llm_provider=openai, llm_model=gpt-5.2, prompt="y"]
  start -> a -> b
  b -> a [loop_restart=true]
  a -> exit [condition="outcome=success"]
}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	diags := Validate(g)
	assertHasRule(t, diags, "loop_restart_failure_class_guard", SeverityWarning)
}
```

b) `TestValidate_LoopRestartTransientGuardWithoutDeterministicFallback_Warns` — properly-guarded transient edge but no companion deterministic fail edge:

```go
func TestValidate_LoopRestartTransientGuardWithoutDeterministicFallback_Warns(t *testing.T) {
	g, err := dot.Parse([]byte(`
digraph G {
  start [shape=Mdiamond]
  exit [shape=Msquare]
  a [shape=box, llm_provider=openai, llm_model=gpt-5.2, prompt="x"]
  check [shape=diamond]
  start -> a -> check
  check -> a [condition="outcome=fail && context.failure_class=transient_infra", loop_restart=true]
  check -> exit [condition="outcome=success"]
}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	diags := Validate(g)
	assertHasRule(t, diags, "loop_restart_failure_class_guard", SeverityWarning)
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/attractor/validate/ -run "TestValidate_LoopRestartOnUnconditionalEdge_Warns|TestValidate_LoopRestartTransientGuardWithoutDeterministicFallback_Warns" -v`
Expected: both FAIL

**Step 3: Fix `lintLoopRestartFailureClassGuard`**

Replace the function. Two changes: (a) skip only edges exclusively conditioned on success (not unconditional ones), and (b) add a second pass that checks properly-guarded nodes have a companion non-restart deterministic fail edge:

```go
func lintLoopRestartFailureClassGuard(g *model.Graph) []Diagnostic {
	var diags []Diagnostic
	// Track nodes that have a properly-guarded transient restart edge.
	guardedRestartSources := map[string]bool{}
	for _, e := range g.Edges {
		if e == nil {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(e.Attr("loop_restart", "false")), "true") {
			continue
		}
		condExpr := strings.TrimSpace(e.Condition())
		// Skip edges exclusively conditioned on success (not a failure path).
		if condExpr != "" && !conditionMentionsFailureOutcome(condExpr) {
			continue
		}
		if conditionHasTransientInfraGuard(condExpr) {
			guardedRestartSources[e.From] = true
			continue
		}
		diags = append(diags, Diagnostic{
			Rule:     "loop_restart_failure_class_guard",
			Severity: SeverityWarning,
			Message:  "loop_restart=true requires condition guarded by context.failure_class=transient_infra",
			EdgeFrom: e.From,
			EdgeTo:   e.To,
			Fix:      "add condition with context.failure_class=transient_infra or remove loop_restart=true",
		})
	}
	// Second pass: nodes with a guarded transient restart must also have a
	// companion non-restart edge for deterministic failures.
	for from := range guardedRestartSources {
		hasDeterministicFallback := false
		for _, e := range g.Edges {
			if e == nil || e.From != from {
				continue
			}
			if strings.EqualFold(strings.TrimSpace(e.Attr("loop_restart", "false")), "true") {
				continue
			}
			if conditionMentionsFailureOutcome(strings.TrimSpace(e.Condition())) {
				hasDeterministicFallback = true
				break
			}
		}
		if !hasDeterministicFallback {
			diags = append(diags, Diagnostic{
				Rule:     "loop_restart_failure_class_guard",
				Severity: SeverityWarning,
				Message:  "node with transient-infra loop_restart must also have a non-restart edge for deterministic failures",
				EdgeFrom: from,
				Fix:      "add an edge for outcome=fail && context.failure_class!=transient_infra without loop_restart",
			})
		}
	}
	return diags
}
```

**Step 4: Update existing `_NoWarning` test**

The existing `TestValidate_LoopRestartFailureEdgeWithTransientInfraGuard_NoWarning` (line 210-231) has a properly-guarded transient edge but no companion deterministic fail edge. Add the companion edge so the test correctly exercises the no-warning path:

Replace the fixture at line 211-221:
```go
	g, err := dot.Parse([]byte(`
digraph G {
  start [shape=Mdiamond]
  exit [shape=Msquare]
  a [shape=box, llm_provider=openai, llm_model=gpt-5.2, prompt="x"]
  check [shape=diamond]
  pm [shape=box, llm_provider=openai, llm_model=gpt-5.2, prompt="postmortem"]
  start -> a -> check
  check -> a [condition="outcome=fail && context.failure_class=transient_infra", loop_restart=true]
  check -> pm [condition="outcome=fail && context.failure_class!=transient_infra"]
  check -> exit [condition="outcome=success"]
}
`))
```

**Step 5: Run tests to verify all pass**

Run: `go test ./internal/attractor/validate/ -run TestValidate_LoopRestart -v`
Expected: all 4 loop_restart tests pass (2 new + 2 existing)

**Step 6: Commit**

```bash
git add internal/attractor/validate/validate.go internal/attractor/validate/validate_test.go
git commit -m "validate: harden loop_restart_failure_class_guard per spec

Two gaps in the lint rule relative to attractor-spec.md line 1509:

1. Unconditional edges were skipped by the conditionMentionsFailureOutcome
   early-return. An unconditional loop_restart=true edge is a de facto failure
   path — the engine blocks it at runtime when the outcome is a failure.
   Fixed by skipping only edges explicitly conditioned on success.

2. The spec requires loop_restart=true edges be 'paired with a non-restart
   deterministic fail edge.' Added a second pass that checks nodes with a
   properly-guarded transient restart also have a companion non-restart edge
   for deterministic failures."
```

---

### Task 2: Reference template — add `fix_fmt` auto-fix node

The reference template routes formatting failures through the full postmortem -> re-plan -> re-implement cycle. Formatting is trivially auto-fixable. Add a `fix_fmt` tool node (placeholder command) between `check_implement` success and `verify_fmt`.

**Files:**
- Modify: `skills/english-to-dotfile/reference_template.dot`
- Test: `internal/attractor/validate/reference_template_guardrail_test.go`

**Step 1: Write failing guardrail test**

Add `TestReferenceTemplate_HasAutoFixBeforeVerifyFmt` in reference_template_guardrail_test.go. Assert that a `fix_fmt` node exists with `shape=parallelogram` and that the routing is `check_implement --[success]--> fix_fmt -> verify_fmt`.

```go
func TestReferenceTemplate_HasAutoFixBeforeVerifyFmt(t *testing.T) {
	g, err := dot.Parse(loadReferenceTemplate(t))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	fixFmt := g.Nodes["fix_fmt"]
	if fixFmt == nil {
		t.Fatal("missing fix_fmt node")
	}
	if fixFmt.Shape() != "parallelogram" {
		t.Fatalf("fix_fmt shape must be parallelogram, got %q", fixFmt.Shape())
	}

	hasCheckImplementToFixFmt := false
	hasFixFmtToVerifyFmt := false
	for _, e := range g.Edges {
		switch {
		case e.From == "check_implement" && e.To == "fix_fmt" && e.Condition() == "outcome=success":
			hasCheckImplementToFixFmt = true
		case e.From == "fix_fmt" && e.To == "verify_fmt":
			hasFixFmtToVerifyFmt = true
		}
	}
	if !hasCheckImplementToFixFmt || !hasFixFmtToVerifyFmt {
		t.Fatalf("missing fix_fmt routing: check_implement_to_fix_fmt=%v fix_fmt_to_verify_fmt=%v",
			hasCheckImplementToFixFmt, hasFixFmtToVerifyFmt)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/attractor/validate/ -run TestReferenceTemplate_HasAutoFixBeforeVerifyFmt -v`
Expected: FAIL (fix_fmt node doesn't exist yet)

**Step 3: Add `fix_fmt` node to reference template**

In `skills/english-to-dotfile/reference_template.dot`:

a) Add the `fix_fmt` node in `cluster_implement_verify`, between `check_implement` and `verify_fmt`:

```dot
        // Auto-fix formatting before verification. Replace placeholder with
        // the project's canonical formatter (e.g. gofmt -w ., cargo fmt,
        // black ., prettier --write .). Runs unconditionally so verify_fmt
        // is a pure assertion.
        fix_fmt [
            shape=parallelogram,
            max_retries=0,
            tool_command="echo 'Replace with project-specific auto-formatter (e.g. gofmt -w ., cargo fmt, black ., prettier --write .)'; exit 0"
        ]
```

b) Change the routing: `check_implement --[success]--> fix_fmt -> verify_fmt` (instead of `check_implement --[success]--> verify_fmt`).

Replace:
```dot
    check_implement -> verify_fmt [condition="outcome=success"]
```
With:
```dot
    check_implement -> fix_fmt [condition="outcome=success"]
    fix_fmt -> verify_fmt
```

**Step 4: Update existing guardrail tests**

a) The existing `TestReferenceTemplate_ImplementFailureRoutedBeforeVerify` (line 54-88) checks for `check_implement -> verify_fmt [condition="outcome=success"]`. Update it to check for `check_implement -> fix_fmt [condition="outcome=success"]` instead.

b) The existing `TestReferenceTemplate_DeterministicToolGatesHaveZeroRetries` (line 90-109) checks `verify_fmt` and `verify_artifacts`. Add `fix_fmt` to the `deterministicGates` slice — a formatter is deterministic (re-running on unchanged code produces the same result), so retries waste cycles.

**Step 5: Run tests to verify all pass**

Run: `go test ./internal/attractor/validate/ -run TestReferenceTemplate -v`
Expected: all reference template guardrail tests pass

**Step 6: Commit**

```bash
git add skills/english-to-dotfile/reference_template.dot internal/attractor/validate/reference_template_guardrail_test.go
git commit -m "reference_template: add fix_fmt auto-fix node before verify_fmt

Formatting failures are trivially auto-fixable but the template previously
routed them through the full postmortem -> re-plan -> re-implement cycle.
This burned hill-climbing iterations and accumulated cycle-breaker signatures
until the run died.

Added a fix_fmt tool node (placeholder command) between check_implement and
verify_fmt. The generator replaces the placeholder with the project's
canonical formatter (gofmt -w, cargo fmt, black, prettier, etc.)."
```

---

### Task 3: Reference template — clarify postmortem status contract

The postmortem prompt says "Analyze why the implementation failed" and "Set status to success." The LLM sometimes resolves this tension by reporting the analyzed failure as its own status. Clarify that `status` reflects whether the analysis was written, not the state of the implementation.

**Files:**
- Modify: `skills/english-to-dotfile/reference_template.dot` (postmortem prompt)
- Test: `internal/attractor/validate/reference_template_guardrail_test.go`

**Step 1: Write failing guardrail test**

Add `TestReferenceTemplate_PostmortemPromptClarifiesStatusContract` in reference_template_guardrail_test.go. Assert the postmortem prompt contains text that explicitly disambiguates task status from implementation status.

```go
func TestReferenceTemplate_PostmortemPromptClarifiesStatusContract(t *testing.T) {
	g, err := dot.Parse(loadReferenceTemplate(t))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	pm := g.Nodes["postmortem"]
	if pm == nil {
		t.Fatal("missing postmortem node")
	}
	prompt := pm.Attr("prompt", "")
	if !strings.Contains(prompt, "whether you completed the analysis") {
		t.Fatal("postmortem prompt must clarify that status reflects analysis completion, not implementation state")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/attractor/validate/ -run TestReferenceTemplate_PostmortemPromptClarifiesStatusContract -v`
Expected: FAIL (current prompt doesn't contain the clarifying text)

**Step 3: Update postmortem prompt**

In `skills/english-to-dotfile/reference_template.dot`, replace the postmortem's status line. Change:

```
Write canonical status JSON per the engine-provided \"Execution status contract\" preamble.\nSet `status` to `success`.
```

To:

```
Write canonical status JSON per the engine-provided \"Execution status contract\" preamble.\nYour status reflects whether you completed the analysis, not the state of the implementation.\nSet `status` to `success` when .ai/postmortem_latest.md is written.
```

**Step 4: Run tests to verify all pass**

Run: `go test ./internal/attractor/validate/ -run TestReferenceTemplate -v`
Expected: all reference template guardrail tests pass

**Step 5: Commit**

```bash
git add skills/english-to-dotfile/reference_template.dot internal/attractor/validate/reference_template_guardrail_test.go
git commit -m "reference_template: clarify postmortem status contract

The postmortem prompt said 'Analyze why the implementation failed' and
'Set status to success.' The LLM sometimes resolved this ambiguity by
reporting the analyzed failure as its own status, which then triggered
the loop_restart guard when the hill-climbing edge had loop_restart=true.

Clarified that status reflects whether the postmortem analysis was
completed, not the state of the implementation."
```

---

### Task 4: Update english-to-dotfile skill text

The skill's Phase 4 and Phase 2 sections should reference the new `fix_fmt` node so generated pipelines include it.

**Files:**
- Modify: `skills/english-to-dotfile/SKILL.md`

**Step 1: Add auto-fix instruction to Phase 2 (Select Topology)**

In the "Constraint fast path" section, after the verify/check pipeline description, add a note that auto-fix nodes precede verify nodes when the language has a canonical formatter.

**Step 2: Add auto-fix instruction to Phase 4 (Write Node Prompts)**

After the "Mandatory status contract text" paragraph, add: "For each deterministic verify gate that has a language-canonical auto-fix command (formatter, import sorter, linter with --fix), populate the preceding fix node's tool_command with that command."

**Step 3: Update the Reference Skeleton**

Update the skeleton in SKILL.md to include `fix_fmt` between `check_implement` and `verify_fmt`, matching the reference template.

**Step 4: Commit**

```bash
git add skills/english-to-dotfile/SKILL.md
git commit -m "english-to-dotfile: instruct generator to populate auto-fix nodes

Updated Phase 2, Phase 4, and the reference skeleton to reference the new
fix_fmt node. The generator must replace the placeholder tool_command with
the project's canonical formatter."
```

---

### Task 5: Run full validate test suite

**Step 1: Run all validate tests**

Run: `go test ./internal/attractor/validate/ -v`
Expected: all tests pass

**Step 2: Run `kilroy attractor validate` on the reference template**

Run: `go run ./cmd/kilroy attractor validate --graph skills/english-to-dotfile/reference_template.dot`
Expected: no errors, no warnings
