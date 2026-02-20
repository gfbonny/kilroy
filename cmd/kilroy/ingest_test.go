package main

import (
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"
)

func TestParseIngestArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
		check   func(*testing.T, *ingestOptions)
	}{
		{
			name:    "missing requirements",
			args:    []string{},
			wantErr: true,
		},
		{
			name: "requirements from positional arg",
			args: []string{"Build a solitaire game"},
			check: func(t *testing.T, o *ingestOptions) {
				if o.requirements != "Build a solitaire game" {
					t.Errorf("requirements = %q, want %q", o.requirements, "Build a solitaire game")
				}
			},
		},
		{
			name: "output flag",
			args: []string{"--output", "pipeline.dot", "Build a solitaire game"},
			check: func(t *testing.T, o *ingestOptions) {
				if o.outputPath != "pipeline.dot" {
					t.Errorf("outputPath = %q, want %q", o.outputPath, "pipeline.dot")
				}
			},
		},
		{
			name: "model flag",
			args: []string{"--model", "claude-opus-4-6", "Build a solitaire game"},
			check: func(t *testing.T, o *ingestOptions) {
				if o.model != "claude-opus-4-6" {
					t.Errorf("model = %q, want %q", o.model, "claude-opus-4-6")
				}
			},
		},
		{
			name: "skill flag",
			args: []string{"--skill", "/tmp/custom-skill.md", "Build a solitaire game"},
			check: func(t *testing.T, o *ingestOptions) {
				if o.skillPath != "/tmp/custom-skill.md" {
					t.Errorf("skillPath = %q, want %q", o.skillPath, "/tmp/custom-skill.md")
				}
			},
		},
		{
			name: "default model",
			args: []string{"Build a solitaire game"},
			check: func(t *testing.T, o *ingestOptions) {
				if o.model != "claude-sonnet-4-5" {
					t.Errorf("model = %q, want default %q", o.model, "claude-sonnet-4-5")
				}
			},
		},
		{
			name: "max-turns flag",
			args: []string{"--max-turns", "10", "Build a solitaire game"},
			check: func(t *testing.T, o *ingestOptions) {
				if o.maxTurns != 10 {
					t.Errorf("maxTurns = %d, want 10", o.maxTurns)
				}
			},
		},
		{
			name:    "max-turns missing value",
			args:    []string{"--max-turns"},
			wantErr: true,
		},
		{
			name:    "max-turns non-integer",
			args:    []string{"--max-turns", "abc", "Build a solitaire game"},
			wantErr: true,
		},
		{
			name:    "max-turns zero",
			args:    []string{"--max-turns", "0", "Build a solitaire game"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts, err := parseIngestArgs(tt.args)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseIngestArgs() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && tt.check != nil {
				tt.check(t, opts)
			}
		})
	}
}

