# Additional Model List End-to-End Support Implementation Plan

> **For Claude:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ensure models defined in the additional model list are recognized and handled consistently across model catalog loading, engine preflight/runtime metadata checks, and runfile/dotfile authoring skills.

**Architecture:** Introduce one canonical additional model registry at `internal/attractor/modeldb/manual_models.yaml` and load it through `internal/attractor/modeldb`. Merge those entries into the effective run catalog during run/resume so engine catalog checks and failover model selection can see them. Update model-authoring skills to reference this same canonical registry as their final fallback source, eliminating duplicate skill-only model lists.

**Tech Stack:** Go (`internal/attractor/modeldb`, `internal/attractor/engine`, `internal/attractor/validate`), YAML (`gopkg.in/yaml.v3`), Markdown skill docs.

---

**Related skills:** `@writing-plans`, `@executing-plans`, `@create-dotfile`, `@create-runfile`

**Execution context:** Run in a dedicated worktree at `/home/user/code/kilroy/.worktrees/additional-model-list-e2e-support`.

**Scope check:** This is one integrated subsystem (model resolution/catalog + skill alignment), not multiple independent projects.

## Chunk 1: Canonical Additional Model Registry

### File Structure (lock this before edits)

- Create: `internal/attractor/modeldb/manual_models.yaml`
  Responsibility: Canonical additional model list (single source of truth).
- Create: `internal/attractor/modeldb/manual_models.go`
  Responsibility: Parse/validate manual models file, resolve provider/model hints, merge entries into `Catalog`.
- Create: `internal/attractor/modeldb/manual_models_test.go`
  Responsibility: Loader/normalization/merge regression coverage.

### Task 1: Add Manual Model Registry Loader (TDD)

**Files:**
- Create: `internal/attractor/modeldb/manual_models.yaml`
- Create: `internal/attractor/modeldb/manual_models.go`
- Create: `internal/attractor/modeldb/manual_models_test.go`

- [ ] **Step 1: Write the failing tests**

```go
package modeldb

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadManualModels_ResolvesProviderModel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manual_models.yaml")
	_ = os.WriteFile(path, []byte(`models:
  - provider: zai
    model: glm-5
    aliases: ["glm5"]
`), 0o644)

	mm, err := LoadManualModels(path)
	if err != nil {
		t.Fatalf("LoadManualModels: %v", err)
	}
	if got, ok := mm.Resolve("zai", "glm-5"); !ok || got != "glm-5" {
		t.Fatalf("Resolve(zai,glm-5)=(%q,%v) want (glm-5,true)", got, ok)
	}
}

func TestManualModels_MergeIntoCatalog_AddsCoveredProviderAndModel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manual_models.yaml")
	_ = os.WriteFile(path, []byte(`models:
  - provider: zai
    model: glm-5
`), 0o644)

	catalog := &Catalog{Models: map[string]ModelEntry{}, CoveredProviders: map[string]bool{}}
	mm, err := LoadManualModels(path)
	if err != nil {
		t.Fatalf("LoadManualModels: %v", err)
	}
	mm.MergeIntoCatalog(catalog)

	if !CatalogCoversProvider(catalog, "zai") {
		t.Fatalf("expected zai in CoveredProviders after merge")
	}
	if !CatalogHasProviderModel(catalog, "zai", "glm-5") {
		t.Fatalf("expected merged model zai/glm-5")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/attractor/modeldb -run 'TestLoadManualModels_ResolvesProviderModel|TestManualModels_MergeIntoCatalog_AddsCoveredProviderAndModel' -count=1`
Expected: FAIL with undefined `LoadManualModels` / `ManualModelEntry` / merge helpers.

- [ ] **Step 3: Implement minimal loader + merge API**

