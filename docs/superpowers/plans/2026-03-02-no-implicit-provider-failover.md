# No Implicit Provider Failover Implementation Plan

> **For Claude:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove all hardcoded provider failover defaults so failover happens only when `llm.providers.<provider>.failover` is explicitly set in the runfile.

**Architecture:** Keep provider capability metadata (API protocol, base URL, aliases, CLI contract) in builtins, but remove builtin failover policy as executable behavior. Runtime/provider resolution and codergen routing will consume only run-config failover data. Preflight and model selection paths must continue to work for explicit failover chains and must not probe or route into implicit providers.

**Tech Stack:** Go, Kilroy attractor engine, provider runtime metadata, table-driven Go tests, markdown docs.

---

## Scope Check

This is a single subsystem change (provider failover policy semantics). It touches provider specs, runtime assembly, codergen routing, preflight behavior, and tests/docs in the same execution domain. A single plan is appropriate.

## File Structure Map

- Modify: `internal/providerspec/builtin.go`
  Responsibility: Builtin provider metadata (remove default failover lists).
- Modify: `internal/providerspec/spec_test.go`
  Responsibility: Assertions for builtin provider defaults (update to "no default failover").
- Modify: `internal/attractor/engine/provider_runtime.go`
  Responsibility: Build runtime provider map from run config; stop inheriting builtin failover when omitted.
- Modify: `internal/attractor/engine/provider_runtime_test.go`
  Responsibility: Runtime failover semantics coverage (explicit list honored, omitted list => none).
- Modify: `internal/attractor/engine/codergen_router.go`
  Responsibility: LLM failover routing logic; remove hardcoded fallback order path.
- Modify: `internal/attractor/engine/codergen_failover_test.go`
  Responsibility: Router failover behavior tests; make failover explicit in fixtures.
- Modify: `internal/attractor/engine/provider_preflight_test.go`
  Responsibility: Preflight coverage for failover-target probing/routing only when explicitly configured.
- Modify: `docs/strongdm/attractor/README.md`
  Responsibility: Operator-facing behavior contract for explicit failover policy.
- Optional modify (if needed by test/doc references): `README.md`
  Responsibility: top-level run-config behavior notes.

## Chunk 1: Remove Implicit Defaults In Runtime + Router

### Task 1: Remove Builtin Failover Defaults From Provider Specs

**Files:**
- Modify: `internal/providerspec/spec_test.go`
- Modify: `internal/providerspec/builtin.go`
- Test: `internal/providerspec/spec_test.go`

- [ ] **Step 1: Write the failing spec-default test**

```go
func TestBuiltinFailoverDefaultsAreEmpty(t *testing.T) {
    for _, provider := range []string{"openai", "anthropic", "google", "kimi", "zai", "cerebras", "minimax", "inception"} {
        spec, ok := Builtin(provider)
        if !ok {
            t.Fatalf("expected builtin provider %q", provider)
        }
        if len(spec.Failover) != 0 {
            t.Fatalf("%s failover=%v want []", provider, spec.Failover)
        }
    }
}
```

Replace the existing `TestBuiltinFailoverDefaultsAreSingleHop` with this new test (do not keep both).

- [ ] **Step 2: Run test to verify it fails against current behavior**

Run: `go test ./internal/providerspec -run TestBuiltinFailoverDefaultsAreEmpty -count=1`
Expected: FAIL with providers still returning single-hop failover defaults.

- [ ] **Step 3: Remove hardcoded failover arrays from builtins**

Edit `internal/providerspec/builtin.go` and delete `Failover: [...]` entries from builtin provider definitions.

- [ ] **Step 4: Run provider spec tests**

Run: `go test ./internal/providerspec -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/providerspec/builtin.go internal/providerspec/spec_test.go
git commit -m "providerspec: remove implicit builtin failover defaults"
```

### Task 2: Make Provider Runtime Resolution Respect Runfile-Only Failover

**Files:**
- Modify: `internal/attractor/engine/provider_runtime_test.go`
- Modify: `internal/attractor/engine/provider_runtime.go`
- Test: `internal/attractor/engine/provider_runtime_test.go`

- [ ] **Step 1: Add/replace runtime tests for omitted-failover behavior**

