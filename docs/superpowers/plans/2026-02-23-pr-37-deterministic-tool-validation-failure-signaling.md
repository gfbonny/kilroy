# PR #37 Deterministic Tool Validation Failure Signaling Implementation Plan

> **For Claude:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make repeated malformed tool-call loops fail with explicit deterministic semantics so Attractor retry/routing follows `failure_class` architecture contracts from the first implementation.

**Architecture:** Keep tool argument validation at the tool registry boundary, but move loop-breaker responsibility to the session loop (where repeated failures across rounds are visible). Propagate deterministic meaning as a typed `llm.InvalidToolCallError`, and ensure provider failover refuses all non-retryable `llm.Error` values so deterministic local faults never fan out into provider failover. Preserve the Anthropic tool-use mismatch retryability behavior as transient infrastructure.

**Tech Stack:** Go (`internal/agent`, `internal/llm`, `internal/attractor/engine`), table-driven/unit tests, `go test`, `go build`.

**Related Skills:** @executing-plans, @fresheyes

**Primary Spec:** `docs/strongdm/attractor/attractor-spec.md` (failure classification and retry gating sections around `failure_class`, retry policy, and loop-restart guards)

---

## Chunk 1: Deterministic Malformed Tool-Call Failure Path

### Task 0: Preflight and Branch Baseline

**Files:**
- No file edits

- [ ] **Step 1: Move to PR #37 branch baseline in a dedicated worktree**

Run:
- `git worktree list`
- `if [ ! -d .worktrees/pr-review ]; then git worktree add .worktrees/pr-review --detach HEAD; fi`
- `cd .worktrees/pr-review`
- `git fetch origin`
- `git checkout validation-circuit-breaker`
- `git status -sb`

Expected:
- Branch is `validation-circuit-breaker`.
- Worktree is clean or contains only intended PR #37 changes.

- [ ] **Step 2: Verify PR #37 baseline behaviors before editing**

Run:
- `rg -n 'CIRCUIT BREAKER|recordValidationFailure|validationFailures' internal/agent/tool_registry.go`
- `go test ./internal/llm -list '^TestErrorFromHTTPStatus_ToolUseMismatch_IsRetryable$'`

Expected:
- First command shows circuit-breaker code currently in the PR branch.
- Second command output determines whether the llm retryability test already exists. If absent, Task 5 Step 1 adds it.
- If the first command returns no matches, skip Task 2 (already stateless) and continue to Task 3.

### File Structure / Responsibilities

- Modify: `internal/agent/tool_registry.go`
- Responsibility: return structured tool execution errors for validation failures (`invalid JSON`, `schema validation`) without stateful per-tool circuit-breaker counters.

- Modify: `internal/agent/tool_registry_test.go`
- Responsibility: verify validation failures are semantically typed and stateless across calls.

- Modify: `internal/agent/session.go`
- Responsibility: detect repeated malformed tool-call patterns from structured result kinds, then abort with `llm.InvalidToolCallError` (deterministic, non-retryable).

- Create: `internal/agent/session_malformed_loop_test.go`
- Responsibility: lock regressions for malformed JSON loops and schema-validation loops, and assert typed deterministic error behavior.

- Modify: `internal/attractor/engine/codergen_router.go`
- Responsibility: prevent provider failover when an error is a non-retryable unified `llm.Error`.

- Modify: `internal/attractor/engine/codergen_failover_test.go`
- Responsibility: assert `llm.InvalidToolCallError` does not trigger failover.

- Verify (existing): `internal/llm/errors.go`, `internal/llm/errors_test.go`
- Responsibility: preserve existing PR #37 behavior where Anthropic `tool_use`/`tool_result` mismatch is retryable.

- Modify/Test: `internal/attractor/engine/retry_failure_class_test.go`
- Responsibility: verify execute-node classification and status mapping (`transient` -> `StatusRetry`, deterministic malformed-tool -> `StatusFail`) before retry policy application.

### Task 1: Add Structured Tool Validation Error Kinds

