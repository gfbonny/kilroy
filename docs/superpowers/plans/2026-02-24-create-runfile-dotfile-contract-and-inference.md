# Create-Runfile Dotfile Contract and Inference Hardening Implementation Plan

> **For Claude:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `create-runfile` require an explicitly user-identified DOT file, infer runfile defaults from that DOT + repo signals (unless the user expresses a choice), and always report assumptions at the end.

**Architecture:** Keep engine/runtime behavior unchanged. Harden only the `create-runfile` skill contract and its reference template so behavior is deterministic and human-ergonomic. Add guardrail tests that fail if the skill drifts from this contract.

**Tech Stack:** Markdown skill authoring, YAML template authoring, Go guardrail tests (`internal/attractor/validate`), `go test`.

---

**Related Skills:** `@create-runfile`, `@create-dotfile`, `@writing-plans`

## Scope Check

This is one subsystem (`create-runfile`) and is intentionally separate from engine refactors. The plan focuses on authoring behavior and guardrails only.

## Guardrails

- Do not change Attractor engine semantics or run-config schema in this plan.
- Do not assume a DOT file path from repo defaults (`pipeline.dot`, `rogue.dot`, etc.).
- Do not encode “avoid deprecated X” negative guidance; provide affirmative canonical field declarations instead.
- Inferred values are defaults; explicit user choices override inferred defaults.
- Inference defaults must be deterministic via an explicit precedence order.
- Preserve boundary: DOT topology/routing remains owned by `create-dotfile`.

## File Structure Map

- Create: `internal/attractor/validate/create_runfile_skill_guardrail_test.go`  
  Responsibility: lock the `create-runfile` contract (explicit dotfile input, inference behavior, assumptions output, canonical field declarations).
- Modify: `skills/create-runfile/SKILL.md`  
  Responsibility: implement the new input/behavior/output contract in affirmative language.
- Modify: `skills/create-runfile/reference_run_template.yaml`  
  Responsibility: add explicit DOT binding marker comments and keep canonical schema examples aligned with the skill.

## Chunk 1: Contract and Guardrails

### Task 1: Add Failing Guardrail Tests for `create-runfile`

**Files:**
- Create: `internal/attractor/validate/create_runfile_skill_guardrail_test.go`
- Test: `internal/attractor/validate/create_runfile_skill_guardrail_test.go`

- [ ] **Step 1: Write the failing tests**

