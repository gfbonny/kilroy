package engine

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/strongdm/kilroy/internal/attractor/runtime"
)

func TestFailureRouting_FanInAllFail_DoesNotFollowUnconditionalEdge(t *testing.T) {
	repo := t.TempDir()
	runCmd(t, repo, "git", "init")
	runCmd(t, repo, "git", "config", "user.name", "tester")
	runCmd(t, repo, "git", "config", "user.email", "tester@example.com")
	_ = os.WriteFile(filepath.Join(repo, "README.md"), []byte("hello\n"), 0o644)
	runCmd(t, repo, "git", "add", "-A")
	runCmd(t, repo, "git", "commit", "-m", "init")

	dot := []byte(`
digraph G {
  graph [goal="fanin-fail-routing", default_max_retry=2]
  start [shape=Mdiamond]
  par [shape=component]
  a [shape=parallelogram, tool_command="echo fail-a >&2; exit 1"]
  b [shape=parallelogram, tool_command="echo fail-b >&2; exit 1"]
  c [shape=parallelogram, tool_command="echo fail-c >&2; exit 1"]
  join [shape=tripleoctagon, max_retries=2]
  verify [shape=parallelogram, tool_command="echo verify > verify.txt"]
  exit [shape=Msquare]

  start -> par
  par -> a
  par -> b
  par -> c
  a -> join
  b -> join
  c -> join
  join -> verify
  verify -> exit
}
`)

	logsRoot := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	res, err := Run(ctx, dot, RunOptions{
		RepoPath: repo,
		RunID:    "fanin-all-fail-no-fallthrough",
		LogsRoot: logsRoot,
	})
	if err == nil {
		t.Fatalf("expected terminal failure at join with no fail path; got success result=%+v", res)
	}

	joinStatusBytes, readErr := os.ReadFile(filepath.Join(logsRoot, "join", "status.json"))
	if readErr != nil {
		t.Fatalf("read join/status.json: %v", readErr)
	}
	joinOut, decodeErr := runtime.DecodeOutcomeJSON(joinStatusBytes)
	if decodeErr != nil {
		t.Fatalf("decode join/status.json: %v", decodeErr)
	}
	if joinOut.Status != runtime.StatusFail {
		t.Fatalf("join status: got %q want %q", joinOut.Status, runtime.StatusFail)
	}
	if !strings.Contains(strings.ToLower(joinOut.FailureReason), "all parallel branches failed") {
		t.Fatalf("join failure_reason: got %q, want phrase %q", joinOut.FailureReason, "all parallel branches failed")
	}

	if _, statErr := os.Stat(filepath.Join(logsRoot, "verify", "status.json")); !os.IsNotExist(statErr) {
		t.Fatalf("verify node should not execute after fan-in all-fail; stat err=%v", statErr)
	}
	if fanInEdgeWasSelected(t, filepath.Join(logsRoot, "progress.ndjson"), "join", "verify") {
		t.Fatalf("unexpected edge_selected join->verify after fan-in all-fail")
	}

	final := mustReadFinalOutcome(t, filepath.Join(logsRoot, "final.json"))
	if final.Status != runtime.FinalFail {
		t.Fatalf("final status: got %q want %q", final.Status, runtime.FinalFail)
	}
	if strings.TrimSpace(final.FailureReason) == "" {
		t.Fatalf("expected non-empty final failure_reason")
	}
}

func fanInEdgeWasSelected(t *testing.T, progressPath, from, to string) bool {
	t.Helper()
	b, err := os.ReadFile(progressPath)
	if err != nil {
		t.Fatalf("read progress %s: %v", progressPath, err)
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var ev map[string]any
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			t.Fatalf("decode progress row %q: %v", line, err)
		}
		if strings.TrimSpace(anyToString(ev["event"])) != "edge_selected" {
			continue
		}
		if strings.TrimSpace(anyToString(ev["from_node"])) == strings.TrimSpace(from) &&
			strings.TrimSpace(anyToString(ev["to_node"])) == strings.TrimSpace(to) {
			return true
		}
	}
	return false
}

func mustReadFinalOutcome(t *testing.T, path string) runtime.FinalOutcome {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read final outcome %s: %v", path, err)
	}
	var out runtime.FinalOutcome
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("decode final outcome %s: %v", path, err)
	}
	return out
}