```go
// internal/attractor/modeldb/manual_models.go
package modeldb

import (
	"fmt"
	"os"
	"strings"

	"github.com/danshapiro/kilroy/internal/modelmeta"
	"gopkg.in/yaml.v3"
)

type ManualModelEntry struct {
	Provider string   `yaml:"provider"`
	Model    string   `yaml:"model"`
	Aliases  []string `yaml:"aliases"`
}

type ManualModels struct {
	entries []ManualModelEntry
	index   map[string]string // provider::normalized_hint -> canonical model
}

type manualModelsFile struct {
	Models []ManualModelEntry `yaml:"models"`
}

func LoadManualModels(path string) (*ManualModels, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var f manualModelsFile
	if err := yaml.Unmarshal(b, &f); err != nil {
		return nil, fmt.Errorf("parse manual models yaml: %w", err)
	}
	mm := &ManualModels{entries: f.Models, index: map[string]string{}}
	for i, e := range f.Models {
		provider := modelmeta.NormalizeProvider(e.Provider)
		model := strings.TrimSpace(e.Model)
		if provider == "" || model == "" {
			return nil, fmt.Errorf("manual_models.yaml entry %d: provider and model are required", i+1)
		}
		mm.index[manualModelIndexKey(provider, model)] = model
		for _, a := range e.Aliases {
			alias := strings.TrimSpace(a)
			if alias == "" {
				return nil, fmt.Errorf("manual_models.yaml entry %d: alias must be non-empty", i+1)
			}
			mm.index[manualModelIndexKey(provider, alias)] = model
		}
	}
	return mm, nil
}

func (m *ManualModels) Resolve(provider, hint string) (string, bool) {
	if m == nil || len(m.index) == 0 {
		return "", false
	}
	v, ok := m.index[manualModelIndexKey(provider, hint)]
	return v, ok
}

func (m *ManualModels) MergeIntoCatalog(c *Catalog) {
	if m == nil || c == nil {
		return
	}
	if c.Models == nil {
		c.Models = map[string]ModelEntry{}
	}
	if c.CoveredProviders == nil {
		c.CoveredProviders = map[string]bool{}
	}
	for _, e := range m.entries {
		provider := modelmeta.NormalizeProvider(e.Provider)
		model := strings.TrimSpace(e.Model)
		if provider == "" || model == "" {
			continue
		}
		c.CoveredProviders[provider] = true
		modelKey := canonicalModelID(provider, model)
		if _, exists := c.Models[modelKey]; !exists {
			c.Models[modelKey] = ModelEntry{Provider: provider, Mode: "chat"}
		}
	}
}

func manualModelIndexKey(provider, hint string) string {
	p := modelmeta.NormalizeProvider(provider)
	h := strings.ToLower(strings.TrimSpace(hint))
	h = strings.Join(strings.Fields(h), " ")
	return p + "::" + h
}
```

- [ ] **Step 4: Add canonical manual model list file**

```yaml
# internal/attractor/modeldb/manual_models.yaml
models:
  - provider: zai
    model: glm-5
    aliases: ["glm5"]
  - provider: kimi
    model: kimi-k2.5
    aliases: ["k2.5"]
  - provider: minimax
    model: minimax-m2.5
    aliases: ["m2.5"]
```

- [ ] **Step 5: Run tests to verify pass**

Run: `go test ./internal/attractor/modeldb -run 'TestLoadManualModels_ResolvesProviderModel|TestManualModels_MergeIntoCatalog_AddsCoveredProviderAndModel' -count=1`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/attractor/modeldb/manual_models.yaml \
  internal/attractor/modeldb/manual_models.go \
  internal/attractor/modeldb/manual_models_test.go