```go
package validate

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func loadCreateRunfileSkill(t *testing.T) string {
	t.Helper()
	repoRoot := findRepoRoot(t)
	b, err := os.ReadFile(filepath.Join(repoRoot, "skills", "create-runfile", "SKILL.md"))
	if err != nil {
		t.Fatalf("read create-runfile skill: %v", err)
	}
	return string(b)
}

func loadCreateRunfileTemplate(t *testing.T) string {
	t.Helper()
	repoRoot := findRepoRoot(t)
	b, err := os.ReadFile(filepath.Join(repoRoot, "skills", "create-runfile", "reference_run_template.yaml"))
	if err != nil {
		t.Fatalf("read create-runfile reference template: %v", err)
	}
	return string(b)
}

func lookupPath(root map[string]any, path ...string) (any, bool) {
	cur := any(root)
	for _, p := range path {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		v, ok := m[p]
		if !ok {
			return nil, false
		}
		cur = v
	}
	return cur, true
}

func listContains(items []any, want string) bool {
	for _, it := range items {
		if s, ok := it.(string); ok && s == want {
			return true
		}
	}
	return false
}

func parseSkillContract(t *testing.T) map[string]any {
	s := loadCreateRunfileSkill(t)
	re := regexp.MustCompile(`(?s)<!-- CRF-CONTRACT-BEGIN -->\s*```yaml\s*(.*?)\s*```\s*<!-- CRF-CONTRACT-END -->`)
	m := re.FindStringSubmatch(s)
	if len(m) != 2 {
		t.Fatal("missing machine-readable CRF contract block in SKILL.md")
	}
	var doc map[string]any
	if err := yaml.Unmarshal([]byte(m[1]), &doc); err != nil {
		t.Fatalf("invalid CRF contract yaml: %v", err)
	}
	return doc
}

func TestCreateRunfileSkill_ContractPayload_RequiredInputs(t *testing.T) {
	contract := parseSkillContract(t)
	if got, _ := lookupPath(contract, "required_inputs", "dotfile", "required"); got != true {
		t.Fatalf("required_inputs.dotfile.required must be true, got %#v", got)
	}
	if got, _ := lookupPath(contract, "required_inputs", "dotfile", "source"); got != "explicit_user_path" {
		t.Fatalf("required_inputs.dotfile.source mismatch: %#v", got)
	}
	if got, _ := lookupPath(contract, "required_inputs", "dotfile", "behavior_if_missing"); got != "stop_and_request" {
		t.Fatalf("required_inputs.dotfile.behavior_if_missing mismatch: %#v", got)
	}
	if got, _ := lookupPath(contract, "required_inputs", "dotfile", "never_assume"); got != true {
		t.Fatalf("required_inputs.dotfile.never_assume must be true, got %#v", got)
	}
}

func TestCreateRunfileSkill_ContractPayload_InferencePolicy(t *testing.T) {
	contract := parseSkillContract(t)
	defaultsFromAny, ok := lookupPath(contract, "inference_policy", "defaults_from")
	if !ok {
		t.Fatal("missing inference_policy.defaults_from")
	}
	defaultsFrom, ok := defaultsFromAny.([]any)
	if !ok {
		t.Fatalf("inference_policy.defaults_from must be list, got %#v", defaultsFromAny)
	}
	if !listContains(defaultsFrom, "dotfile") || !listContains(defaultsFrom, "repo_signals") {
		t.Fatalf("inference_policy.defaults_from must include dotfile and repo_signals, got %#v", defaultsFrom)
	}
	if got, _ := lookupPath(contract, "inference_policy", "user_override"); got != true {
		t.Fatalf("inference_policy.user_override must be true, got %#v", got)
	}
	preAny, ok := lookupPath(contract, "inference_policy", "precedence")
	if !ok {
		t.Fatal("missing inference_policy.precedence")
	}
	pre, ok := preAny.([]any)
	if !ok || len(pre) != 4 {
		t.Fatalf("inference_policy.precedence must be 4-item list, got %#v", preAny)
	}
	want := []string{
		"user_choice",
		"provider_capability_constraints",
		"existing_run_config_if_compatible",
		"default_real",
	}
	for i, w := range want {
		if s, ok := pre[i].(string); !ok || s != w {
			t.Fatalf("inference precedence mismatch at %d: want %q got %#v", i, w, pre[i])
		}
	}
	if got, _ := lookupPath(contract, "inference_policy", "conflicting_existing_config"); got != "do_not_inherit_require_user_resolution" {
		t.Fatalf("inference_policy.conflicting_existing_config mismatch: %#v", got)
	}
	if got, _ := lookupPath(contract, "inference_policy", "test_shim_default"); got != "never" {
		t.Fatalf("inference_policy.test_shim_default must be never, got %#v", got)
	}
}

func TestCreateRunfileSkill_ContractPayload_InferenceSourcesAndDotfileGate(t *testing.T) {
	contract := parseSkillContract(t)
	sourcesAny, ok := lookupPath(contract, "inference_sources")
	if !ok {
		t.Fatal("missing inference_sources")
	}
	sources, ok := sourcesAny.([]any)
	if !ok || !listContains(sources, "llm_provider_attrs") || !listContains(sources, "model_stylesheet") {
		t.Fatalf("inference_sources must include llm_provider_attrs and model_stylesheet, got %#v", sourcesAny)
	}
	if got, _ := lookupPath(contract, "provider_alignment", "require_backend_for_each_dot_provider"); got != true {
		t.Fatalf("provider_alignment.require_backend_for_each_dot_provider must be true, got %#v", got)
	}
	famsAny, ok := lookupPath(contract, "dotfile_preconditions", "placeholder_families")
	if !ok {
		t.Fatal("missing dotfile_preconditions.placeholder_families")
	}
	fams, ok := famsAny.([]any)
	if !ok {
		t.Fatalf("dotfile_preconditions.placeholder_families must be list, got %#v", famsAny)
	}
	for _, fam := range []string{"DEFAULT", "HARD", "VERIFY", "BRANCH"} {
		if !listContains(fams, fam) {
			t.Fatalf("missing placeholder family %q", fam)
		}
	}
	suffixesAny, ok := lookupPath(contract, "dotfile_preconditions", "placeholder_suffixes")
	if !ok {
		t.Fatal("missing dotfile_preconditions.placeholder_suffixes")
	}
	suffixes, ok := suffixesAny.([]any)
	if !ok || !listContains(suffixes, "PROVIDER") || !listContains(suffixes, "MODEL") {
		t.Fatalf("placeholder suffixes must include PROVIDER and MODEL, got %#v", suffixesAny)
	}
	if got, _ := lookupPath(contract, "dotfile_preconditions", "behavior_if_unresolved"); got != "stop_and_request_resolved_dotfile" {
		t.Fatalf("dotfile_preconditions.behavior_if_unresolved mismatch: %#v", got)
	}
}

func TestCreateRunfileSkill_ContractPayload_RuntimeRobustnessAndCanonicalMap(t *testing.T) {
	contract := parseSkillContract(t)
	if got, _ := lookupPath(contract, "runtime_robustness", "preserve_cxdb_autostart"); got != true {
		t.Fatalf("runtime_robustness.preserve_cxdb_autostart must be true, got %#v", got)
	}
	if got, _ := lookupPath(contract, "runtime_robustness", "preserve_git_policy"); got != true {
		t.Fatalf("runtime_robustness.preserve_git_policy must be true, got %#v", got)
	}
	if got, _ := lookupPath(contract, "runtime_robustness", "preserve_preflight_real_baseline"); got != true {
		t.Fatalf("runtime_robustness.preserve_preflight_real_baseline must be true, got %#v", got)
	}
	if got, _ := lookupPath(contract, "runtime_robustness", "preserve_artifact_policy"); got != true {
		t.Fatalf("runtime_robustness.preserve_artifact_policy must be true, got %#v", got)
	}
	fieldsAny, ok := lookupPath(contract, "canonical_field_map")
	if !ok {
		t.Fatal("missing canonical_field_map")
	}
	fields, ok := fieldsAny.([]any)
	if !ok {
		t.Fatalf("canonical_field_map must be list, got %#v", fieldsAny)
	}
	for _, f := range []string{
		"modeldb.openrouter_model_info_update_policy",
		"modeldb.openrouter_model_info_url",
		"modeldb.openrouter_model_info_fetch_timeout_ms",
		"artifact_policy.profiles",
		"artifact_policy.env.managed_roots",
		"artifact_policy.env.overrides",
		"artifact_policy.checkpoint.exclude_globs",
	} {
		if !listContains(fields, f) {
			t.Fatalf("canonical_field_map missing %q", f)
		}
	}
}

func TestCreateRunfileSkill_ContractPayload_OutputContract(t *testing.T) {
	contract := parseSkillContract(t)
	if got, _ := lookupPath(contract, "output_contract", "assumptions_required"); got != true {
		t.Fatalf("output_contract.assumptions_required must be true, got %#v", got)
	}
	if got, _ := lookupPath(contract, "output_contract", "assumptions_placement"); got != "final_section" {
		t.Fatalf("output_contract.assumptions_placement must be final_section, got %#v", got)
	}
}

func TestCreateRunfileTemplate_HasDotfileBindingMarkerAndCanonicalKeys(t *testing.T) {
	tpl := loadCreateRunfileTemplate(t)
	if !strings.Contains(tpl, "REQUIRED_USER_INPUT.dotfile:") {
		t.Fatal("template missing REQUIRED_USER_INPUT.dotfile marker")
	}
	var doc map[string]any
	if err := yaml.Unmarshal([]byte(tpl), &doc); err != nil {
		t.Fatalf("template must parse as yaml: %v", err)
	}
	for _, p := range [][]string{
		{"version"},
		{"repo", "path"},
		{"cxdb", "binary_addr"},
		{"cxdb", "http_base_url"},
		{"cxdb", "autostart", "enabled"},
		{"cxdb", "autostart", "command"},
		{"llm", "cli_profile"},
		{"modeldb", "openrouter_model_info_path"},
		{"modeldb", "openrouter_model_info_update_policy"},
		{"modeldb", "openrouter_model_info_url"},
		{"modeldb", "openrouter_model_info_fetch_timeout_ms"},
		{"git", "require_clean"},
		{"git", "run_branch_prefix"},
		{"git", "commit_per_node"},
		{"artifact_policy", "profiles"},
		{"artifact_policy", "env", "managed_roots"},
		{"artifact_policy", "env", "overrides"},
		{"artifact_policy", "checkpoint", "exclude_globs"},
		{"runtime_policy"},
		{"preflight", "prompt_probes"},
	} {
		if _, ok := lookupPath(doc, p...); !ok {
			t.Fatalf("template missing required key path: %s", strings.Join(p, "."))
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/attractor/validate -run 'CreateRunfileSkill|CreateRunfileTemplate' -count=1`  
Expected: FAIL because required sections/invariants are not yet present.

