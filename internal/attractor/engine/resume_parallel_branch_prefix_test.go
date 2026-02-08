package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/strongdm/kilroy/internal/attractor/runtime"
)

func TestResume_ParallelBranchPrefixPreserved(t *testing.T) {
	repo := t.TempDir()
	runCmd(t, repo, "git", "init")
	runCmd(t, repo, "git", "config", "user.name", "tester")
	runCmd(t, repo, "git", "config", "user.email", "tester@example.com")
	_ = os.WriteFile(filepath.Join(repo, "README.md"), []byte("hello\n"), 0o644)
	runCmd(t, repo, "git", "add", "-A")
	runCmd(t, repo, "git", "commit", "-m", "init")

	logsRoot := t.TempDir()
	dot := []byte(`
digraph G {
  graph [goal="resume parallel prefix"]
  start [shape=Mdiamond]
  par [shape=component]
  a [shape=parallelogram, tool_command="echo a > a.txt"]
  b [shape=parallelogram, tool_command="echo b > b.txt"]
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
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	res, err := Run(ctx, dot, RunOptions{
		RepoPath:        repo,
		RunID:           "resume-prefix",
		LogsRoot:        logsRoot,
		RunBranchPrefix: "attractor/run",
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	startSHA := findCommitForNode(t, repo, res.RunBranch, res.RunID, "start")
	if startSHA == "" {
		t.Fatalf("missing start commit")
	}

	cpPath := filepath.Join(res.LogsRoot, "checkpoint.json")
	cp, err := runtime.LoadCheckpoint(cpPath)
	if err != nil {
		t.Fatalf("LoadCheckpoint: %v", err)
	}
	cp.CurrentNode = "start"
	cp.CompletedNodes = []string{"start"}
	cp.NodeRetries = map[string]int{}
	cp.GitCommitSHA = startSHA
	if err := cp.Save(cpPath); err != nil {
		t.Fatalf("Save checkpoint: %v", err)
	}

	if _, err := Resume(ctx, res.LogsRoot); err != nil {
		t.Fatalf("Resume() error: %v", err)
	}

	b, err := os.ReadFile(filepath.Join(res.LogsRoot, "par", "parallel_results.json"))
	if err != nil {
		t.Fatalf("read parallel_results.json: %v", err)
	}
	var results []map[string]any
	if err := json.Unmarshal(b, &results); err != nil {
		t.Fatalf("unmarshal parallel_results.json: %v", err)
	}
	if len(results) == 0 {
		t.Fatalf("parallel_results.json is empty")
	}
	wantPrefix := fmt.Sprintf("attractor/run/parallel/%s/", res.RunID)
	for _, r := range results {
		branchName := strings.TrimSpace(fmt.Sprint(r["branch_name"]))
		if strings.HasPrefix(branchName, "/parallel/") {
			t.Fatalf("invalid branch name with empty prefix: %q", branchName)
		}
		if !strings.HasPrefix(branchName, wantPrefix) {
			t.Fatalf("branch name %q missing prefix %q", branchName, wantPrefix)
		}
	}
}

func findCommitForNode(t *testing.T, repo string, branch string, runID string, nodeID string) string {
	t.Helper()
	log := runCmdOut(t, repo, "git", "log", "--format=%H:%s", branch)
	wantMsgPrefix := "attractor(" + runID + "): " + nodeID + " ("
	for _, line := range strings.Split(log, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		msg := strings.TrimSpace(parts[1])
		if strings.HasPrefix(msg, wantMsgPrefix) {
			return strings.TrimSpace(parts[0])
		}
	}
	return ""
}
