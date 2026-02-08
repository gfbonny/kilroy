# Architectural Reliability Completion Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Eliminate the remaining reliability gaps where deterministic failures still consume retry budget and provider CLI contract mismatches are discovered too late.

**Architecture:** Keep the existing run/resume/loop-restart foundation, but extend reliability contracts to stage retry and provider adapter boundaries. Deterministic failures must be labeled explicitly at the source (`codergen`, fan-in) and must short-circuit retries. Add deterministic provider preflight with a persisted `preflight_report.json` so provider contract/model issues fail fast before long-running graph execution.

**Tech Stack:** Go 1.22, stdlib `testing`, existing Attractor engine integration tests, existing CXDB test harness, local CLI wrappers in tests.

---

## Baseline (Validated 2026-02-08)

The following is already green in this branch and is the baseline for this completion plan:

1. `go test ./internal/attractor/runtime -count=1`
2. `go test ./internal/attractor/engine -count=1`
3. `go test ./cmd/kilroy -count=1`
4. `go test ./internal/llm/providers/... -count=1`
5. `bash scripts/e2e-guardrail-matrix.sh`

Real-run artifact evidence shows remaining issues despite the green baseline:

1. Deterministic provider/implementation failures still consume full node retry budget before failing (`progress.ndjson` in `/tmp/kilroy-dttf-real-cxdb-20260208T171236Z-postfix/logs`).
2. Anthropic CLI contract mismatch (`--output-format=stream-json` with `--print` requiring `--verbose`) is discovered during branch execution instead of preflight (`stderr.log` under `.../01-impl_tracer_a/...`).
3. Gemini model-not-found is discovered at runtime (`ModelNotFoundError`) rather than preflight (`stderr.log` under `.../02-impl_tracer_b/...`).

---

### Task 1: Add Failing Tests For Failure-Class Retry Gating

**Files:**
- Create: `internal/attractor/engine/retry_failure_class_test.go`
- Reuse: `internal/attractor/engine/engine_test.go` helpers (`runCmd`, `assertExists`)
- Test: `internal/attractor/engine/retry_failure_class_test.go`

**Step 1: Write failing deterministic-short-circuit test**

```go
func TestRun_DeterministicFailure_DoesNotRetry(t *testing.T) {
    // handler returns StatusFail + Meta.failure_class=deterministic
    // graph default_max_retry=3
    // expect exactly one attempt and zero stage_retry_sleep events for node
}
```

**Step 2: Write failing transient-retry test**

```go
func TestRun_TransientFailure_StillRetries(t *testing.T) {
    // handler returns StatusFail + Meta.failure_class=transient_infra for first attempt
    // then success on second attempt
    // expect at least one stage_retry_sleep event and eventual success
}
```

**Step 3: Run tests to verify red**

Run:

```bash
go test ./internal/attractor/engine -run 'TestRun_DeterministicFailure_DoesNotRetry|TestRun_TransientFailure_StillRetries' -count=1
```

Expected: FAIL because `executeWithRetry` currently retries all `fail|retry` outcomes unconditionally.

**Step 4: Commit failing tests**

```bash
git add internal/attractor/engine/retry_failure_class_test.go
git commit -m "test(engine): reproduce missing failure-class retry gating"
```

---

### Task 2: Implement Failure-Class Retry Gating In Engine Retry Loop

**Files:**
- Modify: `internal/attractor/engine/engine.go`
- Optionally create helper: `internal/attractor/engine/retry_class_policy.go`
- Test: `internal/attractor/engine/retry_failure_class_test.go`

**Step 1: Add minimal retry decision helper**

```go
func shouldRetryOutcome(out runtime.Outcome, failureClass string) bool {
    if out.Status != runtime.StatusFail && out.Status != runtime.StatusRetry {
        return false
    }
    return normalizedFailureClassOrDefault(failureClass) == failureClassTransientInfra
}
```

**Step 2: Gate retry path in `executeWithRetry`**

```go
failureClass := classifyFailureClass(out)
if attempt < maxAttempts && shouldRetryOutcome(out, failureClass) {
    // existing backoff + sleep path
} else if attempt < maxAttempts {
    e.appendProgress(map[string]any{
        "event": "stage_retry_blocked",
        "node_id": node.ID,
        "failure_class": normalizedFailureClassOrDefault(failureClass),
        "status": string(out.Status),
    })
    break
}
```

**Step 3: Run task tests to verify green**

Run:

```bash
go test ./internal/attractor/engine -run 'TestRun_DeterministicFailure_DoesNotRetry|TestRun_TransientFailure_StillRetries' -count=1
```

