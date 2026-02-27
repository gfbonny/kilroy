package engine

// failure_dossier_patterns_test.go
//
// Regression tests for the regex patterns in failure_dossier.go:
//   - toolMissingPathPatterns  (4 regexes)
//   - toolMissingExecutablePattern (1 regex)
//
// Each pattern gets at least one "should match" and one "should NOT match"
// case so that future rewrites that break a pattern surface immediately.
// The captured group (submatch[1]) for matching cases is also verified.

import (
	"testing"
)

// ─── toolMissingPathPatterns ──────────────────────────────────────────────────

func TestToolMissingPathPatterns_Pattern0_CdNoSuchFile(t *testing.T) {
	// Pattern 0: `(?mi)\bcd:\s+([^:\r\n]+):\s+No such file or directory`
	re := toolMissingPathPatterns[0]

	matchCases := []struct {
		input   string
		wantCap string // expected submatch[1]
	}{
		{
			input:   "bash: line 3: cd: missing/bootstrap: No such file or directory",
			wantCap: "missing/bootstrap",
		},
		{
			input:   "cd: /nonexistent/path: No such file or directory",
			wantCap: "/nonexistent/path",
		},
		{
			// multi-line: pattern has (?mi) so it should match the relevant line
			input:   "some preamble\ncd: src/generated: No such file or directory\nsome suffix",
			wantCap: "src/generated",
		},
	}
	for _, tc := range matchCases {
		m := re.FindStringSubmatch(tc.input)
		if len(m) < 2 {
			t.Errorf("pattern[0] did not match %q; expected capture %q", tc.input, tc.wantCap)
			continue
		}
		if got := m[1]; got != tc.wantCap {
			t.Errorf("pattern[0] capture for %q: got %q, want %q", tc.input, got, tc.wantCap)
		}
	}

	noMatchCases := []string{
		// Missing the "cd:" prefix
		"No such file or directory",
		// Path has a colon in it (the regex stops at colon) — but that still
		// extracts something, so test a line with no "No such file" part
		"cd: something: permission denied",
		// Empty line
		"",
	}
	for _, s := range noMatchCases {
		if re.MatchString(s) {
			t.Errorf("pattern[0] should NOT match %q", s)
		}
	}
}

func TestToolMissingPathPatterns_Pattern1_TestNoSuchFile(t *testing.T) {
	// Pattern 1: `(?mi)\btest:\s+([^:\r\n]+):\s+No such file or directory`
	re := toolMissingPathPatterns[1]

	matchCases := []struct {
		input   string
		wantCap string
	}{
		{
			input:   "test: build/output: No such file or directory",
			wantCap: "build/output",
		},
		{
			input:   "bash: test: /tmp/missing_file: No such file or directory",
			wantCap: "/tmp/missing_file",
		},
	}
	for _, tc := range matchCases {
		m := re.FindStringSubmatch(tc.input)
		if len(m) < 2 {
			t.Errorf("pattern[1] did not match %q; expected capture %q", tc.input, tc.wantCap)
			continue
		}
		if got := m[1]; got != tc.wantCap {
			t.Errorf("pattern[1] capture for %q: got %q, want %q", tc.input, got, tc.wantCap)
		}
	}

	noMatchCases := []string{
		"cd: missing/path: No such file or directory", // cd not test
		"test: something: permission denied",
		"No such file or directory",
		"",
	}
	for _, s := range noMatchCases {
		if re.MatchString(s) {
			t.Errorf("pattern[1] should NOT match %q", s)
		}
	}
}

func TestToolMissingPathPatterns_Pattern2_CannotStatOpen(t *testing.T) {
	// Pattern 2: `(?mi)\bcannot (?:stat|open)\s+'([^'\r\n]+)'`
	re := toolMissingPathPatterns[2]

	matchCases := []struct {
		input   string
		wantCap string
	}{
		{
			input:   "ls: cannot stat '/some/missing/path': No such file or directory",
			wantCap: "/some/missing/path",
		},
		{
			input:   "cannot open 'src/lib.rs': No such file or directory",
			wantCap: "src/lib.rs",
		},
		{
			input:   "error: cannot stat 'build/output.o'",
			wantCap: "build/output.o",
		},
	}
	for _, tc := range matchCases {
		m := re.FindStringSubmatch(tc.input)
		if len(m) < 2 {
			t.Errorf("pattern[2] did not match %q; expected capture %q", tc.input, tc.wantCap)
			continue
		}
		if got := m[1]; got != tc.wantCap {
			t.Errorf("pattern[2] capture for %q: got %q, want %q", tc.input, got, tc.wantCap)
		}
	}

	noMatchCases := []string{
		// Path not quoted with single quotes
		"cannot stat /some/path: No such file or directory",
		// Different verb
		"cannot read '/some/path'",
		"",
	}
	for _, s := range noMatchCases {
		if re.MatchString(s) {
			t.Errorf("pattern[2] should NOT match %q", s)
		}
	}
}

