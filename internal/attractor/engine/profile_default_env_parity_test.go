package engine

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

type profileDefaultEnvFile struct {
	Version  int                        `yaml:"version"`
	Profiles map[string]map[string]string `yaml:"profiles"`
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

func TestProfileDefaultEnv_VersionAndStructure(t *testing.T) {
	file := loadProfileDefaultEnvFile(t)

	if file.Version != 1 {
		t.Fatalf("profile_default_env.yaml version=%d, want 1", file.Version)
	}
	if len(file.Profiles) == 0 {
		t.Fatal("profile_default_env.yaml has no profiles")
	}

	// Every env value must use {managed_roots.*} templates or be empty.
	for profile, envVars := range file.Profiles {
		for k, v := range envVars {
			if k == "" {
				t.Errorf("profile %q: empty env var key", profile)
			}
			if v == "" {
				t.Errorf("profile %q: empty value for env var %q", profile, k)
			}
		}
	}
}

func TestProfileDefaultEnv_ProfilesMatchAllowedSet(t *testing.T) {
	file := loadProfileDefaultEnvFile(t)

	// Every YAML profile must be in allowedArtifactPolicyProfiles.
	for profile := range file.Profiles {
		if _, ok := allowedArtifactPolicyProfiles[profile]; !ok {
			t.Errorf("YAML profile %q not in allowedArtifactPolicyProfiles", profile)
		}
	}
	// Every allowed profile must be in the YAML.
	for profile := range allowedArtifactPolicyProfiles {
		if _, ok := file.Profiles[profile]; !ok {
			t.Errorf("allowedArtifactPolicyProfiles has profile %q missing from profile_default_env.yaml", profile)
		}
	}
}