Expected: PASS.

**Step 4: Run nearby retry tests**

Run:

```bash
go test ./internal/attractor/engine -run 'TestRun_RetriesOnFail_ThenSucceeds|TestRun_RetriesOnRetryStatus|TestRun_AllowPartialAfterRetryExhaustion' -count=1
```

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/attractor/engine/engine.go internal/attractor/engine/retry_class_policy.go internal/attractor/engine/retry_failure_class_test.go
git commit -m "fix(engine): gate stage retries by normalized failure_class"
```

---

### Task 3: Add Failing Tests For Provider CLI Error Classification

**Files:**
- Create: `internal/attractor/engine/provider_error_classification_test.go`
- Modify test fixture usage as needed: `internal/attractor/engine/codergen_process_test.go`
- Test: `internal/attractor/engine/provider_error_classification_test.go`

**Step 1: Add Anthropic contract mismatch classification test**

```go
func TestClassifyProviderCLIError_AnthropicStreamJSONRequiresVerbose(t *testing.T) {
    stderr := "Error: When using --print, --output-format=stream-json requires --verbose"
    got := classifyProviderCLIError("anthropic", stderr, errors.New("exit status 1"))
    // expect class deterministic + stable signature for provider_contract
}
```

**Step 2: Add Gemini model-not-found classification test**

```go
func TestClassifyProviderCLIError_GeminiModelNotFound(t *testing.T) {
    stderr := "ModelNotFoundError: Requested entity was not found."
    got := classifyProviderCLIError("google", stderr, errors.New("exit status 1"))
    // expect class deterministic + signature provider_model_unavailable
}
```

**Step 3: Add Codex idle-timeout classification test**

```go
func TestClassifyProviderCLIError_CodexIdleTimeout(t *testing.T) {
    stderr := "codex idle timeout after 2m0s with no output"
    got := classifyProviderCLIError("openai", stderr, errors.New("exit status 1"))
    // expect class transient_infra
}
```

**Step 4: Run tests to verify red**

Run:

```bash
go test ./internal/attractor/engine -run 'TestClassifyProviderCLIError_AnthropicStreamJSONRequiresVerbose|TestClassifyProviderCLIError_GeminiModelNotFound|TestClassifyProviderCLIError_CodexIdleTimeout' -count=1
```

Expected: FAIL (classifier does not exist yet).

**Step 5: Commit failing tests**

```bash
git add internal/attractor/engine/provider_error_classification_test.go
git commit -m "test(engine): reproduce missing provider CLI error class mapping"
```

---

### Task 4: Implement Provider Error Envelope And Anthropic Invocation Fix

**Files:**
- Create: `internal/attractor/engine/provider_error_classification.go`
- Modify: `internal/attractor/engine/codergen_router.go`
- Test: `internal/attractor/engine/provider_error_classification_test.go`
- Test: `internal/attractor/engine/codergen_cli_invocation_test.go`

**Step 1: Implement provider classifier**

```go
type providerCLIClassifiedError struct {
    FailureClass     string
    FailureSignature string
    FailureReason    string
}

func classifyProviderCLIError(provider string, stderr string, runErr error) providerCLIClassifiedError {
    // provider-specific deterministic signatures first
    // fallback to transient timeout/network hints
    // final fallback deterministic
}
```

**Step 2: Attach class/signature to CLI failure outcomes in `runCLI`**

```go
classified := classifyProviderCLIError(providerKey, string(stderrBytes), runErr)
return outStr, &runtime.Outcome{
    Status:        runtime.StatusFail,
    FailureReason: classified.FailureReason,
    Meta: map[string]any{
        "failure_class":     classified.FailureClass,
        "failure_signature": classified.FailureSignature,
    },
    ContextUpdates: map[string]any{
        "failure_class": classified.FailureClass,
    },
}, nil
```

**Step 3: Fix Anthropic CLI invocation contract**

```go
case "anthropic":
    exe = envOr("KILROY_CLAUDE_PATH", "claude")
    args = []string{"-p", "--output-format", "stream-json", "--verbose", "--model", modelID}
```

**Step 4: Run tests for green**

Run:

```bash
go test ./internal/attractor/engine -run 'TestClassifyProviderCLIError_AnthropicStreamJSONRequiresVerbose|TestClassifyProviderCLIError_GeminiModelNotFound|TestClassifyProviderCLIError_CodexIdleTimeout|TestBuildCodexIsolatedEnv_UsesAbsoluteStateRoot' -count=1
```

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/attractor/engine/provider_error_classification.go internal/attractor/engine/codergen_router.go internal/attractor/engine/provider_error_classification_test.go internal/attractor/engine/codergen_cli_invocation_test.go
git commit -m "feat(engine): classify provider CLI failures and fix anthropic stream-json invocation"
```