git commit -m "feat(modeldb): add canonical manual model registry and merge helpers"
```

## Chunk 2: Engine Catalog Integration (Run + Resume)

### File Structure (lock this before edits)

- Modify: `internal/attractor/engine/run_with_config.go`
  Responsibility: Snapshot optional manual model list into run storage and load effective run catalog from run-local artifacts only.
- Modify: `internal/attractor/engine/resume.go`
  Responsibility: Resume path uses only run-local snapshotted artifacts (never live repo manual model files).
- Modify: `internal/attractor/engine/run_with_config_test.go`
  Responsibility: Regression coverage for merged manual models in catalog checks.
- Modify: `internal/attractor/engine/resume_catalog_test.go`
  Responsibility: Resume continues to require per-run snapshot while honoring manual merge behavior.

### Task 2: Merge Manual Models Into Effective Catalog (TDD)

**Files:**
- Modify: `internal/attractor/engine/run_with_config.go`
- Modify: `internal/attractor/engine/resume.go`
- Modify: `internal/attractor/engine/run_with_config_test.go`
- Modify: `internal/attractor/engine/resume_catalog_test.go`

- [ ] **Step 1: Write failing engine tests**

```go
func TestLoadCatalogForRun_MergesRunLocalManualModels(t *testing.T) {
	logsRoot := t.TempDir()
	manualPath := filepath.Join(logsRoot, "modeldb", "manual_models.yaml")
	_ = os.MkdirAll(filepath.Dir(manualPath), 0o755)
	_ = os.WriteFile(manualPath, []byte("models:\n  - provider: zai\n    model: glm-5\n"), 0o644)

	catalogPath := filepath.Join(t.TempDir(), "catalog.json")
	_ = os.WriteFile(catalogPath, []byte(`{"data":[{"id":"openai/gpt-5"}]}`), 0o644)

	c, err := loadCatalogForRun(catalogPath, manualPath)
	if err != nil {
		t.Fatalf("loadCatalogForRun: %v", err)
	}
	if !modeldb.CatalogHasProviderModel(c, "zai", "glm-5") {
		t.Fatalf("expected glm-5 from manual models to be present")
	}
}

func TestValidateProviderModelPairs_ManualModelDoesNotWarn(t *testing.T) {
	// build tiny graph with provider=zai model=glm-5 and assert no provider_model_catalog warn
}

func TestLoadCatalogForRun_MissingRunLocalManualSnapshotIsNoop(t *testing.T) {
	catalogPath := filepath.Join(t.TempDir(), "catalog.json")
	_ = os.WriteFile(catalogPath, []byte(`{"data":[{"id":"openai/gpt-5"}]}`), 0o644)

	c, err := loadCatalogForRun(catalogPath, filepath.Join(t.TempDir(), "missing-manual.yaml"))
	if err != nil {
		t.Fatalf("loadCatalogForRun: %v", err)
	}
	if modeldb.CatalogHasProviderModel(c, "zai", "glm-5") {
		t.Fatalf("unexpected merged model when run-local manual snapshot is missing")
	}
}

func TestLoadCatalogForRun_FailsWhenRunLocalManualSnapshotInvalid(t *testing.T) {
	logsRoot := t.TempDir()
	manualPath := filepath.Join(logsRoot, "modeldb", "manual_models.yaml")
	_ = os.MkdirAll(filepath.Dir(manualPath), 0o755)
	_ = os.WriteFile(manualPath, []byte("models: ["), 0o644) // intentionally invalid YAML

	catalogPath := filepath.Join(t.TempDir(), "catalog.json")
	_ = os.WriteFile(catalogPath, []byte(`{"data":[{"id":"openai/gpt-5"}]}`), 0o644)

	if _, err := loadCatalogForRun(catalogPath, manualPath); err == nil {
		t.Fatalf("expected loadCatalogForRun to return manual model parse error")
	}
}

