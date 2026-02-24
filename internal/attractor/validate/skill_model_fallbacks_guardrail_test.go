package validate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func loadSkillFile(t *testing.T, parts ...string) string {
	t.Helper()
	repoRoot := findRepoRoot(t)
	p := filepath.Join(append([]string{repoRoot}, parts...)...)
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read %s: %v", p, err)
	}
	return string(b)
}

func TestSkillModelFallbacksFile_ContainsZAIModelAndAlias(t *testing.T) {
	text := loadSkillFile(t, "skills", "shared", "model_fallbacks.yaml")
	required := []string{
		"providers:",
		"zai:",
		"canonical_model: glm-5",
		"glm-5.0",
		"openrouter_model: z-ai/glm-5",
	}
	for _, needle := range required {
		if !strings.Contains(text, needle) {
			t.Fatalf("skills/shared/model_fallbacks.yaml missing %q", needle)
		}
	}
}

func TestCreateDotfileSkill_UsesSharedModelFallbacks(t *testing.T) {
	text := loadSkillFile(t, "skills", "create-dotfile", "SKILL.md")
	required := []string{
		"skills/shared/model_fallbacks.yaml",
		"glm-5.0` -> `glm-5`",
		"Never silently downgrade or substitute an explicit model request",
	}
	for _, needle := range required {
		if !strings.Contains(text, needle) {
			t.Fatalf("skills/create-dotfile/SKILL.md missing %q", needle)
		}
	}
}

func TestCreateRunfileSkill_UsesSharedModelFallbacks(t *testing.T) {
	text := loadSkillFile(t, "skills", "create-runfile", "SKILL.md")
	required := []string{
		"skills/shared/model_fallbacks.yaml",
		"glm-5.0` -> `glm-5`",
		"Model resolution order:",
	}
	for _, needle := range required {
		if !strings.Contains(text, needle) {
			t.Fatalf("skills/create-runfile/SKILL.md missing %q", needle)
		}
	}
}
