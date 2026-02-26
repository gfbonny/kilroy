package engine

import (
	"strings"
	"testing"
)

func TestInputReferenceScan_ExtractsMarkdownQuotedAndBarePaths(t *testing.T) {
	scanner := deterministicInputReferenceScanner{}
	content := strings.Join([]string{
		`Read [tests](docs/tests.md) and also "C:/repo/tests.md".`,
		`Use .ai/definition_of_done.md as source of truth.`,
		`Glob hint: C:/logs/**/*.md`,
		`Quoted glob: '.ai/**/*.md'`,
	}, "\n")

	refs := scanner.Scan(".ai/definition_of_done.md", []byte(content))
	got := map[string]InputReferenceKind{}
	for _, ref := range refs {
		got[ref.Pattern] = ref.Kind
		if ref.Confidence != "explicit" {
			t.Fatalf("confidence: got %q want explicit", ref.Confidence)
		}
	}

	requireRefKind(t, got, "docs/tests.md", InputReferenceKindPath)
	requireRefKind(t, got, "C:/repo/tests.md", InputReferenceKindPath)
	requireRefKind(t, got, ".ai/definition_of_done.md", InputReferenceKindPath)
	requireRefKind(t, got, "C:/logs/**/*.md", InputReferenceKindGlob)
	requireRefKind(t, got, ".ai/**/*.md", InputReferenceKindGlob)
}

func TestInputReferenceScan_IgnoresURLsAndParsesWindowsGlobToken(t *testing.T) {
	scanner := deterministicInputReferenceScanner{}
	refs := scanner.Scan("requirements.md", []byte(`visit https://example.com and scan C:/**/*.md and ./docs/spec.md`))
	got := map[string]InputReferenceKind{}
	for _, ref := range refs {
		got[ref.Pattern] = ref.Kind
	}
	if _, ok := got["https://example.com"]; ok {
		t.Fatalf("URL should not be treated as input reference: %+v", refs)
	}
	requireRefKind(t, got, "C:/**/*.md", InputReferenceKindGlob)
	requireRefKind(t, got, "docs/spec.md", InputReferenceKindPath)
}

func requireRefKind(t *testing.T, got map[string]InputReferenceKind, pattern string, want InputReferenceKind) {
	t.Helper()
	kind, ok := got[pattern]
	if !ok {
		t.Fatalf("missing extracted reference %q; got=%v", pattern, got)
	}
	if kind != want {
		t.Fatalf("reference %q kind: got %q want %q", pattern, kind, want)
	}
}
