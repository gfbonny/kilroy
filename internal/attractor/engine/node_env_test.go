package engine

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildBaseNodeEnv_PreservesToolchainPaths(t *testing.T) {
	home := t.TempDir()
	cargoHome := filepath.Join(home, ".cargo")
	rustupHome := filepath.Join(home, ".rustup")
	gopath := filepath.Join(home, "go")

	t.Setenv("HOME", home)
	t.Setenv("CARGO_HOME", cargoHome)
	t.Setenv("RUSTUP_HOME", rustupHome)
	t.Setenv("GOPATH", gopath)

	worktree := t.TempDir()
	env := buildBaseNodeEnv(worktree)

	if got := envLookup(env, "CARGO_HOME"); got != cargoHome {
		t.Fatalf("CARGO_HOME: got %q want %q", got, cargoHome)
	}
	if got := envLookup(env, "RUSTUP_HOME"); got != rustupHome {
		t.Fatalf("RUSTUP_HOME: got %q want %q", got, rustupHome)
	}
	if got := envLookup(env, "GOPATH"); got != gopath {
		t.Fatalf("GOPATH: got %q want %q", got, gopath)
	}
	if got := envLookup(env, "CARGO_TARGET_DIR"); got != filepath.Join(worktree, ".cargo-target") {
		t.Fatalf("CARGO_TARGET_DIR: got %q want %q", got, filepath.Join(worktree, ".cargo-target"))
	}
}

func TestBuildBaseNodeEnv_InfersGoPathsFromHOME(t *testing.T) {
	// When GOPATH/GOMODCACHE are not set, Go defaults them to $HOME/go
	// and $HOME/go/pkg/mod. buildBaseNodeEnv should pin them explicitly
	// so that later HOME overrides (codex isolation) don't break Go
	// toolchain resolution.
	home := t.TempDir()
	t.Setenv("HOME", home)
	os.Unsetenv("GOPATH")
	os.Unsetenv("GOMODCACHE")

	worktree := t.TempDir()
	env := buildBaseNodeEnv(worktree)

	if got := envLookup(env, "GOPATH"); got != filepath.Join(home, "go") {
		t.Fatalf("GOPATH: got %q want %q", got, filepath.Join(home, "go"))
	}
	if got := envLookup(env, "GOMODCACHE"); got != filepath.Join(home, "go", "pkg", "mod") {
		t.Fatalf("GOMODCACHE: got %q want %q", got, filepath.Join(home, "go", "pkg", "mod"))
	}
}

func TestBuildBaseNodeEnv_SetsCargoTargetDirToWorktree(t *testing.T) {
	worktree := t.TempDir()
	env := buildBaseNodeEnv(worktree)

	got := envLookup(env, "CARGO_TARGET_DIR")
	want := filepath.Join(worktree, ".cargo-target")
	if got != want {
		t.Fatalf("CARGO_TARGET_DIR: got %q want %q", got, want)
	}
}

func TestBuildBaseNodeEnv_DoesNotOverrideExplicitCargoTargetDir(t *testing.T) {
	t.Setenv("CARGO_TARGET_DIR", "/custom/target")
	worktree := t.TempDir()
	env := buildBaseNodeEnv(worktree)

	got := envLookup(env, "CARGO_TARGET_DIR")
	if got != "/custom/target" {
		t.Fatalf("CARGO_TARGET_DIR: got %q want %q (should not override explicit)", got, "/custom/target")
	}
}

func TestBuildBaseNodeEnv_InfersToolchainPathsFromHOME(t *testing.T) {
	// When CARGO_HOME/RUSTUP_HOME are not set, they default to $HOME/.cargo and $HOME/.rustup.
	// buildBaseNodeEnv should set them explicitly so downstream HOME overrides don't break them.
	home := t.TempDir()
	t.Setenv("HOME", home)
	os.Unsetenv("CARGO_HOME")
	os.Unsetenv("RUSTUP_HOME")

	worktree := t.TempDir()
	env := buildBaseNodeEnv(worktree)

	if got := envLookup(env, "CARGO_HOME"); got != filepath.Join(home, ".cargo") {
		t.Fatalf("CARGO_HOME: got %q want %q", got, filepath.Join(home, ".cargo"))
	}
	if got := envLookup(env, "RUSTUP_HOME"); got != filepath.Join(home, ".rustup") {
		t.Fatalf("RUSTUP_HOME: got %q want %q", got, filepath.Join(home, ".rustup"))
	}
}

func TestBuildBaseNodeEnv_StripsClaudeCode(t *testing.T) {
	t.Setenv("CLAUDECODE", "1")
	worktree := t.TempDir()
	env := buildBaseNodeEnv(worktree)

	if envHasKey(env, "CLAUDECODE") {
		t.Fatal("CLAUDECODE should be stripped from base env")
	}
}

func TestToolHandler_UsesBaseNodeEnv(t *testing.T) {
	// A tool node should see pinned toolchain env vars and have CLAUDECODE stripped.
	// We can verify by running a tool_command that echoes env vars.
	t.Setenv("CLAUDECODE", "1")

	dot := []byte(`digraph G {
  graph [goal="test"]
  start [shape=Mdiamond]
  exit [shape=Msquare]
  check [shape=parallelogram, tool_command="bash -c 'echo CLAUDECODE=$CLAUDECODE; echo CARGO_TARGET_DIR=$CARGO_TARGET_DIR'"]
  start -> check -> exit
}`)
	repo := initTestRepo(t)
	logsRoot := t.TempDir()
	result, err := Run(context.Background(), dot, RunOptions{RepoPath: repo, LogsRoot: logsRoot})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.FinalStatus != "success" {
		t.Fatalf("expected success, got %s", result.FinalStatus)
	}

	// Read stdout to verify env was set correctly.
	stdout, err := os.ReadFile(filepath.Join(logsRoot, "check", "stdout.log"))
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	output := string(stdout)
	if strings.Contains(output, "CLAUDECODE=1") {
		t.Fatal("CLAUDECODE should be stripped from tool node env")
	}
	if !strings.Contains(output, "CARGO_TARGET_DIR=") {
		t.Fatal("CARGO_TARGET_DIR should be set in tool node env")
	}
}
