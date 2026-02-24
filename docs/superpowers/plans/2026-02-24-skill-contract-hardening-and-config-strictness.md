# Skill Contract Hardening and Config Strictness Implementation Plan

> **For Claude:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove silent contract drift between skills and runtime by hardening ingest prompt binding, enforcing strict run-config schema validation, and adding guardrail tests for dotfile/runfile skill outputs.

**Architecture:** Keep Attractor orchestration semantics unchanged while tightening interfaces at the boundaries where LLM-authored artifacts enter the system. Make ingest prompt instructions skill-aware, make run-config decoding reject unsupported keys, and align skill templates/instructions to fields the engine actually consumes. Lock behavior with targeted guardrail tests in `internal/attractor/validate` and unit tests in `internal/attractor/engine`/`internal/attractor/ingest`.

**Tech Stack:** Go (`testing`, `encoding/json`, `gopkg.in/yaml.v3`), Markdown skills, YAML templates.

---

## Scope and Worktree

- Execute in a dedicated worktree.
- Related skills: `@writing-plans`, `@create-dotfile`, `@create-runfile`, `@executing-plans`.

## File Structure

- Modify: `internal/attractor/ingest/ingest.go`
  Responsibility: make ingest prompt rendering skill-name-aware instead of hardcoding `english-to-dotfile`.
- Modify: `internal/attractor/ingest/ingest_prompt.tmpl`
  Responsibility: consume a templated skill name in the operator instruction line.
- Modify: `internal/attractor/ingest/ingest_test.go`
  Responsibility: lock prompt/CLI arg behavior for skill-name-aware prompts.

- Modify: `internal/attractor/engine/config.go`
  Responsibility: strict decode for YAML/JSON unknown fields; schema ownership for accepted top-level keys.
- Modify: `internal/attractor/engine/config_test.go`
  Responsibility: regression tests for unknown-key rejection and allowed metadata keys.

- Modify: `skills/create-runfile/SKILL.md`
  Responsibility: align runfile authoring guidance with schema-backed fields only.
- Modify: `skills/create-runfile/reference_run_template.yaml`
  Responsibility: remove unsupported/no-op keys; keep only engine-consumed fields.
- Create: `internal/attractor/validate/create_runfile_skill_guardrail_test.go`
  Responsibility: assert runfile skill/template contract does not drift back to unsupported keys.

- Modify: `skills/create-dotfile/SKILL.md`
  Responsibility: enforce canonical failure-class vocabulary and generic artifact-hygiene verification guidance.
- Create: `internal/attractor/validate/create_dotfile_skill_guardrail_test.go`
  Responsibility: assert dotfile skill includes canonical failure classes and artifact-hygiene guardrails.

## Chunk 1: Runtime Boundary Contracts

### Task 1: Make Ingest Prompt Skill-Aware (Not Hardcoded)

**Files:**
- Modify: `internal/attractor/ingest/ingest_test.go`
- Modify: `internal/attractor/ingest/ingest.go`
- Modify: `internal/attractor/ingest/ingest_prompt.tmpl`
- Test: `internal/attractor/ingest/ingest_test.go`

- [ ] **Step 1: Write failing tests for prompt skill-name binding**

```go
func TestBuildCLIArgs_PromptUsesSkillNameFromSkillPath(t *testing.T) {
    // skillPath: /tmp/.../skills/create-dotfile/SKILL.md
    // assert final positional prompt contains "Follow the create-dotfile skill"
}

func TestBuildCLIArgs_PromptFallsBackToGenericSkillLabelWithoutSkillPath(t *testing.T) {
    // assert prompt contains a generic fallback label, not "english-to-dotfile"
}
```

- [ ] **Step 2: Run ingest tests and verify failure**

Run: `go test ./internal/attractor/ingest -run 'TestBuildCLIArgs_PromptUsesSkillNameFromSkillPath|TestBuildCLIArgs_PromptFallsBackToGenericSkillLabelWithoutSkillPath' -count=1`
Expected: FAIL because prompt template is currently hardcoded.

- [ ] **Step 3: Implement skill-name-aware prompt rendering**

```go
// ingest.go
func buildPrompt(requirements, skillName string) string { ... }
func inferSkillName(skillPath string) string {
    // prefer parent dir name when file is SKILL.md (e.g., create-dotfile)
    // fallback: "provided" when unknown
}
```

```text
# ingest_prompt.tmpl
Follow the {{.SkillName}} skill in your system prompt exactly.
```

- [ ] **Step 4: Re-run ingest tests**

