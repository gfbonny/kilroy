package engine

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/strongdm/kilroy/internal/attractor/model"
	"github.com/strongdm/kilroy/internal/attractor/runtime"
)

func TestRun_LoopRestartCreatesNewLogDirectory(t *testing.T) {
	repo := t.TempDir()
	runCmd(t, repo, "git", "init")
	runCmd(t, repo, "git", "config", "user.name", "tester")
	runCmd(t, repo, "git", "config", "user.email", "tester@example.com")
	_ = os.WriteFile(filepath.Join(repo, "README.md"), []byte("hello\n"), 0o644)
	runCmd(t, repo, "git", "add", "-A")
	runCmd(t, repo, "git", "commit", "-m", "init")

	// Graph: start -> work -> check
	//   check -> exit [condition="outcome=success"]
	//   check -> work [condition="outcome=fail", loop_restart=true]
	//
	// The backend returns fail on the first call to "work", success on the second.
	dot := []byte(`
digraph G {
  graph [goal="test loop restart"]
  start [shape=Mdiamond]
  exit  [shape=Msquare]
  work  [shape=box, llm_provider=openai, llm_model=gpt-5.2, prompt="do work"]
  check [shape=diamond]
  start -> work
  work -> check
  check -> exit [condition="outcome=success"]
  check -> work [condition="outcome=fail", loop_restart=true]
}
`)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var callCount atomic.Int32
	backend := &countingBackend{
		fn: func(ctx context.Context, exec *Execution, node *model.Node, prompt string) (string, *runtime.Outcome, error) {
			n := callCount.Add(1)
			if node.ID == "work" && n == 1 {
				return "fail", &runtime.Outcome{Status: runtime.StatusFail, FailureReason: "first try"}, nil
			}
			return "ok", &runtime.Outcome{Status: runtime.StatusSuccess}, nil
		},
	}

	g, _, err := Prepare(dot)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}

	logsRoot := t.TempDir()
	eng := &Engine{
		Graph:           g,
		Options:         RunOptions{RepoPath: repo, RunID: "test-restart", LogsRoot: logsRoot, WorktreeDir: filepath.Join(logsRoot, "worktree"), RunBranchPrefix: "attractor/run", RequireClean: true},
		DotSource:       dot,
		LogsRoot:        logsRoot,
		WorktreeDir:     filepath.Join(logsRoot, "worktree"),
		Context:         runtime.NewContext(),
		Registry:        NewDefaultRegistry(),
		Interviewer:     &AutoApproveInterviewer{},
		CodergenBackend: backend,
	}
	eng.RunBranch = "attractor/run/test-restart"

	res, err := eng.run(ctx)
	if err != nil {
		t.Fatalf("run() error: %v", err)
	}
	if res.FinalStatus != runtime.FinalSuccess {
		t.Fatalf("FinalStatus = %v, want success", res.FinalStatus)
	}

	// Verify the backend was called twice for "work" (once per iteration).
	if got := callCount.Load(); got < 2 {
		t.Fatalf("backend call count = %d, want >= 2", got)
	}

	// Verify a restart directory was created.
	restartDir := filepath.Join(logsRoot, "restart-1")
	if _, err := os.Stat(restartDir); err != nil {
		t.Fatalf("expected restart-1 directory to exist: %v", err)
	}

	// Verify manifest.json exists in the restart directory (review fix: metadata in restart dirs).
	manifestPath := filepath.Join(restartDir, "manifest.json")
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("expected manifest.json in restart dir: %v", err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatalf("invalid manifest.json: %v", err)
	}
	if manifest["run_id"] != "test-restart" {
		t.Errorf("manifest run_id = %v, want %q", manifest["run_id"], "test-restart")
	}

	// Verify context was reset on restart (review fix: no stale context bleed).
	// After a successful restart, context should have graph-level attrs but NOT
	// node outcomes from the first (failed) iteration.
	if _, found := eng.Context.Get("node.work.outcome"); found {
		t.Error("stale node outcome leaked across restart boundary")
	}
}

func TestRun_LoopRestartLimitExceeded(t *testing.T) {
	repo := t.TempDir()
	runCmd(t, repo, "git", "init")
	runCmd(t, repo, "git", "config", "user.name", "tester")
	runCmd(t, repo, "git", "config", "user.email", "tester@example.com")
	_ = os.WriteFile(filepath.Join(repo, "README.md"), []byte("hello\n"), 0o644)
	runCmd(t, repo, "git", "add", "-A")
	runCmd(t, repo, "git", "commit", "-m", "init")

	// Always fail, with max_restarts=2 so we hit the limit quickly.
	dot := []byte(`
digraph G {
  graph [goal="test limit", max_restarts="2"]
  start [shape=Mdiamond]
  exit  [shape=Msquare]
  work  [shape=box, llm_provider=openai, llm_model=gpt-5.2, prompt="do work"]
  check [shape=diamond]
  start -> work
  work -> check
  check -> exit [condition="outcome=success"]
  check -> work [condition="outcome=fail", loop_restart=true]
}
`)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	backend := &countingBackend{
		fn: func(ctx context.Context, exec *Execution, node *model.Node, prompt string) (string, *runtime.Outcome, error) {
			return "fail", &runtime.Outcome{Status: runtime.StatusFail, FailureReason: "always fail"}, nil
		},
	}

	g, _, err := Prepare(dot)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}

	logsRoot := t.TempDir()
	eng := &Engine{
		Graph:           g,
		Options:         RunOptions{RepoPath: repo, RunID: "test-limit", LogsRoot: logsRoot, WorktreeDir: filepath.Join(logsRoot, "worktree"), RunBranchPrefix: "attractor/run", RequireClean: true},
		DotSource:       dot,
		LogsRoot:        logsRoot,
		WorktreeDir:     filepath.Join(logsRoot, "worktree"),
		Context:         runtime.NewContext(),
		Registry:        NewDefaultRegistry(),
		Interviewer:     &AutoApproveInterviewer{},
		CodergenBackend: backend,
	}
	eng.RunBranch = "attractor/run/test-limit"

	_, err = eng.run(ctx)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "loop_restart limit exceeded") {
		t.Fatalf("expected loop_restart limit error, got: %v", err)
	}
}

// countingBackend is a test backend with a configurable function.
type countingBackend struct {
	fn func(ctx context.Context, exec *Execution, node *model.Node, prompt string) (string, *runtime.Outcome, error)
}

func (b *countingBackend) Run(ctx context.Context, exec *Execution, node *model.Node, prompt string) (string, *runtime.Outcome, error) {
	return b.fn(ctx, exec, node, prompt)
}