func TestResume_UsesRunLocalManualSnapshot_NotLiveRepoFile(t *testing.T) {
	// create logs_root/modeldb/manual_models.yaml with glm-5 and ensure resume path
	// reads only that run-local snapshot; mutating repo copy must not affect results.
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/attractor/engine -run 'TestLoadCatalogForRun_MergesRunLocalManualModels|TestValidateProviderModelPairs_ManualModelDoesNotWarn|TestLoadCatalogForRun_MissingRunLocalManualSnapshotIsNoop|TestLoadCatalogForRun_FailsWhenRunLocalManualSnapshotInvalid|TestResume_UsesRunLocalManualSnapshot_NotLiveRepoFile' -count=1`
Expected: FAIL because `loadCatalogForRun` is snapshot-only today (no run-local manual merge), resume does not assert run-local-only manual source, and effective-catalog metadata hash/path does not include manual snapshot state.

- [ ] **Step 3: Implement catalog merge in run/resume path**

```go
// run_with_config.go
func loadCatalogForRun(path string, manualSnapshotPath string) (*modeldb.Catalog, error) {
	c, err := modeldb.LoadCatalogFromOpenRouterJSON(path)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(manualSnapshotPath) == "" {
		return c, nil
	}
	mm, err := modeldb.LoadManualModels(manualSnapshotPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// Backward compatibility: older runs may not have snapshotted manual models.
			return c, nil
		}
		return nil, fmt.Errorf("load run-local manual models from %s: %w", manualSnapshotPath, err)
	}

	mm.MergeIntoCatalog(c)
	manualBytes, err := os.ReadFile(manualSnapshotPath)
	if err != nil {
		return nil, fmt.Errorf("hash run-local manual models %s: %w", manualSnapshotPath, err)
	}
	manualSum := sha256.Sum256(manualBytes)
	manualSHA := hex.EncodeToString(manualSum[:])
	effectiveSum := sha256.Sum256([]byte(c.SHA256 + ":" + manualSHA))
	c.SHA256 = hex.EncodeToString(effectiveSum[:])
	c.Path = c.Path + "+" + manualSnapshotPath
	return c, nil
}
```

Imports added in `run_with_config.go` for this block: `crypto/sha256`, `encoding/hex`.

Run-start snapshot step (new helper in `run_with_config.go`):

```go
manualSnapshotPath, manualSHA, err := snapshotManualModelsForRun(cfg.Repo.Path, opts.LogsRoot)
if err != nil {
	return nil, err // file exists but is unreadable/invalid => fail fast at run start
}
catalog, err := loadCatalogForRun(resolved.SnapshotPath, manualSnapshotPath)
if err != nil {
	return nil, err
}
```

Resume step (`resume.go`): use only run-local artifacts.

```go
manualSnapshotPath := firstExistingPath(
	strings.TrimSpace(m.ModelDB.ManualModelsPath),
	filepath.Join(logsRoot, "modeldb", "manual_models.yaml"),
)
cat, err := loadCatalogForRun(snapshotPath, manualSnapshotPath)
```

Manifest update (run start, in `run_with_config.go` manifest writer): persist `manual_models_path` and `manual_models_sha256` under `manifest.ModelDB`; `resume.go` reads manifest first and falls back to canonical run-local path under the same `logs_root` for older manifests.

Callers:

```go
catalog, err := loadCatalogForRun(resolved.SnapshotPath, manualSnapshotPath)
```

```go
cat, err := loadCatalogForRun(snapshotPath, manualSnapshotPath)
```

- [ ] **Step 4: Run tests to verify pass**

Run: `go test ./internal/attractor/engine -run 'TestLoadCatalogForRun_MergesRunLocalManualModels|TestValidateProviderModelPairs_ManualModelDoesNotWarn|TestLoadCatalogForRun_MissingRunLocalManualSnapshotIsNoop|TestLoadCatalogForRun_FailsWhenRunLocalManualSnapshotInvalid|TestResume_UsesRunLocalManualSnapshot_NotLiveRepoFile|TestResume_WithRunConfig_RequiresPerRunModelCatalogSnapshot_OpenRouterName' -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/attractor/engine/run_with_config.go \
  internal/attractor/engine/resume.go \
  internal/attractor/engine/run_with_config_test.go \
  internal/attractor/engine/resume_catalog_test.go
git commit -m "feat(engine): merge manual model list into run and resume catalogs"
```

### Task 3: Provider Preflight Target Coverage for Manual Models (TDD)

**Files:**
- Modify: `internal/attractor/engine/provider_preflight_test.go`
- Modify: `internal/attractor/engine/codergen_failover_test.go`

- [ ] **Step 1: Write failing tests for manual-model visibility in preflight/failover selection**

```go
func TestUsedAPIPromptProbeTargetsForProvider_IncludesManualModel(t *testing.T) {
	// graph contains llm_provider="zai" llm_model="glm-5"
	// catalog produced by loadCatalogForRun includes manual model
	// assert target.Model == "glm-5"
}