- [ ] **Step 3: Commit the failing tests**

```bash
git add internal/attractor/validate/create_runfile_skill_guardrail_test.go
git commit -m "test(validate): add create-runfile skill contract guardrails"
```

### Task 2: Implement the Skill Contract in `SKILL.md`

**Files:**
- Modify: `skills/create-runfile/SKILL.md`
- Test: `internal/attractor/validate/create_runfile_skill_guardrail_test.go`

- [ ] **Step 1: Add machine-readable CRF contract payload to `SKILL.md`**

Add a marker-delimited YAML payload near the top of the skill:

````md
<!-- CRF-CONTRACT-BEGIN -->
```yaml
contract_version: 1
required_inputs:
  dotfile:
    required: true
    source: explicit_user_path
    behavior_if_missing: stop_and_request
    never_assume: true
inference_policy:
  defaults_from: [dotfile, repo_signals]
  user_override: true
  precedence:
    - user_choice
    - provider_capability_constraints
    - existing_run_config_if_compatible
    - default_real
  conflicting_existing_config: do_not_inherit_require_user_resolution
  test_shim_default: never
inference_sources: [llm_provider_attrs, model_stylesheet]
provider_alignment:
  require_backend_for_each_dot_provider: true
dotfile_preconditions:
  placeholder_families: [DEFAULT, HARD, VERIFY, BRANCH]
  placeholder_suffixes: [PROVIDER, MODEL]
  behavior_if_unresolved: stop_and_request_resolved_dotfile
runtime_robustness:
  preserve_cxdb_autostart: true
  preserve_git_policy: true
  preserve_preflight_real_baseline: true
  preserve_artifact_policy: true
canonical_field_map:
  - version
  - repo.path
  - cxdb.binary_addr
  - cxdb.http_base_url
  - cxdb.autostart.enabled
  - cxdb.autostart.command
  - llm.cli_profile
  - llm.providers.<provider>.backend
  - modeldb.openrouter_model_info_path
  - modeldb.openrouter_model_info_update_policy
  - modeldb.openrouter_model_info_url
  - modeldb.openrouter_model_info_fetch_timeout_ms
  - git.require_clean
  - git.run_branch_prefix
  - git.commit_per_node
  - artifact_policy.profiles
  - artifact_policy.env.managed_roots
  - artifact_policy.env.overrides
  - artifact_policy.checkpoint.exclude_globs
  - runtime_policy
  - preflight.prompt_probes
output_contract:
  assumptions_required: true
  assumptions_placement: final_section
```
<!-- CRF-CONTRACT-END -->
````