---

### Task 5: Add Failing Tests For Provider CLI Preflight + Report Artifact

**Files:**
- Modify: `internal/attractor/engine/provider_preflight_test.go`
- Modify: `internal/attractor/engine/run_with_config_integration_test.go`
- Test: `internal/attractor/engine/provider_preflight_test.go`

**Step 1: Add failing test for missing CLI binary**

```go
func TestRunWithConfig_PreflightFails_WhenProviderCLIBinaryMissing(t *testing.T) {
    // cfg: anthropic backend=cli
    // env: KILROY_CLAUDE_PATH=/nonexistent/claude
    // expect deterministic preflight error before cxdb health/stage execution
}
```

**Step 2: Add failing test for Anthropic required flag contract**

```go
func TestRunWithConfig_PreflightFails_WhenAnthropicHelpMissingVerboseFlag(t *testing.T) {
    // fake claude binary prints help without "--verbose"
    // expect preflight deterministic provider_contract failure
}
```

**Step 3: Add failing test for `preflight_report.json` persistence**

```go
func TestRunWithConfig_WritesPreflightReport_Always(t *testing.T) {
    // run one preflight pass and one preflight fail case
    // assert logs_root/preflight_report.json exists in both
}
```

**Step 4: Run tests to verify red**

Run:

```bash
go test ./internal/attractor/engine -run 'TestRunWithConfig_PreflightFails_WhenProviderCLIBinaryMissing|TestRunWithConfig_PreflightFails_WhenAnthropicHelpMissingVerboseFlag|TestRunWithConfig_WritesPreflightReport_Always' -count=1
```

Expected: FAIL (`RunWithConfig` currently has no CLI capability preflight/report artifact).

**Step 5: Commit failing tests**

```bash
git add internal/attractor/engine/provider_preflight_test.go internal/attractor/engine/run_with_config_integration_test.go
git commit -m "test(engine): reproduce missing provider CLI preflight and report artifact"
```

---

### Task 6: Implement Deterministic CLI Preflight + Persist `preflight_report.json`

**Files:**
- Modify: `internal/attractor/engine/run_with_config.go`
- Create: `internal/attractor/engine/provider_preflight.go`
- Modify: `internal/attractor/engine/codergen_router.go` (reuse invocation contract constants/helpers)
- Test: `internal/attractor/engine/provider_preflight_test.go`

**Step 1: Add report model and write helper**

```go
type preflightReport struct {
    Timestamp string                 `json:"timestamp"`
    Checks    []map[string]any       `json:"checks"`
    Summary   map[string]any         `json:"summary"`
}
```

Persist to `filepath.Join(opts.LogsRoot, "preflight_report.json")` before returning from preflight path (pass and fail).

**Step 2: Add deterministic CLI capability checks**

```go
func preflightCLIProvider(provider, exe string, required []string) error {
    // LookPath(exe)
    // run: <exe> --help
    // assert required flags/subcommands present in help text
}
```

Required checks:

1. Anthropic: `-p`, `--output-format`, `--verbose`, `--model`
2. Gemini: `-p`, `--output-format`, `--yolo`, `--model`
3. Codex: `exec`, `--json`, `-m`

**Step 3: Execute preflight before CXDB health and stage execution**

In `RunWithConfig`, after catalog load and provider/model validation, run provider CLI preflight and write the report artifact.

**Step 4: Return class-aware deterministic errors**

Format errors with stable prefix:

```go
return nil, fmt.Errorf("preflight[deterministic]: provider=%s check=%s: %w", provider, checkName, err)
```

**Step 5: Run tests to verify green**

Run:

```bash
go test ./internal/attractor/engine -run 'TestRunWithConfig_PreflightFails_WhenProviderCLIBinaryMissing|TestRunWithConfig_PreflightFails_WhenAnthropicHelpMissingVerboseFlag|TestRunWithConfig_WritesPreflightReport_Always|TestRunWithConfig_FailsFast_WhenCLIModelNotInCatalogForProvider' -count=1
```

Expected: PASS.

**Step 6: Commit**

```bash
git add internal/attractor/engine/run_with_config.go internal/attractor/engine/provider_preflight.go internal/attractor/engine/codergen_router.go internal/attractor/engine/provider_preflight_test.go internal/attractor/engine/run_with_config_integration_test.go
git commit -m "feat(engine): add deterministic provider CLI preflight with persisted report"
```

---

