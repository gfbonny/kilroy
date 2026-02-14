package llm

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
)

func TestGenerateObject_Success(t *testing.T) {
	c := NewClient()
	c.Register(&scriptedAdapter{
		name: "openai",
		steps: []func(req Request) (Response, error){
			func(req Request) (Response, error) {
				return Response{Message: Assistant(`{"name":"Alice","age":30}`)}, nil
			},
		},
	})

	prompt := "extract"
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{"type": "string"},
			"age":  map[string]any{"type": "integer"},
		},
		"required": []string{"name", "age"},
	}
	res, err := GenerateObject(context.Background(), GenerateObjectOptions{
		GenerateOptions: GenerateOptions{
			Client: c,
			Model:  "m",
			Prompt: &prompt,
		},
		Schema: schema,
	})
	if err != nil {
		t.Fatalf("GenerateObject: %v", err)
	}
	m, ok := res.Output.(map[string]any)
	if !ok {
		t.Fatalf("output type: %T", res.Output)
	}
	if m["name"] != "Alice" {
		t.Fatalf("name: %v", m["name"])
	}
	if _, ok := m["age"].(json.Number); !ok {
		t.Fatalf("age type: %T (%v)", m["age"], m["age"])
	}
}

func TestGenerateObject_ParseFailure_RaisesNoObjectGeneratedError(t *testing.T) {
	c := NewClient()
	c.Register(&scriptedAdapter{
		name: "openai",
		steps: []func(req Request) (Response, error){
			func(req Request) (Response, error) { return Response{Message: Assistant("not json")}, nil },
		},
	})

	prompt := "extract"
	_, err := GenerateObject(context.Background(), GenerateObjectOptions{
		GenerateOptions: GenerateOptions{
			Client: c,
			Model:  "m",
			Prompt: &prompt,
		},
		Schema: map[string]any{"type": "object"},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	var noe *NoObjectGeneratedError
	if !errors.As(err, &noe) {
		t.Fatalf("expected NoObjectGeneratedError, got %T (%v)", err, err)
	}
	if noe.RawText == "" {
		t.Fatalf("expected RawText to be set")
	}
}

func TestCompileJSONSchema_DoesNotDependOnGetwd(t *testing.T) {
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

	compiled, err := compileJSONSchema(schema)
	if err != nil {
		t.Fatalf("compileJSONSchema: %v", err)
	}
	if err := compiled.Validate(map[string]any{"file_path": "x"}); err != nil {
		t.Fatalf("validate: %v", err)
	}
}