Add/replace tests to assert:
- omitted `Failover` leaves `ProviderRuntime.Failover` empty,
- no synthesized failover targets appear unless a configured provider explicitly references them,
- explicit `Failover` still synthesizes target runtime metadata as needed.
- rewrite `TestResolveProviderRuntimes_MergesBuiltinAndConfigOverrides` to validate explicit synthesis from configured failover (not builtin defaults),
- replace `TestResolveProviderRuntimes_OmittedFailoverUsesBuiltinFallback` with `TestResolveProviderRuntimes_OmittedFailoverHasNoFallback`.
- add `TestResolveProviderRuntimes_BuiltinFailoverIgnoredWhenOmitted` for a builtin provider key (for example `openai`) asserting omitted `Failover` yields no runtime failover and no synthesized target runtime.
- keep/verify explicit-empty behavior coverage (`TestResolveProviderRuntimes_ExplicitEmptyFailoverDisablesBuiltinFallback`) so `failover: []` remains a protected contract.

```go
func TestResolveProviderRuntimes_OmittedFailoverHasNoFallback(t *testing.T) {
    cfg := &RunConfigFile{}
    cfg.LLM.Providers = map[string]ProviderConfig{
        "zai": {Backend: BackendAPI},
    }
    rt, err := resolveProviderRuntimes(cfg)
    if err != nil {
        t.Fatalf("resolveProviderRuntimes: %v", err)
    }
    zai, ok := rt["zai"]
    if !ok {
        t.Fatalf("missing runtime for zai")
    }
    if len(zai.Failover) != 0 {
        t.Fatalf("zai failover=%v want []", zai.Failover)
    }
    if _, ok := rt["cerebras"]; ok {
        t.Fatalf("unexpected synthesized failover target cerebras without explicit failover")
    }
}

func TestResolveProviderRuntimes_ExplicitFailoverSynthesizesTargetRuntime(t *testing.T) {
    cfg := &RunConfigFile{}
    cfg.LLM.Providers = map[string]ProviderConfig{
        "kimi": {
            Backend:  BackendAPI,
            Failover: []string{"zai"},
            API: ProviderAPIConfig{
                APIKeyEnv: "KIMI_API_KEY",
                Headers:   map[string]string{"X-Trace": "1"},
            },
        },
    }
    rt, err := resolveProviderRuntimes(cfg)
    if err != nil {
        t.Fatalf("resolveProviderRuntimes: %v", err)
    }
    kimi, ok := rt["kimi"]
    if !ok {
        t.Fatalf("missing runtime for kimi")
    }
    if got := kimi.Failover; len(got) != 1 || got[0] != "zai" {
        t.Fatalf("kimi failover=%v want [zai]", got)
    }
    zai, ok := rt["zai"]
    if !ok {
        t.Fatalf("expected synthesized failover target runtime for zai")
    }
    if len(zai.Failover) != 0 {
        t.Fatalf("zai synthesized failover=%v want [] (no implicit chained defaults)", zai.Failover)
    }
    if _, ok := rt["cerebras"]; ok {
        t.Fatalf("unexpected recursive synthesized failover target cerebras without explicit runfile chain")
    }
    if got := kimi.APIHeaders(); got["X-Trace"] != "1" {
        t.Fatalf("expected runtime headers copy, got %v", got)
    }
}

func TestResolveProviderRuntimes_BuiltinFailoverIgnoredWhenOmitted(t *testing.T) {
    cfg := &RunConfigFile{}
    cfg.LLM.Providers = map[string]ProviderConfig{
        "openai": {Backend: BackendAPI},
    }
    rt, err := resolveProviderRuntimes(cfg)
    if err != nil {
        t.Fatalf("resolveProviderRuntimes: %v", err)
    }
    openai, ok := rt["openai"]
    if !ok {
        t.Fatalf("missing runtime for openai")
    }
    if len(openai.Failover) != 0 {
        t.Fatalf("openai failover=%v want [] when runfile omits failover", openai.Failover)
    }
    if _, ok := rt["google"]; ok {
        t.Fatalf("unexpected synthesized google runtime without explicit runfile failover")
    }
}
```

- [ ] **Step 2: Run targeted runtime tests to capture baseline**

Run: `go test ./internal/attractor/engine -run 'TestResolveProviderRuntimes_' -count=1`
Expected: PASS after legacy default-based tests are rewritten/replaced in Step 1. This is baseline verification only; Step 3 cleanup is still mandatory and enforced by Step 4 code-presence checks.

- [ ] **Step 3: Implement runtime behavior changes**

In `provider_runtime.go`:
- remove inheritance from `builtin.Failover` when `pc.Failover` is omitted,
- keep explicit `pc.Failover` canonicalization,
- when synthesizing referenced target runtimes, do not inject builtin failover chains implicitly.

- [ ] **Step 4: Re-run runtime tests + dead-path check**

Run:
- `go test ./internal/attractor/engine -run 'TestResolveProviderRuntimes_' -count=1`
- `rg -n 'len\\(builtin\\.Failover\\)|builtin\\.Failover' internal/attractor/engine/provider_runtime.go`
Expected: tests PASS; `rg` returns no matches (implicit builtin-failover inheritance path removed).