- [ ] **Step 2: Keep human-readable sections synchronized with the contract payload**

Keep or add prose sections (`Required Inputs`, `Inference Policy`, `Inference Sources`, `Dotfile Preconditions`, `Runtime Robustness Non-Negotiables`, `Output Contract`) but treat them as explanatory text. The YAML payload is the guardrail source of truth.

- [ ] **Step 3: Ensure deterministic precedence and conflict handling are explicit**

In prose and payload:
- precedence must be `user_choice > provider_capability_constraints > existing_run_config_if_compatible > default_real`
- `test_shim` remains opt-in only
- conflicting existing config must not be inherited silently and must require explicit user resolution.

- [ ] **Step 4: Run skill-contract tests only**

Run: `go test ./internal/attractor/validate -run 'CreateRunfileSkill_ContractPayload_' -count=1`  
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add skills/create-runfile/SKILL.md
git commit -m "skill(create-runfile): require explicit dotfile input and assumption reporting"
```

## Chunk 2: Template Alignment and Final Validation

### Task 3: Align `reference_run_template.yaml` with Dotfile Binding Contract

**Files:**
- Modify: `skills/create-runfile/reference_run_template.yaml`
- Test: `internal/attractor/validate/create_runfile_skill_guardrail_test.go`

- [ ] **Step 1: Add explicit binding comments at top of template**

Add comment header:

```yaml
# This runfile is authored for the dotfile identified by the user.
# REQUIRED_USER_INPUT.dotfile: /absolute/path/to/pipeline.dot
```

- [ ] **Step 2: Keep canonical schema examples aligned with SKILL.md**

Confirm template still demonstrates canonical fields used by the skill, especially:
- `llm.providers.<provider>.backend`
- `modeldb.openrouter_model_info_path`
- `modeldb.openrouter_model_info_update_policy`
- `modeldb.openrouter_model_info_url`
- `modeldb.openrouter_model_info_fetch_timeout_ms`
- `cxdb.autostart.enabled`
- `cxdb.autostart.command`
- `git.require_clean`
- `git.run_branch_prefix`
- `git.commit_per_node`
- `artifact_policy.profiles`
- `artifact_policy.env.managed_roots`
- `artifact_policy.env.overrides`
- `artifact_policy.checkpoint.exclude_globs`
- `runtime_policy`
- `preflight.prompt_probes`
- Add/retain the top marker comment `# REQUIRED_USER_INPUT.dotfile: /absolute/path/to/pipeline.dot`.