### Task 7: Add Fan-In Aggregate Failure Classification To Prevent Blind Join Retries

**Files:**
- Modify: `internal/attractor/engine/parallel_handlers.go`
- Modify: `internal/attractor/engine/parallel_guardrails_test.go`
- Optionally add: `internal/attractor/engine/fanin_failure_class_test.go`

**Step 1: Add failing fan-in classification tests**

```go
func TestFanIn_AllParallelBranchesFail_DeterministicClass(t *testing.T) {
    // parallel.results with all branch outcomes fail + failure_class=deterministic
    // expect fan-in outcome fail with Meta.failure_class=deterministic
}

func TestFanIn_AllParallelBranchesFail_TransientClass(t *testing.T) {
    // one branch failure_class=transient_infra
    // expect fan-in failure_class=transient_infra
}
```

**Step 2: Run tests to verify red**

Run:

```bash
go test ./internal/attractor/engine -run 'TestFanIn_AllParallelBranchesFail_DeterministicClass|TestFanIn_AllParallelBranchesFail_TransientClass' -count=1
```

Expected: FAIL (fan-in currently returns unclassified `"all parallel branches failed"`).

**Step 3: Implement aggregate classification in `FanInHandler.Execute`**

```go
// when no winner
failureClass := aggregateBranchFailureClass(results) // deterministic unless any transient
return runtime.Outcome{
    Status:        runtime.StatusFail,
    FailureReason: "all parallel branches failed",
    Meta: map[string]any{
        "failure_class":     failureClass,
        "failure_signature": "parallel_all_failed|" + failureClass,
    },
    ContextUpdates: map[string]any{"failure_class": failureClass},
}, nil
```

**Step 4: Run tests to verify green**

Run:

```bash
go test ./internal/attractor/engine -run 'TestFanIn_AllParallelBranchesFail_DeterministicClass|TestFanIn_AllParallelBranchesFail_TransientClass|TestRun_DeterministicFailure_DoesNotRetry' -count=1
```

Expected: PASS; deterministic fan-in outcomes no longer burn retry budget after Task 2.

**Step 5: Commit**

```bash
git add internal/attractor/engine/parallel_handlers.go internal/attractor/engine/parallel_guardrails_test.go internal/attractor/engine/fanin_failure_class_test.go
git commit -m "fix(engine): classify fan-in aggregate failures for retry gating"
```

---

### Task 8: Update Runbook + Guardrail Matrix, Then Run Full Gate

**Files:**
- Modify: `docs/strongdm/attractor/README.md`
- Modify: `scripts/e2e-guardrail-matrix.sh`

**Step 1: Update runbook semantics**

Document:

1. Stage retries are class-gated (`transient_infra` only by default).
2. Provider CLI preflight is deterministic and writes `preflight_report.json`.
3. Fan-in deterministic all-fail outcomes no longer retry blindly.

**Step 2: Extend guardrail script**

Add a fourth check that runs new deterministic retry-gating tests:

```bash
go test ./internal/attractor/engine -run 'TestRun_DeterministicFailure_DoesNotRetry|TestFanIn_AllParallelBranchesFail_DeterministicClass' -count=1
```

**Step 3: Run full verification gate**

Run:

```bash
go test ./cmd/kilroy -count=1
go test ./internal/attractor/runtime -count=1
go test ./internal/attractor/engine -count=1
go test ./internal/llm/providers/... -count=1
bash scripts/e2e-guardrail-matrix.sh
```

Expected: all PASS.

**Step 4: Optional real-run acceptance check**

Run a detached real scenario and verify:

1. Deterministic provider contract/model failures fail fast in preflight when possible.
2. Deterministic stage failures do not consume full retry budget.
3. `final.json` includes explicit deterministic failure reason and CXDB IDs.

**Step 5: Commit**

```bash
git add docs/strongdm/attractor/README.md scripts/e2e-guardrail-matrix.sh
git commit -m "docs+tests: codify class-gated retries and provider preflight guardrails"
```

---

## Required Green Exit Criteria

The branch is complete only when all of the following are true:

1. New failure-class retry gating tests are green.
2. New provider classifier + preflight tests are green.
3. Existing baseline suites remain green:
   - `go test ./cmd/kilroy -count=1`
   - `go test ./internal/attractor/runtime -count=1`
   - `go test ./internal/attractor/engine -count=1`
   - `go test ./internal/llm/providers/... -count=1`
4. `bash scripts/e2e-guardrail-matrix.sh` passes with new class-gating coverage.
5. `preflight_report.json` is present for `RunWithConfig` runs (pass and fail paths).

