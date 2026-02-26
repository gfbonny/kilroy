package validate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAttractorSpec_IncludesInputMaterializationNormativeContract(t *testing.T) {
	repoRoot := findRepoRoot(t)
	specPath := filepath.Join(repoRoot, "docs", "strongdm", "attractor", "attractor-spec.md")
	b, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read attractor-spec.md: %v", err)
	}
	text := strings.ToLower(string(b))

	required := []string{
		"input materialization",
		"transitive reference closure",
		"run and branch worktree hydration",
		"inputs.materialize.default_include",
		"inputs.materialize.include",
		"fail-on-unmatched",
		"best-effort",
	}
	for _, needle := range required {
		if !strings.Contains(text, strings.ToLower(needle)) {
			t.Fatalf("attractor-spec.md must include normative input materialization phrase %q", needle)
		}
	}
}
