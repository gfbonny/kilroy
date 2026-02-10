package engine

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestRun_StallWatchdog(t *testing.T) {
	if _, err := exec.LookPath("sleep"); err != nil {
		t.Skip("requires sleep binary")
	}
	dot := []byte(`digraph G {
  start [shape=Mdiamond]
  wait [shape=parallelogram, tool_command="sleep 2"]
  exit [shape=Msquare]
  start -> wait
  wait -> exit [condition="outcome=success"]
}`)
	repo := initTestRepo(t)
	opts := RunOptions{
		RepoPath:           repo,
		StallTimeout:       150 * time.Millisecond,
		StallCheckInterval: 25 * time.Millisecond,
	}
	_, err := Run(context.Background(), dot, opts)
	if err == nil {
		t.Fatal("expected stall watchdog timeout")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "stall watchdog") {
		t.Fatalf("expected stall watchdog error, got: %v", err)
	}
}
