package ingest

import (
	"testing"
)

func TestExtractDigraph(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		check   func(*testing.T, string)
	}{
		{
			name:  "clean digraph only",
			input: "digraph foo {\n    start [shape=Mdiamond]\n    exit [shape=Msquare]\n    start -> exit\n}",
			check: func(t *testing.T, got string) {
				if got[:7] != "digraph" {
					t.Errorf("should start with 'digraph', got %q", got[:20])
				}
			},
		},
		{
			name:  "digraph with leading text",
			input: "Here is the pipeline:\n\ndigraph foo {\n    start [shape=Mdiamond]\n    exit [shape=Msquare]\n    start -> exit\n}",
			check: func(t *testing.T, got string) {
				if got[:7] != "digraph" {
					t.Errorf("should start with 'digraph', got %q", got[:20])
				}
			},
		},
		{
			name:  "digraph in markdown code fence",
			input: "```dot\ndigraph foo {\n    start [shape=Mdiamond]\n    exit [shape=Msquare]\n    start -> exit\n}\n```",
			check: func(t *testing.T, got string) {
				if got[:7] != "digraph" {
					t.Errorf("should start with 'digraph', got %q", got[:20])
				}
				if got[len(got)-1] != '}' {
					t.Errorf("should end with '}', got %q", got[len(got)-5:])
				}
			},
		},
		{
			name:  "digraph with trailing text",
			input: "digraph foo {\n    start [shape=Mdiamond]\n    exit [shape=Msquare]\n    start -> exit\n}\n\nThis pipeline has 2 nodes.",
			check: func(t *testing.T, got string) {
				if got[len(got)-1] != '}' {
					t.Errorf("should end with '}', got %q", got[len(got)-10:])
				}
			},
		},
		{
			name:  "nested braces in prompts",
			input: "digraph foo {\n    n1 [prompt=\"if (x) { return }\"]\n    start [shape=Mdiamond]\n    exit [shape=Msquare]\n    start -> n1 -> exit\n}",
			check: func(t *testing.T, got string) {
				if got[:7] != "digraph" {
					t.Errorf("should start with 'digraph'")
				}
				if got[len(got)-1] != '}' {
					t.Errorf("should end with '}'")
				}
			},
		},
		{
			name:    "no digraph found",
			input:   "I couldn't generate the pipeline because the requirements are unclear.",
			wantErr: true,
		},
		{
			name:    "empty input",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractDigraph(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ExtractDigraph() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && tt.check != nil {
				tt.check(t, got)
			}
		})
	}
}