Run: `go test ./internal/attractor/ingest -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/attractor/ingest/ingest.go \
  internal/attractor/ingest/ingest_prompt.tmpl \
  internal/attractor/ingest/ingest_test.go
git commit -m "ingest: bind prompt instruction to active skill name"
```

### Task 2: Enforce Strict Run Config Decoding

**Files:**
- Modify: `internal/attractor/engine/config_test.go`
- Modify: `internal/attractor/engine/config.go`
- Test: `internal/attractor/engine/config_test.go`

- [ ] **Step 1: Add failing tests for unknown-key rejection and allowed metadata**

```go
func TestLoadRunConfigFile_RejectsUnknownTopLevelKey(t *testing.T) {
    // include artifact_policy: ... in YAML
    // expect error mentions unknown field artifact_policy
}

func TestLoadRunConfigFile_RejectsUnknownNestedKey(t *testing.T) {
    // typo key under llm.providers.openai
    // expect strict decode failure
}

func TestLoadRunConfigFile_AllowsGraphAndTaskMetadata(t *testing.T) {
    // include graph: ... and task: ...
    // expect load success
}
```

- [ ] **Step 2: Run config tests and verify failure**

Run: `go test ./internal/attractor/engine -run 'TestLoadRunConfigFile_RejectsUnknownTopLevelKey|TestLoadRunConfigFile_RejectsUnknownNestedKey|TestLoadRunConfigFile_AllowsGraphAndTaskMetadata' -count=1`
Expected: FAIL because loader currently tolerates unknown keys.

- [ ] **Step 3: Implement strict YAML/JSON decode and schema allowances**

```go
// config.go
// add optional schema fields for accepted metadata:
Graph string `json:"graph,omitempty" yaml:"graph,omitempty"`
Task  string `json:"task,omitempty" yaml:"task,omitempty"`

// YAML:
dec := yaml.NewDecoder(bytes.NewReader(b))
dec.KnownFields(true)

// JSON:
dec := json.NewDecoder(bytes.NewReader(b))
dec.DisallowUnknownFields()
```

- [ ] **Step 4: Re-run targeted and package tests**

Run: `go test ./internal/attractor/engine -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/attractor/engine/config.go internal/attractor/engine/config_test.go
git commit -m "engine/config: reject unknown run-config keys with strict decoding"
```

## Chunk 2: Skill and Template Contract Guardrails

### Task 3: Align Create-Runfile Skill/Template to Schema-Backed Fields

**Files:**
- Create: `internal/attractor/validate/create_runfile_skill_guardrail_test.go`
- Modify: `skills/create-runfile/SKILL.md`
- Modify: `skills/create-runfile/reference_run_template.yaml`
- Test: `internal/attractor/validate/create_runfile_skill_guardrail_test.go`

- [ ] **Step 1: Write failing guardrail tests for unsupported key usage**

```go
func TestCreateRunfileTemplate_DoesNotContainArtifactPolicy(t *testing.T) {
    // parse template text; fail if "artifact_policy:" appears
}

func TestCreateRunfileSkill_RequiresSchemaBackedFieldsOnly(t *testing.T) {
    // assert SKILL.md includes "Do not emit unsupported keys"
    // assert SKILL.md references git.checkpoint_exclude_globs guidance
}
```

- [ ] **Step 2: Run validate tests and verify failure**

Run: `go test ./internal/attractor/validate -run 'TestCreateRunfileTemplate_DoesNotContainArtifactPolicy|TestCreateRunfileSkill_RequiresSchemaBackedFieldsOnly' -count=1`
Expected: FAIL because current template includes `artifact_policy`.

- [ ] **Step 3: Update runfile skill and template**

```yaml
# reference_run_template.yaml (git section)
git:
  require_clean: false
  run_branch_prefix: attractor/run
  commit_per_node: true
  checkpoint_exclude_globs:
    - "**/.cargo-target*/**"
    - "**/.cargo_target*/**"
    - "**/.wasm-pack/**"
    - "**/.tmpbuild/**"
```

```md
# SKILL.md additions
- Emit only fields supported by `internal/attractor/engine/config.go`.
- Do not emit unsupported keys (example: `artifact_policy`).
- For artifact/checkpoint hygiene, use `git.checkpoint_exclude_globs`.
```

- [ ] **Step 4: Re-run validate + config package tests**

Run: `go test ./internal/attractor/validate ./internal/attractor/engine -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/attractor/validate/create_runfile_skill_guardrail_test.go \
  skills/create-runfile/SKILL.md \
  skills/create-runfile/reference_run_template.yaml
git commit -m "skill(create-runfile): remove unsupported fields and enforce schema-backed output"
```