func TestPickFailoverModelFromRuntime_SeesManualModelEntries(t *testing.T) {
	// merged catalog should make provider model set include manual entries
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/attractor/engine -run 'TestUsedAPIPromptProbeTargetsForProvider_IncludesManualModel|TestPickFailoverModelFromRuntime_SeesManualModelEntries' -count=1`
Expected: FAIL before implementation/fixtures are updated.

- [ ] **Step 3: Update tests/fixtures to use merged-catalog path**

```go
// Use helper that writes manual_models.yaml under temp logs_root/modeldb,
// then call loadCatalogForRun(snapshotPath, manualSnapshotPath).
```

- [ ] **Step 4: Re-run target tests**

Run: `go test ./internal/attractor/engine -run 'TestUsedAPIPromptProbeTargetsForProvider_IncludesManualModel|TestPickFailoverModelFromRuntime_SeesManualModelEntries' -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/attractor/engine/provider_preflight_test.go \
  internal/attractor/engine/codergen_failover_test.go
git commit -m "test(engine): cover manual-model visibility in preflight and failover paths"
```

## Chunk 3: Skill Contract Alignment (Single Source of Truth)

### File Structure (lock this before edits)

- Modify: `skills/create-dotfile/SKILL.md`
  Responsibility: Model resolution order references canonical manual model list only.
- Modify: `skills/create-runfile/SKILL.md`
  Responsibility: Runfile model resolution order references canonical manual model list only.
- Modify: `skills/english-to-dotfile/SKILL.md`
  Responsibility: Bundle-generation model source order includes canonical manual list.
- Delete: `skills/shared/model_fallbacks.yaml`
  Responsibility: Remove duplicate source of truth.
- Modify: `internal/attractor/validate/skill_model_fallbacks_guardrail_test.go`
  Responsibility: Guardrails enforce canonical source path and reject legacy duplicate path.

### Task 4: Move Skills to Canonical Manual Model Source (TDD)

**Files:**
- Modify: `skills/create-dotfile/SKILL.md`
- Modify: `skills/create-runfile/SKILL.md`
- Modify: `skills/english-to-dotfile/SKILL.md`
- Delete: `skills/shared/model_fallbacks.yaml`
- Modify: `internal/attractor/validate/skill_model_fallbacks_guardrail_test.go`

- [ ] **Step 1: Write failing guardrail tests first**

```go
func TestSkillModelSources_UseManualModelsOnly(t *testing.T) {
	paths := []string{
		"skills/create-dotfile/SKILL.md",
		"skills/create-runfile/SKILL.md",
		"skills/english-to-dotfile/SKILL.md",
	}
	for _, p := range paths {
		text := loadSkillFile(t, strings.Split(p, "/")...)
		if !strings.Contains(text, "internal/attractor/modeldb/manual_models.yaml") {
			t.Fatalf("%s missing canonical manual model source", p)
		}
		if strings.Contains(text, "skills/shared/model_fallbacks.yaml") {
			t.Fatalf("%s must not reference legacy fallback path", p)
		}
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/attractor/validate -run 'TestSkillModelSources_UseManualModelsOnly' -count=1`
Expected: FAIL until skills are updated.

- [ ] **Step 3: Update skills + remove duplicate fallback file**

```md
# create-dotfile / create-runfile / english-to-dotfile
Model source order:
1) run snapshot (if available)
2) internal/attractor/modeldb/pinned/openrouter_models.json
3) internal/attractor/modeldb/manual_models.yaml
4) preserve explicit user model ID when unresolved
```

- [ ] **Step 4: Re-run guardrail tests**

Run: `go test ./internal/attractor/validate -run 'TestSkillModelSources_UseManualModelsOnly' -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add skills/create-dotfile/SKILL.md \
  skills/create-runfile/SKILL.md \
  skills/english-to-dotfile/SKILL.md \
  internal/attractor/validate/skill_model_fallbacks_guardrail_test.go
