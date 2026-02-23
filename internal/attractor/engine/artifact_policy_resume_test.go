package engine

import (
	"testing"

	"github.com/danshapiro/kilroy/internal/attractor/runtime"
)

func TestResume_RestoreArtifactPolicy_UsesCheckpointSnapshot(t *testing.T) {
	cp := runtime.NewCheckpoint()
	cp.Extra = map[string]any{
		"artifact_policy_resolved": map[string]any{
			"profiles": []any{"rust"},
			"env":      map[string]any{"vars": map[string]any{"CARGO_TARGET_DIR": "/tmp/policy-target"}},
		},
	}
	cfg := validMinimalRunConfigForTest()
	rp, err := restoreArtifactPolicyForResume(cp, cfg, ResolveArtifactPolicyInput{LogsRoot: t.TempDir()})
	if err != nil {
		t.Fatalf("restoreArtifactPolicyForResume: %v", err)
	}
	if got := rp.Env.Vars["CARGO_TARGET_DIR"]; got != "/tmp/policy-target" {
		t.Fatalf("CARGO_TARGET_DIR=%q want /tmp/policy-target", got)
	}
}

func TestResume_RestoreArtifactPolicy_FallsBackToResolverWhenSnapshotMissing(t *testing.T) {
	cp := runtime.NewCheckpoint() // no artifact_policy_resolved in Extra
	cfg := validMinimalRunConfigForTest()
	cfg.ArtifactPolicy.Profiles = []string{"rust"}
	rp, err := restoreArtifactPolicyForResume(cp, cfg, ResolveArtifactPolicyInput{LogsRoot: t.TempDir()})
	if err != nil {
		t.Fatalf("restoreArtifactPolicyForResume: %v", err)
	}
	if len(rp.Profiles) == 0 {
		t.Fatal("expected resolver fallback to populate artifact policy from run config")
	}
}
