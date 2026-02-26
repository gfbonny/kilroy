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

func TestResolveArtifactPolicy_OSOverridesConfigOverrides(t *testing.T) {
	t.Setenv("CARGO_TARGET_DIR", "/tmp/from-os")
	cfg := validMinimalRunConfigForTest()
	cfg.ArtifactPolicy.Profiles = []string{"rust"}
	cfg.ArtifactPolicy.Env.Overrides = map[string]map[string]string{
		"rust": {"CARGO_TARGET_DIR": "{managed_roots.tool_cache_root}/cargo-target"},
	}

	rp, err := ResolveArtifactPolicy(cfg, ResolveArtifactPolicyInput{LogsRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if got := rp.Env.Vars["CARGO_TARGET_DIR"]; got != "/tmp/from-os" {
		t.Fatalf("CARGO_TARGET_DIR=%q want /tmp/from-os", got)
	}
}

func TestResolveArtifactPolicy_NoImplicitEnvWithoutOverrides(t *testing.T) {
	cfg := validMinimalRunConfigForTest()
	cfg.ArtifactPolicy.Profiles = []string{"rust"}
	// No env.overrides set — engine should NOT inject language-specific defaults.

	rp, err := ResolveArtifactPolicy(cfg, ResolveArtifactPolicyInput{LogsRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if len(rp.Env.Vars) != 0 {
		t.Fatalf("expected empty env vars without explicit overrides, got %+v", rp.Env.Vars)
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