- [ ] **Step 5: Commit**

```bash
git add internal/attractor/engine/provider_runtime.go internal/attractor/engine/provider_runtime_test.go
git commit -m "engine/runtime: require explicit runfile failover"
```

### Task 3: Remove Hardcoded Router Fallback Order

**Files:**
- Modify: `internal/attractor/engine/codergen_failover_test.go`
- Modify: `internal/attractor/engine/codergen_router.go`
- Test: `internal/attractor/engine/codergen_failover_test.go`

- [ ] **Step 1: Add/adjust failing router tests to require explicit failover config**

Update existing tests that currently rely on implicit defaults to explicitly set failover in fixtures:
- `TestCodergenRouter_WithFailoverText_FailsOverToDifferentProvider`
- `TestCodergenRouter_WithFailoverText_AppliesForceModelToFailoverProvider`
- in each updated test, resolve runtime policy via `resolveProviderRuntimes(cfg)` and construct router with `NewCodergenRouterWithRuntimes(cfg, catalog, runtimes)` (avoid bypassing runtime failover policy with direct `NewCodergenRouter(...)` setup).
- delete or rewrite `TestFailoverOrder_DefaultsAreSingleHop` because hardcoded default order is being removed.

Add a test for omitted failover:

```go
func TestCodergenRouter_WithFailoverText_OmittedFailoverDoesNotFallback(t *testing.T) {
    cfg := &RunConfigFile{Version: 1}
    cfg.LLM.Providers = map[string]ProviderConfig{
        "openai": {Backend: BackendAPI},
        "google": {Backend: BackendAPI},
    }
    runtimes, err := resolveProviderRuntimes(cfg)
    if err != nil {
        t.Fatalf("resolveProviderRuntimes: %v", err)
    }
    r := NewCodergenRouterWithRuntimes(cfg, nil, runtimes)
    attempted := []string{}
    _, used, err := r.withFailoverText(context.Background(), nil, &model.Node{ID: "n1"}, llm.NewClient(), "openai", "gpt-5.4", func(provider, model string) (string, error) {
        attempted = append(attempted, provider)
        return "", llm.NewNetworkError(provider, "synthetic outage")
    })
    if err == nil {
        t.Fatal("expected error")
    }
    if used.Provider != "openai" {
        t.Fatalf("unexpected fallback provider: %q", used.Provider)
    }
    if len(attempted) != 1 || attempted[0] != "openai" {
        t.Fatalf("unexpected fallback attempts: %v", attempted)
    }
}
```

- [ ] **Step 2: Run targeted router failover tests and verify failures**

Run: `go test ./internal/attractor/engine -run 'TestCodergenRouter_WithFailoverText_|TestFailoverOrder_' -count=1`
Expected: FAIL while hardcoded default order still exists.

- [ ] **Step 3: Implement router changes**

In `codergen_router.go`:
- remove `failoverOrder(primary)` fallback path,
- remove/retire `failoverOrder()` helper if now unused,
- keep runtime-driven `failoverOrderFromRuntime` only,
- keep failover attempts gated by retryable error classification.
- keep edits constrained to failover-specific helpers/branches in this file (do not mix unrelated router refactors).

- [ ] **Step 4: Run router failover tests**

Run:
- `go test ./internal/attractor/engine -run 'TestCodergenRouter_WithFailoverText_|TestFailoverOrder_|TestPickFailoverModelFromRuntime_' -count=1`
- `rg -n 'failoverOrder\\(' internal/attractor/engine/codergen_router.go`
Expected: tests PASS; `rg` returns no matches (hardcoded fallback-order helper removed).

- [ ] **Step 5: Commit**

```bash
git add internal/attractor/engine/codergen_router.go internal/attractor/engine/codergen_failover_test.go
git commit -m "engine/router: remove hardcoded provider failover order"
```

## Chunk 2: Guardrails, Docs, and Verification

### Task 4: Confirm Preflight Only Traverses Explicit Failover Chains

**Files:**
- Create: `internal/attractor/engine/provider_preflight_failover_policy_test.go`
- Optional Modify: `internal/attractor/engine/provider_preflight.go` (only if Step 2 reveals implicit traversal still exists)
- Optional Modify: `internal/attractor/engine/provider_runtime.go` (only if Step 2 reveals implicit traversal still exists)
- Test: `internal/attractor/engine/provider_preflight_failover_policy_test.go`

- [ ] **Step 1: Add failing preflight regression test for omitted failover**

