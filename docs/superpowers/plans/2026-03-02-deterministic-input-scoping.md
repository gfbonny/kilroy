# Deterministic Input Scoping and Reference Exclusion Implementation Plan

> **For Claude:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make input materialization deterministically include only intended sources and deterministically exclude unintended closures (for example `.worktrees`, `node_modules`, artifact paths) without requiring a separate “clean directory” workaround.

**Architecture:** Keep `imports`/`include`/`default_include` as the authoritative seed allowlist and keep `follow_references=true` as the production-safe default. Add explicit reference exclusion config and root-scoped resolution so transitive closure remains on, but only traverses intended graph edges. Harden scanner classification to stop treating policy/example glob text as real references, and update docs/templates so production runs have a single idiomatic include/exclude contract.

**Tech Stack:** Go (`internal/attractor/engine`), YAML/JSON runfile config, Doublestar glob matching, attractor spec/docs, runfile skill templates, table-driven Go tests.

---

## Scope Check

This is one subsystem: input materialization semantics (config contract + scanner + resolver + operator docs/templates). It should land as one plan because partial rollout would leave mismatched behavior between runtime, tests, and runfile guidance.

## File Structure

### Config and policy contract
- Modify: `internal/attractor/engine/config.go`
  - Add new input-materialization config fields for deterministic transitive exclusion and defaults.
  - Validate and normalize new fields.
- Modify: `internal/attractor/engine/input_materialization_config.go`
  - Add normalization/validation helpers for exclusion glob entries.
- Modify: `internal/attractor/engine/input_materialization_config_test.go`
  - Add field-presence + normalization regression tests.
- Modify: `internal/attractor/engine/config_test.go`
  - Add load/validation/default tests for new config behavior.

### Scanner and closure resolver
- Modify: `internal/attractor/engine/input_reference_scan.go`
  - Tighten glob token classification and structured token acceptance.
  - Prevent known policy/example exclude lines from being interpreted as true input refs.
- Modify: `internal/attractor/engine/input_materialization.go`
  - Resolve discovered references relative to the source file’s owning root (no cross-root fan-out for relative refs).
  - Apply explicit exclusion globs to discovered transitive refs.
  - Keep explicit seed imports authoritative (include wins over transitive exclusion).
- Modify tests:
  - `internal/attractor/engine/input_reference_scan_test.go`
  - `internal/attractor/engine/input_materialization_test.go`
  - `internal/attractor/engine/input_manifest_contract_test.go`
  - `internal/attractor/engine/input_materialization_integration_test.go` (if needed for branch/snapshot root regression)

### Operator-facing contract
- Modify: `docs/strongdm/attractor/attractor-spec.md`
  - Document deterministic include/exclude semantics and root-scoped traversal.
- Modify: `skills/create-runfile/reference_run_template.yaml` (@create-runfile)
  - Add explicit exclusion field examples and guidance.
- Modify: `skills/create-runfile/SKILL.md` (@create-runfile)
  - Update idiomatic production recommendations (keep follow refs on; exclude intentionally).
- Modify: `internal/attractor/validate/create_runfile_template_guardrail_test.go`
  - Lock template regression coverage for new config contract.

## Chunk 1: Define Deterministic Include/Exclude Config Contract

### Task 1: Add config schema + validation for transitive reference exclusions

**Files:**
- Modify: `internal/attractor/engine/config.go`
- Modify: `internal/attractor/engine/input_materialization_config.go`
- Modify: `internal/attractor/engine/config_test.go`
- Modify: `internal/attractor/engine/input_materialization_config_test.go`
- Test: `internal/attractor/engine/config_test.go`
- Test: `internal/attractor/engine/input_materialization_config_test.go`

- [ ] **Step 1: Write failing tests for new config fields and normalization**

Add tests that assert:
- runfile accepts `inputs.materialize.reference_exclude` (string list),
- entries are trim-normalized and deduplicated in stable first-seen order,
- empty/whitespace-only entries are rejected with deterministic validation errors,
- malformed glob patterns are rejected with deterministic validation errors,
- default when omitted is empty (nil/empty list).

```go
func TestLoadRunConfigFile_InputMaterializationReferenceExclude(t *testing.T) {
    cfg, err := loadRunConfigFromBytesForTest(t, []byte(`