func TestResolveDefaultIngestSkillPath_UsesBinaryRelativeDefaults(t *testing.T) {
	tmp := t.TempDir()
	binaryPath := filepath.Join(tmp, "bin", "kilroy")
	binarySkill := filepath.Join(tmp, "share", "kilroy", "skills", "english-to-dotfile", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(binaryPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binaryPath, []byte(""), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(binarySkill), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binarySkill, []byte("# skill"), 0o644); err != nil {
		t.Fatal(err)
	}

	old := osExecutable
	osExecutable = func() (string, error) { return binaryPath, nil }
	t.Cleanup(func() { osExecutable = old })

	got := resolveDefaultIngestSkillPath(filepath.Join(tmp, "repo-without-skill"))
	if canonicalPath(got) != canonicalPath(binarySkill) {
		t.Fatalf("resolveDefaultIngestSkillPath() = %q, want %q", got, binarySkill)
	}
}

func TestResolveDefaultIngestSkillPath_PrefersRepoSkill(t *testing.T) {
	tmp := t.TempDir()
	repoSkill := filepath.Join(tmp, "repo", "skills", "english-to-dotfile", "SKILL.md")
	binaryPath := filepath.Join(tmp, "bin", "kilroy")
	binarySkill := filepath.Join(tmp, "share", "kilroy", "skills", "english-to-dotfile", "SKILL.md")

	if err := os.MkdirAll(filepath.Dir(repoSkill), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(repoSkill, []byte("# repo skill"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(binaryPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binaryPath, []byte(""), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(binarySkill), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binarySkill, []byte("# binary skill"), 0o644); err != nil {
		t.Fatal(err)
	}

	old := osExecutable
	osExecutable = func() (string, error) { return binaryPath, nil }
	t.Cleanup(func() { osExecutable = old })

	got := resolveDefaultIngestSkillPath(filepath.Join(tmp, "repo"))
	if canonicalPath(got) != canonicalPath(repoSkill) {
		t.Fatalf("resolveDefaultIngestSkillPath() = %q, want %q", got, repoSkill)
	}
}

func TestResolveDefaultIngestSkillPath_UsesGoInstallModuleCacheFallback(t *testing.T) {
	tmp := t.TempDir()
	moduleDir := filepath.Join(tmp, "pkg", "mod", "github.com", "danshapiro", "kilroy@v1.2.3")
	moduleSkill := filepath.Join(moduleDir, "skills", "english-to-dotfile", "SKILL.md")
	binaryPath := filepath.Join(tmp, "bin", "kilroy")

	if err := os.MkdirAll(filepath.Dir(moduleSkill), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(moduleSkill, []byte("# module-cache skill"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(binaryPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binaryPath, []byte(""), 0o755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("GOMODCACHE", filepath.Join(tmp, "pkg", "mod"))
	oldExe := osExecutable
	osExecutable = func() (string, error) { return binaryPath, nil }
	t.Cleanup(func() { osExecutable = oldExe })

	oldBuildInfo := readBuildInfo
	readBuildInfo = func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{Main: debug.Module{Path: "github.com/danshapiro/kilroy", Version: "v1.2.3"}}, true
	}
	t.Cleanup(func() { readBuildInfo = oldBuildInfo })

	got := resolveDefaultIngestSkillPath(filepath.Join(tmp, "repo-without-skill"))
	if canonicalPath(got) != canonicalPath(moduleSkill) {
		t.Fatalf("resolveDefaultIngestSkillPath() = %q, want %q", got, moduleSkill)
	}
}

func TestResolveDefaultIngestSkillPath_UsesModuleCacheWhenBuildVersionIsDevel(t *testing.T) {
	tmp := t.TempDir()
	moduleDir := filepath.Join(tmp, "pkg", "mod", "github.com", "danshapiro", "kilroy@v0.0.0-20260219062932-c5e2760d4aae")
	moduleSkill := filepath.Join(moduleDir, "skills", "english-to-dotfile", "SKILL.md")
	binaryPath := filepath.Join(tmp, "bin", "kilroy")

	if err := os.MkdirAll(filepath.Dir(moduleSkill), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(moduleSkill, []byte("# module-cache skill"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(binaryPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binaryPath, []byte(""), 0o755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("GOMODCACHE", filepath.Join(tmp, "pkg", "mod"))
	oldExe := osExecutable
	osExecutable = func() (string, error) { return binaryPath, nil }
	t.Cleanup(func() { osExecutable = oldExe })

	oldBuildInfo := readBuildInfo
	readBuildInfo = func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{Main: debug.Module{Path: "github.com/danshapiro/kilroy", Version: "(devel)"}}, true
	}
	t.Cleanup(func() { readBuildInfo = oldBuildInfo })

	got := resolveDefaultIngestSkillPath(filepath.Join(tmp, "repo-without-skill"))
	if canonicalPath(got) != canonicalPath(moduleSkill) {
		t.Fatalf("resolveDefaultIngestSkillPath() = %q, want %q", got, moduleSkill)
	}
}

func canonicalPath(p string) string {
	p = filepath.Clean(p)
	if abs, err := filepath.Abs(p); err == nil {
		p = abs
	}
	if resolved, err := filepath.EvalSymlinks(p); err == nil {
		return filepath.Clean(resolved)
	}
	return filepath.Clean(p)
}

func TestRunIngest_EmptySkillPathReturnsHelpfulError(t *testing.T) {
	_, err := runIngest(&ingestOptions{
		requirements: "Build a solitaire game",
		repoPath:     t.TempDir(),
		model:        "claude-sonnet-4-5",
		validate:     true,
	})
	if err == nil {
		t.Fatal("expected error for missing default skill")
	}
	if got := err.Error(); !containsAll(got, "no default skill file found", "--skill") {
		t.Fatalf("unexpected error: %q", got)
	}
}

func containsAll(s string, needles ...string) bool {
	for _, n := range needles {
		if !strings.Contains(s, n) {
			return false
		}
	}
	return true
}