func TestToolMissingPathPatterns_Pattern3_OpenNoSuchFile(t *testing.T) {
	// Pattern 3: `(?mi)\bopen\s+([^:\r\n]+):\s+no such file or directory`
	re := toolMissingPathPatterns[3]

	matchCases := []struct {
		input   string
		wantCap string
	}{
		{
			input:   "open /etc/missing.conf: no such file or directory",
			wantCap: "/etc/missing.conf",
		},
		{
			input:   "error: open config/settings.yaml: no such file or directory",
			wantCap: "config/settings.yaml",
		},
		{
			// Go-style os.Open error
			input:   "open ./relative/path: no such file or directory",
			wantCap: "./relative/path",
		},
	}
	for _, tc := range matchCases {
		m := re.FindStringSubmatch(tc.input)
		if len(m) < 2 {
			t.Errorf("pattern[3] did not match %q; expected capture %q", tc.input, tc.wantCap)
			continue
		}
		if got := m[1]; got != tc.wantCap {
			t.Errorf("pattern[3] capture for %q: got %q, want %q", tc.input, got, tc.wantCap)
		}
	}

	noMatchCases := []string{
		// Different error suffix
		"open /etc/missing.conf: permission denied",
		// No "open" keyword
		"read /etc/missing.conf: no such file or directory",
		"",
	}
	for _, s := range noMatchCases {
		if re.MatchString(s) {
			t.Errorf("pattern[3] should NOT match %q", s)
		}
	}
}

// ─── toolMissingExecutablePattern ─────────────────────────────────────────────

func TestToolMissingExecutablePattern_Matches(t *testing.T) {
	// Pattern: `(?mi)^(?:.*?:\s+)?([A-Za-z0-9._+\-/]+):\s+command not found$`
	re := toolMissingExecutablePattern

	matchCases := []struct {
		input   string
		wantCap string
	}{
		{
			input:   "bash: line 1: wasm-pack: command not found",
			wantCap: "wasm-pack",
		},
		{
			input:   "npm: command not found",
			wantCap: "npm",
		},
		{
			input:   "/usr/bin/env: cargo: command not found",
			wantCap: "cargo",
		},
		{
			input:   "zsh: rustup: command not found",
			wantCap: "rustup",
		},
		{
			// executable with dots and plus signs
			input:   "bash: g++: command not found",
			wantCap: "g++",
		},
	}
	for _, tc := range matchCases {
		m := re.FindStringSubmatch(tc.input)
		if len(m) < 2 {
			t.Errorf("execPattern did not match %q; expected capture %q", tc.input, tc.wantCap)
			continue
		}
		if got := m[1]; got != tc.wantCap {
			t.Errorf("execPattern capture for %q: got %q, want %q", tc.input, got, tc.wantCap)
		}
	}
}

func TestToolMissingExecutablePattern_NoMatch(t *testing.T) {
	re := toolMissingExecutablePattern

	noMatchCases := []string{
		// Similar but ends with different text
		"bash: wasm-pack: not found",
		// No colon-separated suffix
		"command not found",
		// Trailing whitespace after "not found" breaks the $ anchor
		"npm: command not found ",
		// Path with spaces (not matched by character class)
		"my tool: command not found",
		"",
	}
	for _, s := range noMatchCases {
		if re.MatchString(s) {
			t.Errorf("execPattern should NOT match %q", s)
		}
	}
}

// ─── extractMissingExecutables integration ────────────────────────────────────

func TestExtractMissingExecutables_DeduplicatesAndSorts(t *testing.T) {
	text := `
bash: line 1: wasm-pack: command not found
npm: command not found
bash: line 2: wasm-pack: command not found
`
	got := extractMissingExecutables(text)
	want := []string{"npm", "wasm-pack"}
	if len(got) != len(want) {
		t.Fatalf("extractMissingExecutables: got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("extractMissingExecutables[%d]: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestExtractMissingExecutables_EmptyInput(t *testing.T) {
	if got := extractMissingExecutables(""); got != nil {
		t.Fatalf("extractMissingExecutables(\"\") = %v, want nil", got)
	}
}

func TestExtractMissingExecutables_NoMatches(t *testing.T) {
	if got := extractMissingExecutables("everything went fine"); got != nil {
		t.Fatalf("extractMissingExecutables(no match) = %v, want nil", got)
	}
}

// ─── toolMissingPathPatterns count guard ──────────────────────────────────────

// TestToolMissingPathPatterns_Count guards the expected number of patterns.
// If a pattern is added or removed, this test will fail and force an explicit
// review of the regression coverage above.
func TestToolMissingPathPatterns_Count(t *testing.T) {
	const wantCount = 4
	if got := len(toolMissingPathPatterns); got != wantCount {
		t.Fatalf("len(toolMissingPathPatterns) = %d, want %d; update regression tests if you added/removed a pattern", got, wantCount)
	}
}
