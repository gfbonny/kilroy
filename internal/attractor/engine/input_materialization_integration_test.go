package engine

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/danshapiro/kilroy/internal/attractor/runtime"
)

func TestInputMaterializationIntegration_RunStartupHydratesUntrackedDoDAndReferences(t *testing.T) {
	repo := initTestRepo(t)
	logsRoot := t.TempDir()
	mustWriteInputFile(t, filepath.Join(repo, ".ai", "definition_of_done.md"), "Do all checks. See [tests](../tests.md)\n")
	mustWriteInputFile(t, filepath.Join(repo, "tests.md"), "acceptance tests")

	cfg := newInputMaterializationRunConfigForTest(t, repo)
	dot := []byte(`
digraph G {
  graph [goal="input startup hydration"]
  start [shape=Mdiamond]
  exit [shape=Msquare]
  read_inputs [shape=parallelogram, tool_command="test -f .ai/definition_of_done.md && test -f tests.md && cp .ai/definition_of_done.md dod_seen.txt"]
  start -> read_inputs -> exit
}
`)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	res, err := RunWithConfig(ctx, dot, cfg, RunOptions{
		RunID:       "input-startup-hydration",
		LogsRoot:    logsRoot,
		DisableCXDB: true,
	})
	if err != nil {
		t.Fatalf("RunWithConfig: %v", err)
	}
	if res.FinalStatus != runtime.FinalSuccess {
		t.Fatalf("final status: got %q want %q", res.FinalStatus, runtime.FinalSuccess)
	}

	assertExists(t, filepath.Join(res.WorktreeDir, ".ai", "definition_of_done.md"))
	assertExists(t, filepath.Join(res.WorktreeDir, "tests.md"))
	assertExists(t, filepath.Join(res.WorktreeDir, "dod_seen.txt"))
	assertExists(t, inputRunManifestPath(res.LogsRoot))
	assertExists(t, inputStageManifestPath(res.LogsRoot, "read_inputs"))
	assertExists(t, inputSnapshotFilesRoot(res.LogsRoot))
}

func TestInputMaterializationIntegration_ParallelBranchHydration(t *testing.T) {
	repo := initTestRepo(t)
	logsRoot := t.TempDir()
	mustWriteInputFile(t, filepath.Join(repo, ".ai", "definition_of_done.md"), "Run branch checks. See [tests](../tests.md)\n")
	mustWriteInputFile(t, filepath.Join(repo, "tests.md"), "branch tests")

	cfg := newInputMaterializationRunConfigForTest(t, repo)
	dot := []byte(`
digraph P {
  graph [goal="parallel branch hydration"]
  start [shape=Mdiamond]
  par [shape=component]
  a [shape=parallelogram, tool_command="test -f .ai/definition_of_done.md && test -f tests.md && echo a > branch_a.txt"]
  b [shape=parallelogram, tool_command="test -f .ai/definition_of_done.md && test -f tests.md && echo b > branch_b.txt"]
  join [shape=tripleoctagon]
  exit [shape=Msquare]
  start -> par
  par -> a
  par -> b
  a -> join
  b -> join
  join -> exit
}
`)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	res, err := RunWithConfig(ctx, dot, cfg, RunOptions{
		RunID:       "input-branch-hydration",
		LogsRoot:    logsRoot,
		DisableCXDB: true,
	})
	if err != nil {
		t.Fatalf("RunWithConfig: %v", err)
	}
	if res.FinalStatus != runtime.FinalSuccess {
		t.Fatalf("final status: got %q want %q", res.FinalStatus, runtime.FinalSuccess)
	}

	resultsPath := filepath.Join(res.LogsRoot, "par", "parallel_results.json")
	b, err := os.ReadFile(resultsPath)
	if err != nil {
		t.Fatalf("read parallel_results.json: %v", err)
	}
	var results []parallelBranchResult
	if err := json.Unmarshal(b, &results); err != nil {
		t.Fatalf("unmarshal parallel_results.json: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 branch results, got %d", len(results))
	}
	for _, r := range results {
		assertExists(t, filepath.Join(r.WorktreeDir, ".ai", "definition_of_done.md"))
		assertExists(t, filepath.Join(r.WorktreeDir, "tests.md"))
		assertExists(t, inputRunManifestPath(r.LogsRoot))
	}
}

func newInputMaterializationRunConfigForTest(t *testing.T, repo string) *RunConfigFile {
	t.Helper()
	cfg := &RunConfigFile{Version: 1}
	cfg.Repo.Path = repo
	cfg.CXDB.BinaryAddr = "127.0.0.1:9"
	cfg.CXDB.HTTPBaseURL = "http://127.0.0.1:9"
	cfg.LLM.CLIProfile = "real"
	cfg.LLM.Providers = map[string]ProviderConfig{}
	cfg.ModelDB.OpenRouterModelInfoPath = writePinnedCatalog(t)
	cfg.ModelDB.OpenRouterModelInfoUpdatePolicy = "pinned"

	requireClean := false
	cfg.Git.RequireClean = &requireClean

	enabled := true
	follow := true
	infer := false
	cfg.Inputs.Materialize.Enabled = &enabled
	cfg.Inputs.Materialize.Include = nil
	cfg.Inputs.Materialize.DefaultInclude = []string{".ai/**"}
	cfg.Inputs.Materialize.FollowReferences = &follow
	cfg.Inputs.Materialize.InferWithLLM = &infer
	return cfg
}