**Files:**
- Modify: `internal/agent/tool_registry.go`
- Test: `internal/agent/tool_registry_test.go`

- [ ] **Step 1: Write failing test for structured validation error kinds**

```go
func TestToolRegistry_ErrorKind_ForValidationFailures(t *testing.T) {
	r := NewToolRegistry()
	_ = r.Register(RegisteredTool{
		Definition: llm.ToolDefinition{
			Name: "t",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{"path": map[string]any{"type": "string"}},
				"required": []string{"path"},
			},
		},
		Exec: func(context.Context, ExecutionEnvironment, map[string]any) (any, error) { return "ok", nil },
	})
	env := NewLocalExecutionEnvironment(t.TempDir())

	badJSON := r.ExecuteCall(context.Background(), env, llm.ToolCallData{Name: "t", Arguments: json.RawMessage(`{"path":`)})
	if badJSON.ErrorKind != ToolErrorKindInvalidArgumentsJSON {
		t.Fatalf("badJSON.ErrorKind=%q", badJSON.ErrorKind)
	}

	badSchema := r.ExecuteCall(context.Background(), env, llm.ToolCallData{Name: "t", Arguments: json.RawMessage(`{}`)})
	if badSchema.ErrorKind != ToolErrorKindSchemaValidation {
		t.Fatalf("badSchema.ErrorKind=%q", badSchema.ErrorKind)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/agent -run TestToolRegistry_ErrorKind_ForValidationFailures -count=1`
Expected: FAIL (missing `ErrorKind` symbols/fields).

- [ ] **Step 3: Implement minimal structured error-kind support**

```go
type ToolErrorKind string

const (
	ToolErrorKindNone                 ToolErrorKind = ""
	ToolErrorKindInvalidArgumentsJSON ToolErrorKind = "invalid_arguments_json"
	ToolErrorKindSchemaValidation     ToolErrorKind = "schema_validation"
)

type ToolExecResult struct {
	ToolName   string
	CallID     string
	Output     string
	FullOutput string
	IsError    bool
	ErrorKind  ToolErrorKind
}
```

Implementation notes:
- Set `ErrorKind` on invalid JSON and schema-validation failures.
- Leave `ErrorKind` empty for success, unknown-tool, and executor runtime failures.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/agent -run TestToolRegistry_ErrorKind_ForValidationFailures -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/agent/tool_registry.go internal/agent/tool_registry_test.go
git commit -m "agent: add structured tool validation error kinds"
```

### Task 2: Remove Stateful Message-Only Circuit Breaker from Tool Registry

**Files:**
- Modify: `internal/agent/tool_registry.go`
- Modify/Test: `internal/agent/tool_registry_test.go`

- [ ] **Step 1: Write failing test for stateless validation messaging**

```go
func TestToolRegistry_ValidationErrorsAreStatelessAcrossCalls(t *testing.T) {
	r := NewToolRegistry()
	_ = r.Register(RegisteredTool{
		Definition: llm.ToolDefinition{
			Name: "t",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{"path": map[string]any{"type": "string"}},
				"required": []string{"path"},
			},
		},
		Exec: func(context.Context, ExecutionEnvironment, map[string]any) (any, error) { return "ok", nil },
	})
	env := NewLocalExecutionEnvironment(t.TempDir())

	first := r.ExecuteCall(context.Background(), env, llm.ToolCallData{Name: "t", Arguments: json.RawMessage(`{}`)})
	third := r.ExecuteCall(context.Background(), env, llm.ToolCallData{Name: "t", Arguments: json.RawMessage(`{}`)})
	if strings.Contains(first.Output, "CIRCUIT BREAKER") || strings.Contains(third.Output, "CIRCUIT BREAKER") {
		t.Fatalf("unexpected stateful breaker text in registry output")
	}
}
```

- [ ] **Step 2: Run test to verify it fails on current PR branch**

Run: `go test ./internal/agent -run TestToolRegistry_ValidationErrorsAreStatelessAcrossCalls -count=1`
Expected: FAIL (current branch emits `CIRCUIT BREAKER` after repeated failures).

- [ ] **Step 3: Implement stateless registry behavior and adjust tests**

Implementation checklist:
- Remove `validationFailures` map from `ToolRegistry`.
- Remove `circuitBreakerThreshold`, `recordValidationFailure`, and `resetValidationFailures`.
- Keep validation errors direct and actionable (schema error text can include required fields without cross-call counters).
- Replace `TestToolRegistry_CircuitBreaker_EscalatesAfterConsecutiveFailures` with stateless semantics tests.

- [ ] **Step 4: Run focused tool-registry tests**

Run: `go test ./internal/agent -run 'TestToolRegistry_(ErrorKind_ForValidationFailures|ValidationErrorsAreStatelessAcrossCalls)' -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/agent/tool_registry.go internal/agent/tool_registry_test.go
git commit -m "agent: make tool validation errors stateless in registry"
```

### Task 3: Trip Session Guard on Repeated Malformed Calls and Return Typed Deterministic Error

**Files:**
- Modify: `internal/agent/session.go`
- Create/Test: `internal/agent/session_malformed_loop_test.go`

- [ ] **Step 1: Add failing tests for typed deterministic guard behavior**

Test A (malformed JSON loop):
```go
func TestSession_RepeatedMalformedJSONLoop_ReturnsInvalidToolCallError(t *testing.T) {
	dir := t.TempDir()
	c := llm.NewClient()

	f := &fakeAdapter{
		name: "openai",
		steps: []func(req llm.Request) llm.Response{
			func(req llm.Request) llm.Response {
				return llm.Response{
					Message: llm.Message{
						Role: llm.RoleAssistant,
						Content: []llm.ContentPart{{
							Kind: llm.ContentToolCall,
							ToolCall: &llm.ToolCallData{
								ID:        "glob:1",
								Name:      "glob",
								Arguments: json.RawMessage(`{\"pattern\":\"*.c\"}{\"path\":\".\"}`),
							},
						}},
					},
				}
			},
			func(req llm.Request) llm.Response {
				return llm.Response{
					Message: llm.Message{
						Role: llm.RoleAssistant,
						Content: []llm.ContentPart{{
							Kind: llm.ContentToolCall,
							ToolCall: &llm.ToolCallData{
								ID:        "glob:2",
								Name:      "glob",
								Arguments: json.RawMessage(`{\"pattern\":\"*.c\"}{\"path\":\".\"}`),
							},
						}},
					},
				}
			},
			func(req llm.Request) llm.Response {
				t.Fatalf("unexpected third request after malformed-loop guard")
				return llm.Response{Message: llm.Assistant("unreachable")}
			},
		},
	}
	c.Register(f)

	sess, err := NewSession(c, NewOpenAIProfile("gpt-5.2"), NewLocalExecutionEnvironment(dir), SessionConfig{
		MaxToolRoundsPerInput:          50,
		MaxTurns:                       50,
		RepeatedMalformedToolCallLimit: 2,
	})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = sess.ProcessInput(ctx, "trigger malformed json loop")
	if err == nil {
		t.Fatal("expected error")
	}
	var inv *llm.InvalidToolCallError
	if !errors.As(err, &inv) {
		t.Fatalf("expected llm.InvalidToolCallError, got %T (%v)", err, err)
	}
}
```

Test B (new schema-loop test):
```go
func TestSession_RepeatedSchemaValidationLoop_ReturnsInvalidToolCallError(t *testing.T) {
	dir := t.TempDir()
	c := llm.NewClient()

	f := &fakeAdapter{
		name: "openai",
		steps: []func(req llm.Request) llm.Response{
			func(req llm.Request) llm.Response {
				return llm.Response{
					Message: llm.Message{
						Role: llm.RoleAssistant,
						Content: []llm.ContentPart{{
							Kind: llm.ContentToolCall,
							ToolCall: &llm.ToolCallData{
								ID:        "glob:1",
								Name:      "glob",
								Arguments: json.RawMessage(`{}`), // missing required "pattern"
							},
						}},
					},
				}
			},
			func(req llm.Request) llm.Response {
				return llm.Response{
					Message: llm.Message{
						Role: llm.RoleAssistant,
						Content: []llm.ContentPart{{
							Kind: llm.ContentToolCall,
							ToolCall: &llm.ToolCallData{
								ID:        "glob:2",
								Name:      "glob",
								Arguments: json.RawMessage(`{}`), // repeat same malformed schema args
							},
						}},
					},
				}
			},
			func(req llm.Request) llm.Response {
				t.Fatalf("unexpected third request after malformed-loop guard")
				return llm.Response{Message: llm.Assistant("unreachable")}
			},
		},
	}
	c.Register(f)

	sess, err := NewSession(c, NewOpenAIProfile("gpt-5.2"), NewLocalExecutionEnvironment(dir), SessionConfig{
		MaxToolRoundsPerInput:          50,
		MaxTurns:                       50,
		RepeatedMalformedToolCallLimit: 2,
	})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = sess.ProcessInput(ctx, "trigger repeated schema-validation loop")
	if err == nil {
		t.Fatal("expected error")
	}
	var inv *llm.InvalidToolCallError
	if !errors.As(err, &inv) {
		t.Fatalf("expected llm.InvalidToolCallError, got %T (%v)", err, err)
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/agent -run 'TestSession_(RepeatedMalformedJSONLoop_ReturnsInvalidToolCallError|RepeatedSchemaValidationLoop_ReturnsInvalidToolCallError)' -count=1`
Expected: FAIL (current code returns `fmt.Errorf(...)` and only tracks invalid JSON message text).

- [ ] **Step 3: Implement session guard using structured error kinds and typed error return**

```go
func malformedToolCallsFingerprint(calls []llm.ToolCallData, results []ToolExecResult) string {
	if len(calls) == 0 || len(calls) != len(results) {
		return ""
	}
	var b strings.Builder
	for i := range calls {
		if !results[i].IsError {
			continue
		}
		switch results[i].ErrorKind {
		case ToolErrorKindInvalidArgumentsJSON, ToolErrorKindSchemaValidation:
			// include tool + args hash so repeated malformed patterns trip deterministically
		default:
			continue
		}
		b.WriteString(strings.TrimSpace(calls[i].Name))
		b.WriteByte(':')
		b.WriteString(shortHash(calls[i].Arguments))
		b.WriteByte(';')
	}
	return b.String()
}
```

When threshold is reached:
```go
err := llm.NewInvalidToolCallError(
	fmt.Sprintf("repeated malformed tool calls detected (repeats=%d limit=%d)", malformedRepeats, s.cfg.RepeatedMalformedToolCallLimit),
)
```

- [ ] **Step 4: Run tests to verify pass**

Run: `go test ./internal/agent -run 'TestSession_(RepeatedMalformedJSONLoop_ReturnsInvalidToolCallError|RepeatedSchemaValidationLoop_ReturnsInvalidToolCallError)' -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/agent/session.go internal/agent/session_malformed_loop_test.go
git commit -m "agent/session: return InvalidToolCallError for repeated malformed calls"
```

### Task 4: Block Provider Failover for Non-Retryable Unified LLM Errors

**Files:**
- Modify: `internal/attractor/engine/codergen_router.go`
- Test: `internal/attractor/engine/codergen_failover_test.go`

- [ ] **Step 1: Add failing failover test for invalid tool-call error**

```go
func TestShouldFailoverLLMError_InvalidToolCallDoesNotFailover(t *testing.T) {
	err := llm.NewInvalidToolCallError("repeated malformed tool calls")
	if shouldFailoverLLMError(err) {
		t.Fatalf("InvalidToolCallError should not trigger failover")
	}
}

func TestShouldFailoverLLMError_AbortDoesNotFailover(t *testing.T) {
	err := llm.NewAbortError("user canceled")
	if shouldFailoverLLMError(err) {
		t.Fatalf("AbortError should not trigger failover")
	}
}

func TestShouldFailoverLLMError_RateLimitDoesFailover(t *testing.T) {
	err := llm.ErrorFromHTTPStatus("openai", 429, "rate limited", nil, nil)
	if !shouldFailoverLLMError(err) {
		t.Fatalf("RateLimitError should trigger failover")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/attractor/engine -run 'TestShouldFailoverLLMError_(InvalidToolCallDoesNotFailover|AbortDoesNotFailover)' -count=1`
Expected: FAIL (at least `InvalidToolCallDoesNotFailover` should fail on the current branch baseline).

- [ ] **Step 3: Implement deterministic non-retryable failover guard with quota exception**

```go
var qe *llm.QuotaExceededError
if errors.As(err, &qe) {
	return true // preserve budget/quota failover behavior
}
var le llm.Error
if errors.As(err, &le) && !le.Retryable() {
	return false
}
```

Implementation note:
- Keep existing explicit guards (`NotFound`, `ContentFilter`, `ErrTurnLimit`, bootstrap errors). This rule makes non-retryable errors deterministic by default while preserving quota/budget failover semantics.

- [ ] **Step 4: Run focused failover tests**

Run: `go test ./internal/attractor/engine -run 'TestShouldFailoverLLMError_(NotFoundDoesNotFailover|ContentFilterDoesNotFailover|QuotaExceededDoesFailover|RateLimitDoesFailover|TurnLimitDoesNotFailover|InvalidToolCallDoesNotFailover|AbortDoesNotFailover)' -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/attractor/engine/codergen_router.go internal/attractor/engine/codergen_failover_test.go
git commit -m "engine/failover: respect llm non-retryable errors"
```

### Task 5: Verify Anthropic Tool-Use Mismatch Retryability and End-to-End Classification

**Files:**
- Modify: `internal/llm/errors.go`
- Modify/Test: `internal/llm/errors_test.go`
- Test: `internal/attractor/engine/provider_error_classification_test.go`
- Test: `internal/attractor/engine/retry_failure_class_test.go`

- [ ] **Step 1: Add failing llm retryability test for Anthropic tool-use mismatch**

```go
func TestErrorFromHTTPStatus_ToolUseMismatch_IsRetryable(t *testing.T) {
	err := ErrorFromHTTPStatus("anthropic", 400, "tool_use ids were found without tool_result blocks immediately after", nil, nil)
	e, ok := err.(Error)
	if !ok {
		t.Fatalf("not an llm.Error: %T", err)
	}
	if !e.Retryable() {
		t.Fatalf("expected retryable, got non-retryable")
	}
	if _, ok := err.(*ServerError); !ok {
		t.Fatalf("expected *ServerError, got %T", err)
	}
}
```

- [ ] **Step 2: Run llm test and follow explicit branch rule**

Run: `go test ./internal/llm -run '^TestErrorFromHTTPStatus_ToolUseMismatch_IsRetryable$' -count=1`
Expected decision rule:
- If the test FAILS, continue to Step 3 and implement classification.
- If the test PASSES, skip Step 3 (classification already present) and continue to Step 4.
- If output says no matching tests, return to Step 1 and ensure the test was added correctly.

- [ ] **Step 3: Implement llm classification for this Anthropic 400 pattern (only if Step 2 failed)**

```go
case strings.EqualFold(strings.TrimSpace(base.provider), "anthropic") &&
	strings.Contains(lower, "tool_use ids were found without tool_result"):
	base.retryable = true
	return &ServerError{base}
```

- [ ] **Step 4: Add explicit engine-level classification regression tests**

```go
func TestClassifyAPIError_AnthropicToolUseMismatch_IsTransientInfra(t *testing.T) {
	err := llm.ErrorFromHTTPStatus("anthropic", 400, "tool_use ids were found without tool_result blocks", nil, nil)
	cls, _ := classifyAPIError(err)
	if cls != failureClassTransientInfra {
		t.Fatalf("class=%q want %q", cls, failureClassTransientInfra)
	}
}

func TestClassifyAPIError_InvalidToolCallError_IsDeterministic(t *testing.T) {
	err := llm.NewInvalidToolCallError("repeated malformed tool calls")
	cls, _ := classifyAPIError(err)
	if cls != failureClassDeterministic {
		t.Fatalf("class=%q want %q", cls, failureClassDeterministic)
	}
}
```

- [ ] **Step 5: Extend execute-node API error classification test with status assertions**

```go
func TestExecuteNode_APIError_SetsFailureClass(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus runtime.Status
		wantClass  string
	}{
		{
			name:       "anthropic mismatch retry",
			err:        llm.ErrorFromHTTPStatus("anthropic", 400, "tool_use ids were found without tool_result blocks", nil, nil),
			wantStatus: runtime.StatusRetry,
			wantClass:  failureClassTransientInfra,
		},
		{
			name:       "invalid tool call deterministic fail",
			err:        llm.NewInvalidToolCallError("repeated malformed tool calls"),
			wantStatus: runtime.StatusFail,
			wantClass:  failureClassDeterministic,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Existing fixture setup from this test stays unchanged.
			out, _ := eng.executeNode(context.Background(), node)
			if out.Status != tc.wantStatus {
				t.Fatalf("status: got %q want %q", out.Status, tc.wantStatus)
			}
			hint := readFailureClassHint(out)
			if got := normalizedFailureClass(hint); got != tc.wantClass {
				t.Fatalf("failure_class: got %q want %q (raw hint=%q)", got, tc.wantClass, hint)
			}
		})
	}
}
```

- [ ] **Step 6: Run targeted llm + engine classification/status tests**

Run:
- `go test ./internal/llm -run '^TestErrorFromHTTPStatus_ToolUseMismatch_IsRetryable$' -count=1`
- `go test ./internal/attractor/engine -run 'TestClassifyAPIError_(AnthropicToolUseMismatch_IsTransientInfra|InvalidToolCallError_IsDeterministic)' -count=1`
- `go test ./internal/attractor/engine -run '^TestExecuteNode_APIError_SetsFailureClass$' -count=1`

Expected: all test invocations PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/llm/errors.go internal/llm/errors_test.go internal/attractor/engine/provider_error_classification_test.go internal/attractor/engine/retry_failure_class_test.go
git commit -m "engine: lock deterministic/transient classification invariants"
```

### Task 6: Full Validation and PR Notes

**Files:**
- No required file edits

- [ ] **Step 1: Run package-level suites touched by this work**

Run:
- `go test ./internal/agent`
- `go test ./internal/attractor/engine`
- `go test ./internal/llm`

Expected: PASS.

- [ ] **Step 2: Run full repository tests and build**

Run:
- `go test ./...`
- `go build -o ./kilroy ./cmd/kilroy`

Expected: PASS.

- [ ] **Step 3: Run gofmt on touched Go files**

Run:
- `gofmt -w internal/agent/tool_registry.go internal/agent/tool_registry_test.go internal/agent/session.go internal/agent/session_malformed_loop_test.go internal/attractor/engine/codergen_router.go internal/attractor/engine/codergen_failover_test.go internal/attractor/engine/retry_failure_class_test.go internal/llm/errors.go internal/llm/errors_test.go internal/attractor/engine/provider_error_classification_test.go`
- `go test ./internal/agent ./internal/attractor/engine ./internal/llm`
- `go test ./...`

Expected: no output, files normalized.
Expected: all post-format test commands PASS.

- [ ] **Step 4: Commit final polish if code changed during verification**

```bash
git add -A
git commit -m "test: finalize deterministic malformed-tool loop behavior"
```

### Task 7: PR Verification Summary and Reviewer Hand-off

**Files:**
- No code changes required

- [ ] **Step 1: Capture verification transcript in PR comment draft**

Include:
- Which new tests were added
- Which existing regressions were updated
- Why deterministic signaling now follows Attractor `failure_class` architecture

- [ ] **Step 2: Run fresheyes review against final diff**

Run in Codex (exact prompt):
`Use the $fresheyes skill to review the changes between origin/main and this branch, focusing on failure classification semantics, failover behavior, and test coverage completeness.`

Expected: no blocking findings.

- [ ] **Step 3: Commit if fresheyes requires fixes, then re-run affected tests**

```bash
go test ./internal/agent ./internal/attractor/engine ./internal/llm
go test ./...
```

Expected: PASS.

## Chunk 2: PR Landing Workflow

### Task 8: Gate 1 and Gate 2 Alignment During Execution

**Files:**
- No code changes required

- [ ] **Step 1: Keep Gate 1 and Gate 2 checkpoints explicit in the execution log**

Record in review notes:
- Gate 1 definition: explicit user approval of the PR assessment/recommendation (for example “approve”, “proceed”, “merge it”).
- Gate 1 recording: PR review thread checkpoint note in this session.
- Gate 2 definition: explicit user approval after build/test/fresheyes results are presented.
- Gate 2 recording: PR review thread checkpoint note in this session.
- gate log template:
- `GATE <1|2> | timestamp=<UTC ISO8601> | user_quote=\"<exact approval phrase>\" | scope=\"<assessment|post-results>\"`

Expected: no merge/push actions happen before Gate 2 approval.

- [ ] **Step 2: Re-run implementation deltas against `origin/main...HEAD` before Gate 2**

Run:
- `git fetch origin`
- `git diff --name-status origin/main...HEAD`
- `git diff --stat origin/main...HEAD`

Expected: changed files match intended scope from Chunk 1.

- [ ] **Step 3: Capture build/test/fresheyes evidence bundle**

Run:
- `go build -o ./kilroy ./cmd/kilroy`
- `go test ./internal/agent ./internal/attractor/engine ./internal/llm`
- fresheyes prompt:
`Use the $fresheyes skill to review the changes between origin/main and this branch, focusing on deterministic/transient failure-class handling, failover policy, and regression coverage.`

Expected: build/test pass and no blocking fresheyes findings.

### Task 9: Push and Merge Discipline

**Files:**
- No code changes required

- [ ] **Step 1: Enforce preconditions before any push/merge**

Preconditions:
- Task 8 Step 3 evidence collection is complete and green.
- Chunk 3 Task 10 is complete and green.
- Gate 2 approval is explicitly recorded.

Expected: no push/merge commands run unless both preconditions are true.

- [ ] **Step 2: Show final diff summary before any push**

Run:
- `git log --oneline --decorate -n 15`
- `git diff --stat origin/main...HEAD`

Expected: concise, auditable commit set aligned to plan tasks.
Expected: if unexpected files appear, stop and resolve scope drift before continuing.

- [ ] **Step 3: Push only after explicit user approval**

Run:
- `git push origin validation-circuit-breaker`

Expected: remote branch updated with reviewed commits only.

- [ ] **Step 4: Merge only after explicit user merge approval**

Pre-check:
- `gh pr view 37 --json mergeable,mergeStateStatus,url`

Preferred:
- `gh pr merge 37 --merge`

Fallback only if GitHub merge state is stale:
- `git status --short`
- `gh pr view 37 --json mergeable,mergeStateStatus,state,url`
- decision rules:
- if `mergeable` is conflicting/blocked (or `mergeStateStatus` indicates conflict/policy block), stop and ask user; do not run local fallback merge.
- if `mergeable` is mergeable and state is clean, prefer `gh pr merge 37 --merge`.
- use local fallback only for stale/inconsistent GitHub merge state after user re-approval.
- obtain renewed explicit user approval before any direct push fallback
- `git fetch origin`
- `git checkout main`
- `git merge --ff-only origin/main`
- `git diff --stat origin/main...origin/validation-circuit-breaker`
- `git merge --no-ff origin/validation-circuit-breaker -m "Merge pull request #37 from thewoolleyman/validation-circuit-breaker"`
- `git push origin main`

Expected: PR closed with merged result on `main`.

- [ ] **Step 5: Post-merge PR comment and merge-state verification**

Run:
- `gh pr view 37 --json state,mergedAt,url`
- if state remains OPEN after fallback merge path, retry `gh pr merge 37 --merge`; if still not merged, stop and ask user instead of force-closing.
- if merged state is confirmed, run:
- `gh pr comment 37 --body "Great contribution. This PR improves tool-call loop resilience and failure-class correctness.\n\nWhat was strong:\n- Clear focus on malformed tool-call behavior and retry classification\n- Regression tests added around failover and classification paths\n\nWhat we tightened before merge:\n- deterministic signaling for repeated malformed tool calls\n- failover guard behavior for non-retryable llm errors with quota exception\n- provider-scoped transient classification for Anthropic tool-use mismatch\n\n— Codex CLI"`

Expected: contributor-facing summary posted and PR state closed.

## Chunk 3: Completion Criteria and Recovery Paths

**Sequencing rule:** Complete Chunk 3 (Task 10 and any needed Task 11 recovery) before executing Chunk 2 merge steps.

### Task 10: Definition of Done Validation

**Files:**
- No code changes required

- [ ] **Step 1: Confirm architecture outcomes**

Checklist:
- Repeated malformed tool calls surface `llm.InvalidToolCallError`.
- Engine classification resolves malformed-tool loop failures as deterministic.
- Anthropic tool-use mismatch remains transient/retryable as a provider-scoped exception to default 400 deterministic behavior.
- Failover blocks deterministic non-retryable llm errors (quota exception preserved).

Expected: all four conditions satisfied by tests and code paths in Chunk 1.

- [ ] **Step 2: Confirm test coverage outcomes**

Run:
- `go test ./internal/agent -run 'TestToolRegistry_|TestSession_' -count=1`
- `go test ./internal/attractor/engine -run 'TestShouldFailoverLLMError_|TestExecuteNode_APIError_SetsFailureClass|TestClassifyAPIError_' -count=1`
- `go test ./internal/llm -list '^TestErrorFromHTTPStatus_ToolUseMismatch_IsRetryable$'`
- `go test ./internal/llm -run '^TestErrorFromHTTPStatus_ToolUseMismatch_IsRetryable$' -count=1`

Expected: PASS for all targeted regression suites.
Expected: `-list` output must include `TestErrorFromHTTPStatus_ToolUseMismatch_IsRetryable` before the `-run` command is accepted.

- [ ] **Step 3: Confirm clean final state**

Run:
- `go test ./...`
- `go build -o ./kilroy ./cmd/kilroy`
- `git status --short`

Expected: tests/build pass and working tree is clean before merge flow.

### Task 11: Recovery Plan If a Final Check Fails

**Files:**
- Modify (if needed): files implicated by failing test output

- [ ] **Step 1: Classify failure cause quickly**

Decision table:
- Test-only regression: patch tests for true contract drift only after code decision is approved.
- Behavior regression: patch code and keep existing/expanded test assertions.
- Architecture mismatch: fix classification semantics first, then update dependent tests.

Expected: single root cause selected before editing.

- [ ] **Step 2: Apply minimal fix and rerun smallest relevant suite**

Run pattern:
- edit affected files
- rerun exact failing package from prior output, for example:
- `go test ./internal/agent -count=1`
- `go test ./internal/attractor/engine -count=1`
- `go test ./internal/llm -count=1`
- run `gofmt -w` on any touched Go files
- commit the recovery fix before broad rerun:
- `git add -A`
- `git commit -m "fix: resolve recovery validation failure"`

Expected: targeted suite returns PASS before broad rerun.

- [ ] **Step 3: Re-run full validation**

Run:
- `go test ./...`
- `go build -o ./kilroy ./cmd/kilroy`
- `git status --short`

Expected: full green before returning to merge flow.