version: 1
repo: { path: /tmp/repo }
cxdb: { binary_addr: 127.0.0.1:9009, http_base_url: http://127.0.0.1:9010 }
llm: { providers: { openai: { backend: api } } }
modeldb: { openrouter_model_info_path: /tmp/catalog.json }
inputs:
  materialize:
    imports:
      - pattern: docs/**/*.md
        required: false
    reference_exclude:
      - " **/node_modules/** "
      - "**/.worktrees/**"
      - "**/node_modules/**"
`))
    if err != nil {
        t.Fatalf("LoadRunConfigFile: %v", err)
    }
    got := cfg.Inputs.Materialize.ReferenceExclude
    want := []string{"**/node_modules/**", "**/.worktrees/**"}
    if !reflect.DeepEqual(got, want) {
        t.Fatalf("reference_exclude mismatch: got=%v want=%v", got, want)
    }
}
```

- [ ] **Step 2: Run targeted tests to confirm failure baseline**

Run: `go test ./internal/attractor/engine -run 'TestLoadRunConfigFile_InputMaterializationReferenceExclude|TestNormalizeAndValidateReferenceExclude|TestNormalizeAndValidateReferenceExclude_InvalidGlobRejected' -count=1`
Expected: FAIL because schema/normalization for `reference_exclude` does not exist yet.

- [ ] **Step 3: Implement schema + normalization + validation**

Implementation details:
- Add `ReferenceExclude []string` to `InputMaterializationConfig` and `InputMaterializationPolicy`.
- In config defaulting path, trim/normalize this list.
- Add helper in `input_materialization_config.go`:
  - validate non-empty after trim,
  - reject malformed glob patterns with deterministic error text,
  - normalize slashes and `filepath.Clean` for non-glob segments,
  - dedupe deterministically.
- Keep contract simple: list is optional; empty means no additional exclusion policy.

- [ ] **Step 4: Re-run targeted tests and full engine package tests**

Run:
- `go test ./internal/attractor/engine -run 'TestLoadRunConfigFile_InputMaterializationReferenceExclude|TestNormalizeAndValidateReferenceExclude|TestNormalizeAndValidateReferenceExclude_InvalidGlobRejected' -count=1`
- `go test ./internal/attractor/engine -run 'TestLoadRunConfigFile_InputMaterialization' -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/attractor/engine/config.go internal/attractor/engine/input_materialization_config.go internal/attractor/engine/config_test.go internal/attractor/engine/input_materialization_config_test.go
git commit -m "engine/config: add deterministic reference exclusion schema"
```

## Chunk 2: Enforce Root-Scoped Reference Resolution

### Task 2: Prevent cross-root fan-out for relative discovered references

**Files:**
- Modify: `internal/attractor/engine/input_materialization.go`
- Modify: `internal/attractor/engine/input_materialization_test.go`
- Optional modify: `internal/attractor/engine/input_materialization_integration_test.go`
- Test: `internal/attractor/engine/input_materialization_test.go`

- [ ] **Step 1: Add failing unit tests for source-root-scoped resolution**

Add tests that assert:
- discovered relative refs from file in root A only resolve in root A,
- stage roots `[worktree, snapshot]` do not cause same relative ref to explode across both roots,
- explicit seed include still works for files in either root.

```go
func TestInputMaterialization_DiscoveredRelativeRefsResolveWithinOwningRoot(t *testing.T) {
    worktree := t.TempDir()
    snapshot := t.TempDir()
    target := t.TempDir()

    mustWriteInputFile(t, filepath.Join(worktree, "docs", "seed.md"), "See [next](next.md)")
    mustWriteInputFile(t, filepath.Join(worktree, "docs", "next.md"), "worktree copy")
    mustWriteInputFile(t, filepath.Join(snapshot, "docs", "next.md"), "snapshot copy")

    _, err := materializeInputClosure(context.Background(), InputMaterializationOptions{
        SourceRoots:      []string{worktree, snapshot},
        Include:          []string{"docs/seed.md"},
        FollowReferences: true,
        TargetRoot:       target,
    })
    if err != nil {
        t.Fatalf("materializeInputClosure: %v", err)
    }

    got, err := os.ReadFile(filepath.Join(target, "docs", "next.md"))
    if err != nil {
        t.Fatal(err)
    }
    if string(got) != "worktree copy" {
        t.Fatalf("unexpected cross-root ref resolution; got %q", string(got))
    }
}
```

- [ ] **Step 2: Run targeted tests to capture current behavior**

Run: `go test ./internal/attractor/engine -run 'TestInputMaterialization_DiscoveredRelativeRefsResolveWithinOwningRoot' -count=1`
Expected: FAIL because resolver currently probes all roots for relative discovered patterns.

- [ ] **Step 3: Implement root-scoped resolver behavior**

In `input_materialization.go`:
- Determine owning root for each `current` source file.
- For discovered relative paths/globs, probe only:
  - source file directory,
  - source file’s owning root + relative pattern,
  - existing source-target mapping fallback when explicitly needed.
- Do not iterate all `roots` for relative discovered refs.
- Keep seed expansion semantics unchanged (`include/default_include/imports` still evaluate against all roots).

- [ ] **Step 4: Run targeted and nearby regression tests**

Run:
- `go test ./internal/attractor/engine -run 'TestInputMaterialization_DiscoveredRelativeRefsResolveWithinOwningRoot|TestInputMaterialization_TransitiveReferenceClosureIncludesReferencedFiles|TestInputMaterialization_ExplicitIncludeTraversesArtifactDocs' -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/attractor/engine/input_materialization.go internal/attractor/engine/input_materialization_test.go internal/attractor/engine/input_materialization_integration_test.go
git commit -m "engine/input: scope discovered references to owning source root"
```

## Chunk 3: Add Deterministic Transitive Exclusion Behavior

### Task 3: Apply explicit exclusion globs to discovered transitive references

**Files:**
- Modify: `internal/attractor/engine/input_materialization.go`
- Modify: `internal/attractor/engine/input_materialization_test.go`
- Modify: `internal/attractor/engine/input_manifest_contract_test.go`
- Test: `internal/attractor/engine/input_materialization_test.go`
- Test: `internal/attractor/engine/input_manifest_contract_test.go`

- [ ] **Step 1: Add failing tests for transitive exclusion and include precedence**

Add tests that assert:
- discovered refs matching `reference_exclude` are skipped,
- explicit includes are still materialized even if they match exclude glob (include precedence),
- manifests/progress show stable resolved counts under exclusion.

```go
func TestInputMaterialization_ReferenceExcludeSkipsDiscoveredArtifacts(t *testing.T) {
    source := t.TempDir()
    target := t.TempDir()

    mustWriteInputFile(t, filepath.Join(source, "docs", "seed.md"), "See [vendor](../node_modules/pkg/readme.md)")
    mustWriteInputFile(t, filepath.Join(source, "node_modules", "pkg", "readme.md"), "artifact")

    manifest, err := materializeInputClosure(context.Background(), InputMaterializationOptions{
        SourceRoots:       []string{source},
        Include:           []string{"docs/seed.md"},
        FollowReferences:  true,
        ReferenceExclude:  []string{"**/node_modules/**"},
        TargetRoot:        target,
    })
    if err != nil {
        t.Fatalf("materializeInputClosure: %v", err)
    }
    if hasString(manifest.ResolvedFiles, "node_modules/pkg/readme.md") {
        t.Fatalf("excluded transitive path should not be resolved: %+v", manifest.ResolvedFiles)
    }
}
```

Add a tiny test-local helper in `input_materialization_test.go` if one does not exist:

```go
func hasString(items []string, want string) bool {
    for _, item := range items {
        if item == want {
            return true
        }
    }
    return false
}
```

- [ ] **Step 2: Run targeted tests to verify failures before implementation**

Run: `go test ./internal/attractor/engine -run 'TestInputMaterialization_ReferenceExcludeSkipsDiscoveredArtifacts|TestInputMaterialization_ReferenceExclude_ExplicitIncludeWins' -count=1`
Expected: FAIL because resolver does not yet apply explicit exclusion policy.

- [ ] **Step 3: Implement exclusion filtering in materialization closure**

Implementation rules:
- Add `ReferenceExclude` to `InputMaterializationOptions` and wire policy from engine config.
- Filter discovered candidates and/or resolved matches by exclusion globs.
- Deterministic precedence:
  - explicit seed includes always win,
  - exclusions apply to discovered transitive refs,
  - best-effort defaults remain best-effort.
- Reuse existing doublestar matching with path normalization to slash style.

- [ ] **Step 4: Validate behavior with unit + contract tests**

Run:
- `go test ./internal/attractor/engine -run 'TestInputMaterialization_ReferenceExclude|TestInputManifestContract_FollowReferencesFalseSkipsRecursiveClosure' -count=1`
Expected: PASS; follow-ref false contract remains unchanged.

- [ ] **Step 5: Commit**

```bash
git add internal/attractor/engine/input_materialization.go internal/attractor/engine/input_materialization_test.go internal/attractor/engine/input_manifest_contract_test.go
git commit -m "engine/input: add deterministic transitive reference exclusions"
```

## Chunk 4: Harden Scanner to Avoid False-Positive Glob Discovery

### Task 4: Stop policy/example glob text from becoming references

**Files:**
- Modify: `internal/attractor/engine/input_reference_scan.go`
- Modify: `internal/attractor/engine/input_reference_scan_test.go`
- Test: `internal/attractor/engine/input_reference_scan_test.go`

- [ ] **Step 1: Add failing scanner tests for exclusion/policy text**

Add tests that assert lines like below do not emit discovered refs:
- `checkpoint.exclude_globs: ["**/node_modules/**"]`
- `exclude_globs:` with list items containing globs
- prose examples where globs are instructional, not file refs.

```go
func TestInputReferenceScan_IgnoresExcludePolicyGlobExamples(t *testing.T) {
    scanner := deterministicInputReferenceScanner{}
    content := []byte(`
exclude_globs:
  - "**/node_modules/**"
  - "**/.worktrees/**"
Use docs/spec.md as the required input.
`)
    refs := scanner.Scan("docs/guide.md", content)
    got := map[string]InputReferenceKind{}
    for _, ref := range refs {
        got[ref.Pattern] = ref.Kind
    }
    if _, ok := got["**/node_modules/**"]; ok {
        t.Fatalf("exclude policy glob should not be a discovered input reference: %+v", refs)
    }
    requireRefKind(t, got, "docs/spec.md", InputReferenceKindPath)
}
```

- [ ] **Step 2: Run targeted scanner tests for red baseline**

Run: `go test ./internal/attractor/engine -run 'TestInputReferenceScan_' -count=1`
Expected: FAIL on new false-positive exclusion tests.

- [ ] **Step 3: Implement scanner hardening**

Implementation details:
- Introduce a small line-aware guard for structured captures:
  - skip tokens captured from lines that are clearly exclusion-policy declarations (for example `exclude_globs`, `reference_exclude`, `checkpoint.exclude_globs`),
  - keep markdown-link extraction behavior for real content paths.
- Tighten glob classification heuristics so wildcard-only instructional tokens do not automatically become refs.

- [ ] **Step 4: Re-run scanner and nearby materialization tests**

Run:
- `go test ./internal/attractor/engine -run 'TestInputReferenceScan_|TestInputMaterialization_TransitiveReferenceClosureIncludesReferencedFiles' -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/attractor/engine/input_reference_scan.go internal/attractor/engine/input_reference_scan_test.go
git commit -m "engine/input: suppress false-positive glob reference discovery"
```

## Chunk 5: Document Idiomatic Production Usage and Lock Template Contract

### Task 5: Update spec/template/skill guidance for deterministic include/exclude

**Files:**
- Modify: `docs/strongdm/attractor/attractor-spec.md`
- Modify: `skills/create-runfile/reference_run_template.yaml`
- Modify: `skills/create-runfile/SKILL.md`
- Modify: `internal/attractor/validate/create_runfile_template_guardrail_test.go`
- Optional modify: `demo/substack-run-v01.yaml`
- Test: `internal/attractor/validate/create_runfile_template_guardrail_test.go`

- [ ] **Step 1: Add failing guardrail test for run template contract**

Add assertions that reference template includes the new exclusion field and keeps:
- `follow_references: true` as baseline recommendation,
- explicit comments that `follow_references: false` is seed-only mode and may miss required dependencies.

- [ ] **Step 2: Run targeted guardrail test to establish baseline**

Run: `go test ./internal/attractor/validate -run 'TestCreateRunfileTemplate' -count=1`
Expected: FAIL until template/docs are updated.

- [ ] **Step 3: Update docs/template/skill with deterministic contract**

Required documentation points:
- Include sources through `imports` (or legacy include/default_include mapping).
- Exclude transitive noise through `reference_exclude`.
- Keep `follow_references=true` for production unless operator intentionally wants seed-only behavior.
- Explain that clean-directory symlink workaround is no longer required when config is set correctly.

Template snippet target:

```yaml
inputs:
  materialize:
    enabled: true
    imports:
      - pattern: "docs/**/*.md"
        required: false
    reference_exclude:
      - "**/node_modules/**"
      - "**/.worktrees/**"
      - "**/.git/**"
    follow_references: true
    infer_with_llm: false
```

- [ ] **Step 4: Re-run guardrail + relevant engine tests**

Run:
- `go test ./internal/attractor/validate -run 'TestCreateRunfileTemplate|TestReferenceTemplate' -count=1`
- `go test ./internal/attractor/engine -run 'TestLoadRunConfigFile_InputMaterialization|TestInputManifestContract_' -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add docs/strongdm/attractor/attractor-spec.md skills/create-runfile/reference_run_template.yaml skills/create-runfile/SKILL.md internal/attractor/validate/create_runfile_template_guardrail_test.go demo/substack-run-v01.yaml
git commit -m "docs/runfile: codify deterministic input include/exclude contract"
```

## Chunk 6: Final Validation and Integration Gate

### Task 6: Full repo validation checklist before merge

**Files:**
- Modify: none (validation only)

- [ ] **Step 1: Run formatting check exactly like CI**

Run: `gofmt -l . | grep -v '^\./\.claude/' | grep -v '^\.claude/'`
Expected: no output.

- [ ] **Step 2: Run vet/build/tests**

Run:
- `go vet ./...`
- `go build ./cmd/kilroy/`
- `go test ./...`
Expected: PASS.

- [ ] **Step 3: Validate demo DOT graphs with current binary**

Run:

```bash
go build -o ./kilroy ./cmd/kilroy
for f in demo/**/*.dot; do echo "Validating $f"; ./kilroy attractor validate --graph "$f"; done
```

Expected: all validations pass.

- [ ] **Step 4: Summarize contract verification evidence in PR description draft**

Include:
- new config fields and precedence,
- before/after behavior for cross-root and false glob expansion,
- proof that follow-ref default remains production-safe while exclusion controls scope.

- [ ] **Step 5: Final commit for any post-validation nits**

```bash
git status --short
# Stage only files touched by this plan (no blanket add):
git add internal/attractor/engine/config.go internal/attractor/engine/input_materialization_config.go internal/attractor/engine/input_materialization.go internal/attractor/engine/input_reference_scan.go internal/attractor/engine/config_test.go internal/attractor/engine/input_materialization_config_test.go internal/attractor/engine/input_materialization_test.go internal/attractor/engine/input_reference_scan_test.go internal/attractor/engine/input_manifest_contract_test.go internal/attractor/engine/input_materialization_integration_test.go internal/attractor/validate/create_runfile_template_guardrail_test.go docs/strongdm/attractor/attractor-spec.md skills/create-runfile/reference_run_template.yaml skills/create-runfile/SKILL.md demo/substack-run-v01.yaml
git commit -m "chore: finalize deterministic input scoping validation fixes"
```

## Execution Notes

- Keep TDD order strict per task: failing test -> minimal implementation -> targeted tests -> commit.
- Avoid broad refactors in unrelated packages.
- Preserve backward compatibility for existing runfiles that do not declare `reference_exclude`.
- Do not change production provider routing or run command behavior in this effort.

## Expected Outcomes

- Production runs can deterministically declare what should be included and what should be excluded without relying on workspace layout tricks.
- `follow_references=false` remains available but documented as seed-only mode; default guidance remains `true` for production reliability.
- Input closure avoids accidental expansion into artifact trees and multi-root fan-out from incidental glob text.
