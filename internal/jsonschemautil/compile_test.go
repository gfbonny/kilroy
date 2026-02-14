package jsonschemautil

import (
	"os"
	"testing"
)

func TestCompileMapSchema_DoesNotDependOnProcessCWD(t *testing.T) {
	temp := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(temp); err != nil {
		t.Fatalf("chdir temp: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWD) })
	if err := os.RemoveAll(temp); err != nil {
		t.Fatalf("remove temp: %v", err)
	}

	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"file_path": map[string]any{"type": "string"},
		},
		"required": []string{"file_path"},
	}

	compiled, err := CompileMapSchema(schema, nil)
	if err != nil {
		t.Fatalf("CompileMapSchema: %v", err)
	}
	if err := compiled.Validate(map[string]any{"file_path": "x"}); err != nil {
		t.Fatalf("validate: %v", err)
	}
}
