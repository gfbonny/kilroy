package dotenv

import (
	"os"
	"strings"
	"testing"
)

// parseString is a test helper that parses a .env string.
func parseString(s string) ([][2]string, error) {
	return parse(strings.NewReader(s))
}

func TestParseBasic(t *testing.T) {
	pairs, err := parseString("KEY=VALUE\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(pairs) != 1 || pairs[0][0] != "KEY" || pairs[0][1] != "VALUE" {
		t.Fatalf("unexpected pairs: %v", pairs)
	}
}

func TestParseDoubleQuoted(t *testing.T) {
	pairs, err := parseString(`KEY="hello world"`)
	if err != nil {
		t.Fatal(err)
	}
	if pairs[0][1] != "hello world" {
		t.Fatalf("got %q", pairs[0][1])
	}
}

func TestParseSingleQuoted(t *testing.T) {
	pairs, err := parseString("KEY='hello world'")
	if err != nil {
		t.Fatal(err)
	}
	if pairs[0][1] != "hello world" {
		t.Fatalf("got %q", pairs[0][1])
	}
}

func TestParseDoubleQuotedPreservesWhitespace(t *testing.T) {
	pairs, err := parseString(`KEY="  spaces  "`)
	if err != nil {
		t.Fatal(err)
	}
	if pairs[0][1] != "  spaces  " {
		t.Fatalf("got %q", pairs[0][1])
	}
}

func TestParseCommentsAndBlanks(t *testing.T) {
	input := `
# This is a comment
KEY=VALUE

# Another comment
OTHER=123
`
	pairs, err := parseString(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(pairs) != 2 {
		t.Fatalf("expected 2 pairs, got %d: %v", len(pairs), pairs)
	}
}

func TestParseInlineComment(t *testing.T) {
	pairs, err := parseString("KEY=VALUE # inline comment\n")
	if err != nil {
		t.Fatal(err)
	}
	if pairs[0][1] != "VALUE" {
		t.Fatalf("got %q, want %q", pairs[0][1], "VALUE")
	}
}

func TestParseExportPrefix(t *testing.T) {
	pairs, err := parseString("export KEY=VALUE\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(pairs) != 1 || pairs[0][0] != "KEY" || pairs[0][1] != "VALUE" {
		t.Fatalf("unexpected pairs: %v", pairs)
	}
}

func TestParseEmptyValue(t *testing.T) {
	pairs, err := parseString("KEY=\n")
	if err != nil {
		t.Fatal(err)
	}
	if pairs[0][1] != "" {
		t.Fatalf("expected empty value, got %q", pairs[0][1])
	}
}

func TestLoadDoesNotOverwriteExisting(t *testing.T) {
	const key = "DOTENV_TEST_NO_OVERWRITE"
	t.Setenv(key, "shell_value")

	f, err := os.CreateTemp(t.TempDir(), "*.env")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString(key + "=env_value\n")
	f.Close()

	if err := Load(f.Name()); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv(key); got != "shell_value" {
		t.Fatalf("Load overwrote existing env var: got %q, want %q", got, "shell_value")
	}
}

func TestOverloadDoesOverwrite(t *testing.T) {
	const key = "DOTENV_TEST_OVERLOAD"
	t.Setenv(key, "shell_value")

	f, err := os.CreateTemp(t.TempDir(), "*.env")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString(key + "=env_value\n")
	f.Close()

	if err := Overload(f.Name()); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv(key); got != "env_value" {
		t.Fatalf("Overload did not overwrite: got %q, want %q", got, "env_value")
	}
}

func TestLoadMissingFileReturnsNil(t *testing.T) {
	err := Load("/nonexistent/path/that/does/not/exist/.env")
	if err != nil {
		t.Fatalf("expected nil for missing file, got: %v", err)
	}
}

func TestOverloadMissingFileReturnsNil(t *testing.T) {
	err := Overload("/nonexistent/path/that/does/not/exist/.env")
	if err != nil {
		t.Fatalf("expected nil for missing file, got: %v", err)
	}
}

func TestLoadSetsNewVar(t *testing.T) {
	const key = "DOTENV_TEST_NEW_VAR_LOAD"
	os.Unsetenv(key)

	f, err := os.CreateTemp(t.TempDir(), "*.env")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString(key + "=loaded_value\n")
	f.Close()

	if err := Load(f.Name()); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv(key); got != "loaded_value" {
		t.Fatalf("got %q, want %q", got, "loaded_value")
	}
}
