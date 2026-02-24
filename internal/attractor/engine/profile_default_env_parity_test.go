package engine

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

type profileDefaultEnvFile struct {
	Version  int                                      `yaml:"version"`
	Profiles map[string]profileDefaultEnvFileEntry `yaml:"profiles"`
}

type profileDefaultEnvFileEntry struct {
	Env map[string]string `yaml:"env"`
}

func findRepoRootFromEngine(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root (no go.mod)")
		}
		dir = parent
	}
}

func loadProfileDefaultEnvFile(t *testing.T) profileDefaultEnvFile {
	t.Helper()
	repoRoot := findRepoRootFromEngine(t)
	p := filepath.Join(repoRoot, "skills", "shared", "profile_default_env.yaml")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read profile_default_env.yaml: %v", err)
	}
	var file profileDefaultEnvFile
	if err := yaml.Unmarshal(b, &file); err != nil {
		t.Fatalf("parse profile_default_env.yaml: %v", err)
	}
	return file
}

func TestProfileDefaultEnv_MatchesEngine(t *testing.T) {
	file := loadProfileDefaultEnvFile(t)

	if file.Version != 1 {
		t.Fatalf("profile_default_env.yaml version=%d, want 1", file.Version)
	}

	// Every engine profile must be in the YAML.
	for profile, engineEnv := range profileDefaultEnv {
		yamlEntry, ok := file.Profiles[profile]
		if !ok {
			t.Errorf("engine profile %q missing from profile_default_env.yaml", profile)
			continue
		}
		yamlEnv := yamlEntry.Env
		if yamlEnv == nil {
			yamlEnv = map[string]string{}
		}
		for k, v := range engineEnv {
			got, ok := yamlEnv[k]
			if !ok {
				t.Errorf("profile %q: engine env var %q missing from YAML", profile, k)
				continue
			}
			if got != v {
				t.Errorf("profile %q: env var %q = %q in YAML, want %q", profile, k, got, v)
			}
		}
		for k := range yamlEnv {
			if _, ok := engineEnv[k]; !ok {
				t.Errorf("profile %q: YAML has extra env var %q not in engine", profile, k)
			}
		}
	}

	// Every YAML profile must be in the engine.
	for profile := range file.Profiles {
		if _, ok := profileDefaultEnv[profile]; !ok {
			t.Errorf("YAML profile %q not present in engine profileDefaultEnv", profile)
		}
	}
}

func TestProfileDefaultEnv_MatchesAllowedProfiles(t *testing.T) {
	// Ensure profileDefaultEnv and allowedArtifactPolicyProfiles stay in sync.
	for profile := range profileDefaultEnv {
		if _, ok := allowedArtifactPolicyProfiles[profile]; !ok {
			t.Errorf("profileDefaultEnv has profile %q not in allowedArtifactPolicyProfiles", profile)
		}
	}
	for profile := range allowedArtifactPolicyProfiles {
		if _, ok := profileDefaultEnv[profile]; !ok {
			t.Errorf("allowedArtifactPolicyProfiles has profile %q not in profileDefaultEnv", profile)
		}
	}
}