Add a test that configures only one provider with no `Failover` and asserts preflight target derivation does not include additional providers.

```go
func TestUsedAPIProviders_OmittedFailoverDoesNotIncludeImplicitTargets(t *testing.T) {
    g, err := dot.Parse([]byte(`digraph G {
      start [shape=Mdiamond]
      a [shape=box, llm_provider="openai", llm_model="gpt-5.4", prompt="x"]
      exit [shape=Msquare]
      start -> a -> exit
    }`))
    if err != nil {
        t.Fatalf("parse dot: %v", err)
    }
    cfg := &RunConfigFile{}
    t.Setenv("TEST_GOOGLE_API_KEY", "x")
    cfg.LLM.Providers = map[string]ProviderConfig{
        "openai": {Backend: BackendAPI}, // failover omitted on purpose
        "google": {
            Backend: BackendAPI, // should not be included without explicit failover edge
            API: ProviderAPIConfig{
                APIKeyEnv: "TEST_GOOGLE_API_KEY",
            },
        },
    }
    runtimes, err := resolveProviderRuntimes(cfg)
    if err != nil {
        t.Fatalf("resolveProviderRuntimes: %v", err)
    }
    got := usedAPIProviders(g, runtimes)
    if strings.Join(got, ",") != "openai" {
        t.Fatalf("providers=%v want [openai]", got)
    }
}
```

- [ ] **Step 2: Run targeted preflight tests and verify behavior**

Run: `go test ./internal/attractor/engine -run 'TestUsedAPIProviders_' -count=1`
Expected: PASS if explicit-only traversal is already correct; may FAIL if any implicit traversal remains.

- [ ] **Step 3: Apply minimal code changes only if preflight still implicitly traverses defaults**

If failures show implicit traversal, adjust runtime/preflight glue so only configured failover edges are traversed.

- [ ] **Step 4: Re-run preflight tests**

Run: `go test ./internal/attractor/engine -run 'TestUsedAPIProviders_|TestProviderPreflight_PromptProbe_' -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/attractor/engine/provider_preflight_failover_policy_test.go
# If Step 3 required code changes, include only the touched files:
# git add internal/attractor/engine/provider_preflight.go internal/attractor/engine/provider_runtime.go
git commit -m "engine/preflight: enforce explicit-only failover traversal"
```

### Task 5: Document Explicit Failover Contract + Run Full Verification

**Files:**
- Modify: `docs/strongdm/attractor/README.md`
- Optional Modify: `README.md`

- [ ] **Step 1: Add doc statement for new contract**

Add a concise contract note under provider plug-ins:
- no implicit failover defaults,
- failover occurs only when `llm.providers.<provider>.failover` is explicitly set,
- use `failover: []` to explicitly disable failover for a provider entry.

- [ ] **Step 2: Add run-config example snippet**

```yaml
llm:
  providers:
    openai:
      backend: api
      failover: [google]
    google:
      backend: api
      failover: []
```

- [ ] **Step 3: Run focused and broad verification**

Run:
- `go test ./internal/providerspec -count=1`
- `go test ./internal/attractor/engine -run 'Failover|ProviderRuntime|UsedAPIProviders|ProviderPreflight_PromptProbe' -count=1`

Expected: PASS.

- [ ] **Step 4: Run CI parity checks for this repo before final commit**

Run:
- `gofmt -l . | grep -v '^\./\.claude/' | grep -v '^\.claude/'`
- `go vet ./...`
- `go build ./cmd/kilroy/`
- `go test ./...`
- `for f in demo/**/*.dot; do echo \"Validating $f\"; ./kilroy attractor validate --graph \"$f\"; done`
- `rg -n 'Failover:\\s*\\[]string\\{' internal/providerspec/builtin.go && false || true`
- `rg -n 'func failoverOrder\\(' internal/attractor/engine/codergen_router.go && false || true`
- `rg -n 'else if len\\(builtin\\.Failover\\)' internal/attractor/engine/provider_runtime.go && false || true`

Expected: no output from gofmt command; all others exit 0.

- [ ] **Step 5: Commit**

```bash
git add docs/strongdm/attractor/README.md
# If top-level docs were edited, include explicitly:
# git add README.md
git commit -m "docs(attractor): document explicit-only provider failover policy"
```

## Final Merge Checklist

- [ ] Confirm no hardcoded failover defaults remain in provider specs or router fallback helpers.
- [ ] Confirm omitted `failover` in run config results in zero failover attempts.
- [ ] Confirm explicit failover lists still work for routing and preflight probes.
- [ ] Confirm documentation matches implementation.
- [ ] Rebase worktree branch onto `main`, fast-forward merge, and push after user approval.