### Task 4: Harden Create-Dotfile Failure-Class and Artifact-Hygiene Guidance

**Files:**
- Create: `internal/attractor/validate/create_dotfile_skill_guardrail_test.go`
- Modify: `skills/create-dotfile/SKILL.md`
- Test: `internal/attractor/validate/create_dotfile_skill_guardrail_test.go`

- [ ] **Step 1: Write failing skill-guardrail tests**

```go
func TestCreateDotfileSkill_UsesCanonicalFailureClasses(t *testing.T) {
    // require explicit list:
    // transient_infra, budget_exhausted, compilation_loop,
    // deterministic, canceled, structural
}

func TestCreateDotfileSkill_RejectsNonCanonicalFailureClassLabels(t *testing.T) {
    // require guidance that labels like "semantic" are not valid failure_class values
}

func TestCreateDotfileSkill_RequiresGenericArtifactHygieneChecks(t *testing.T) {
    // require guidance to emit deterministic verify step with exact offending paths
}
```

- [ ] **Step 2: Run validate tests and verify failure**

Run: `go test ./internal/attractor/validate -run 'TestCreateDotfileSkill_UsesCanonicalFailureClasses|TestCreateDotfileSkill_RejectsNonCanonicalFailureClassLabels|TestCreateDotfileSkill_RequiresGenericArtifactHygieneChecks' -count=1`
Expected: FAIL before skill text is updated.

- [ ] **Step 3: Update create-dotfile skill contract text**

```md
# SKILL.md additions
- `failure_class` must be one of:
  `transient_infra`, `budget_exhausted`, `compilation_loop`,
  `deterministic`, `canceled`, `structural`.
- Do not emit non-canonical values (example: `semantic`).
- `verify_artifacts` checks must report exact offending paths and fail deterministically.
```

- [ ] **Step 4: Re-run validate tests**

Run: `go test ./internal/attractor/validate -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/attractor/validate/create_dotfile_skill_guardrail_test.go \
  skills/create-dotfile/SKILL.md
git commit -m "skill(create-dotfile): enforce canonical failure classes and artifact hygiene guardrails"
```

### Task 5: Final Regression Pass

**Files:**
- Test: `internal/attractor/ingest/ingest_test.go`
- Test: `internal/attractor/engine/config_test.go`
- Test: `internal/attractor/validate/*.go`

- [ ] **Step 1: Run focused package tests**

Run: `go test ./internal/attractor/ingest ./internal/attractor/engine ./internal/attractor/validate -count=1`
Expected: PASS.

- [ ] **Step 2: Run full suite smoke**

Run: `go test ./...`
Expected: PASS (or capture and triage unrelated pre-existing failures).

- [ ] **Step 3: Build binary**

Run: `go build -o ./kilroy ./cmd/kilroy`
Expected: build succeeds.

- [ ] **Step 4: Sanity-check template and generated rogue artifacts**

Run: `./kilroy attractor validate --graph demo/rogue/rogue.dot`
Expected: `ok: rogue.dot`.

- [ ] **Step 5: Commit final polish (if needed)**

```bash
git add -A
git commit -m "test(validate): lock skill/runtime contracts for ingest and run-config schema"
```

## Risks and Mitigations

- Risk: strict decode may reject legacy configs with ad-hoc keys.
- Mitigation: explicitly allow known metadata keys (`graph`, `task`), fail fast on everything else, and document the accepted schema in skill/template guardrails.

- Risk: skill edits drift again and reintroduce unsupported fields/classes.
- Mitigation: dedicated guardrail tests in `internal/attractor/validate` assert required language directly.

- Risk: ingest prompt wording change regresses existing ingest tests.
- Mitigation: add explicit prompt-shape tests in `internal/attractor/ingest/ingest_test.go`.

## Acceptance Criteria

- Ingest prompt instructions reference the active skill name rather than hardcoded `english-to-dotfile`.
- `LoadRunConfigFile` rejects unknown YAML/JSON keys.
- `skills/create-runfile/reference_run_template.yaml` no longer includes unsupported `artifact_policy`.
- `skills/create-runfile/SKILL.md` explicitly forbids unsupported keys and points to schema-backed hygiene keys.
- `skills/create-dotfile/SKILL.md` uses canonical failure classes and explicit artifact-hygiene guidance.
- New and updated tests pass in ingest/engine/validate packages.