- [ ] **Step 3: Run targeted tests**

Run: `go test ./internal/attractor/validate -run 'CreateRunfileTemplate_' -count=1`  
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add skills/create-runfile/reference_run_template.yaml
git commit -m "skill(create-runfile): bind reference template to explicit user dotfile input"
```

### Task 4: Final Regression Pass

**Files:**
- Modify: none expected (verification only)
- Test: `internal/attractor/validate/create_runfile_skill_guardrail_test.go`

- [ ] **Step 1: Run full validate package tests**

Run: `go test ./internal/attractor/validate -count=1`  
Expected: PASS.

- [ ] **Step 2: Run repository formatting/lint sanity for touched files (lightweight)**

Run:

```bash
rg -n "CRF-CONTRACT-BEGIN|CRF-CONTRACT-END" skills/create-runfile/SKILL.md
rg -n "provider_capability_constraints|existing_run_config_if_compatible|do_not_inherit_require_user_resolution|test_shim_default: never" skills/create-runfile/SKILL.md
rg -n "placeholder_families|DEFAULT|HARD|VERIFY|BRANCH|placeholder_suffixes|PROVIDER|MODEL" skills/create-runfile/SKILL.md
rg -n "modeldb\\.openrouter_model_info_update_policy|modeldb\\.openrouter_model_info_url|modeldb\\.openrouter_model_info_fetch_timeout_ms|artifact_policy\\.profiles|artifact_policy\\.env\\.managed_roots|artifact_policy\\.env\\.overrides|artifact_policy\\.checkpoint\\.exclude_globs" skills/create-runfile/SKILL.md
rg -n "REQUIRED_USER_INPUT.dotfile" skills/create-runfile/reference_run_template.yaml
```

Expected: matching lines are present.

- [ ] **Step 3: Commit final verification note (optional if no file changes)**

```bash
# No-op commit only if your workflow requires it; otherwise skip.
```

## Risks and Mitigations

- Risk: Markdown guidance drifts while tests stay green.
- Mitigation: Guardrail tests parse marker-delimited YAML contract payload (`CRF-CONTRACT-BEGIN/END`) and assert semantic keys/values.

- Risk: Skill text could drift from template examples.
- Mitigation: Template guardrail assertions parse YAML and enforce required canonical keys, including `modeldb.*` and `artifact_policy.*` invariants.

## Definition of Done

- `create-runfile` refuses to proceed without a user-identified dotfile path.
- `create-runfile` infers runfile defaults from the specified dotfile + repo signals unless the user expresses a choice.
- `create-runfile` applies deterministic inference precedence (user choice > provider constraints > existing run config if compatible > `real` default), with `test_shim` opt-in only.
- `create-runfile` does not silently inherit conflicting existing run config and instead requires explicit user resolution.
- `create-runfile` uses both `llm_provider` attrs and `model_stylesheet` as inference sources.
- `create-runfile` stops and requests a resolved dotfile when unresolved `*_PROVIDER` / `*_MODEL` placeholders remain (including `DEFAULT_*`, `HARD_*`, `VERIFY_*`, `BRANCH_*` families).
- `create-runfile` preserves runtime robustness non-negotiables for `cxdb.autostart.*`, `git.*`, `modeldb.*` catalog knobs, `artifact_policy.*`, and real-provider preflight baseline expectations.
- `create-runfile` always reports assumptions at the end.
- Skill guidance is affirmative/canonical, not deprecation-negative.
- Guardrail tests pass and lock these behaviors.