git rm -f skills/shared/model_fallbacks.yaml
git commit -m "skills+validate: use canonical manual model list and remove duplicate fallback file"
```

## Chunk 4: Docs + Verification Bundle

### Task 5: Document Manual Model List Contract

**Files:**
- Modify: `docs/strongdm/attractor/README.md`
- Modify: `skills/using-kilroy/SKILL.md`

- [ ] **Step 1: Add docs tests/checklist lines to README and using-kilroy skill**

```md
- Additional manual model registry: internal/attractor/modeldb/manual_models.yaml
- Used to supplement pinned/provider catalog when introducing newer provider-native IDs.
- Runtime merges manual models into per-run effective catalog for preflight/catalog checks.
```

- [ ] **Step 2: Validate docs references are present**

Run: `rg -n 'manual_models.yaml|additional manual model registry|supplement' docs/strongdm/attractor/README.md skills/using-kilroy/SKILL.md`
Expected: matches in both files.

- [ ] **Step 3: Commit**

```bash
git add docs/strongdm/attractor/README.md skills/using-kilroy/SKILL.md
git commit -m "docs: document canonical manual model list and runtime merge behavior"
```

### Task 6: Full Verification Pass + Final Integration Commit

**Files:**
- Modify: (none; verification only unless fixes are needed)

- [ ] **Step 1: Run focused package tests**

Run:
```bash
go test ./internal/attractor/modeldb -count=1
go test ./internal/attractor/engine -run 'TestLoadCatalogForRun_MergesRunLocalManualModels|TestValidateProviderModelPairs_ManualModelDoesNotWarn|TestLoadCatalogForRun_MissingRunLocalManualSnapshotIsNoop|TestLoadCatalogForRun_FailsWhenRunLocalManualSnapshotInvalid|TestUsedAPIPromptProbeTargetsForProvider_IncludesManualModel|TestResume_UsesRunLocalManualSnapshot_NotLiveRepoFile|TestResume_WithRunConfig_RequiresPerRunModelCatalogSnapshot_OpenRouterName' -count=1
go test ./internal/attractor/validate -run 'TestSkillModelSources_UseManualModelsOnly' -count=1
```
Expected: all PASS.

- [ ] **Step 2: Run broad regression sweep for safety**

Run: `go test ./...`
Expected: PASS.

- [ ] **Step 3: Commit any final fixups if verification required changes**

```bash
git add internal/attractor/modeldb/manual_models.yaml \
  internal/attractor/modeldb/manual_models.go \
  internal/attractor/modeldb/manual_models_test.go \
  internal/attractor/engine/run_with_config.go \
  internal/attractor/engine/resume.go \
  internal/attractor/engine/run_with_config_test.go \
  internal/attractor/engine/resume_catalog_test.go \
  internal/attractor/engine/provider_preflight_test.go \
  internal/attractor/engine/codergen_failover_test.go \
  internal/attractor/validate/skill_model_fallbacks_guardrail_test.go \
  skills/create-dotfile/SKILL.md \
  skills/create-runfile/SKILL.md \
  skills/english-to-dotfile/SKILL.md \
  docs/strongdm/attractor/README.md \
  skills/using-kilroy/SKILL.md
git commit -m "test: finalize additional model list end-to-end coverage and stability"
```

- [ ] **Step 4: Capture implementation evidence for handoff**

Run:
```bash
git log --oneline -n 8
```
Expected: shows task-level commits for modeldb, engine, skills/validate, docs, and final verification.

## Acceptance Criteria

- `internal/attractor/modeldb/manual_models.yaml` exists and is the only canonical additional model list.
- Effective run catalog (`loadCatalogForRun`) merges run-local manual snapshot entries when present.
- Resume never reads live repository manual-model files; it uses only run-local snapshot artifacts.
- Missing run-local manual snapshot remains a no-op (older runs stay resumable), while malformed snapshot files fail with explicit errors.
- Effective catalog metadata (`Catalog.SHA256`, `Catalog.Path`) reflects merged manual snapshot state when merge occurs.
- Engine catalog checks no longer warn for provider/model pairs that exist only in manual model list.
- Skills (`create-dotfile`, `create-runfile`, `english-to-dotfile`) reference canonical manual list and do not reference duplicate skill-only fallback lists.
- Guardrail tests lock this contract to prevent future drift.

## Risks and Mitigations

- Risk: Dual-source drift between skills and engine.
  Mitigation: Delete duplicate `skills/shared/model_fallbacks.yaml`; enforce via validate guardrail tests.
- Risk: Manual model list typo introduces silent misses.
  Mitigation: Loader validation tests + merge tests + preflight target tests.
- Risk: Resume behavior diverges from run behavior.
  Mitigation: Both code paths call `loadCatalogForRun(snapshotPath, manualSnapshotPath)` using run-local snapshots; resume never consults live repo files.

## Execution Notes

- Keep implementation DRY: model parsing/normalization lives in `modeldb` only.
- Keep YAGNI: do not add new CLI flags or run config schema for this feature.
- Preserve Attractor contract: model identifiers remain provider-native strings; manual list is supplemental metadata, not a strict global allowlist.
