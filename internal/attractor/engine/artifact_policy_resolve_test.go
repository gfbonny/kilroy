package engine

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveArtifactPolicy_RelativeManagedRootsUseLogsRoot(t *testing.T) {
	logsRoot := t.TempDir()
	cfg := validMinimalRunConfigForTest()
	cfg.ArtifactPolicy.Profiles = []string{"rust"}
	cfg.ArtifactPolicy.Env.ManagedRoots = map[string]string{"tool_cache_root": "managed"}

	rp, err := ResolveArtifactPolicy(cfg, ResolveArtifactPolicyInput{LogsRoot: logsRoot})
	if err != nil {
		t.Fatal(err)
	}
	if got := rp.ManagedRoots["tool_cache_root"]; !strings.HasPrefix(got, filepath.Join(logsRoot, "policy-managed-roots")) {
		t.Fatalf("tool_cache_root=%q not under logs root policy-managed-roots", got)
	}
}

func TestResolveArtifactPolicy_OSOverridesProfileDefaults(t *testing.T) {
	t.Setenv("CARGO_TARGET_DIR", "/tmp/from-os")
	cfg := validMinimalRunConfigForTest()
	cfg.ArtifactPolicy.Profiles = []string{"rust"}

	rp, err := ResolveArtifactPolicy(cfg, ResolveArtifactPolicyInput{LogsRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if got := rp.Env.Vars["CARGO_TARGET_DIR"]; got != "/tmp/from-os" {
		t.Fatalf("CARGO_TARGET_DIR=%q want /tmp/from-os", got)
	}
}

func TestResolveArtifactPolicy_CheckpointExcludesMirrorConfig(t *testing.T) {
	cfg := validMinimalRunConfigForTest()
	cfg.ArtifactPolicy.Checkpoint.ExcludeGlobs = []string{"**/.cargo-target*/**"}
	rp, err := ResolveArtifactPolicy(cfg, ResolveArtifactPolicyInput{LogsRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if len(rp.Checkpoint.ExcludeGlobs) != 1 || rp.Checkpoint.ExcludeGlobs[0] != "**/.cargo-target*/**" {
		t.Fatalf("checkpoint excludes mismatch: %+v", rp.Checkpoint.ExcludeGlobs)
	}
}
