package engine

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/danshapiro/kilroy/internal/attractor/dot"
	"github.com/danshapiro/kilroy/internal/attractor/runtime"
)

// TestSelectAllEligibleEdges_Step5FallbackEmitsStructuredLog verifies that when
// a node has only conditional outgoing edges and none match the current outcome,
// selectAllEligibleEdges emits a structured log entry to stderr identifying the
// node, edge count, and outcome. This makes the step-5 fallback observable.
func TestSelectAllEligibleEdges_Step5FallbackEmitsStructuredLog(t *testing.T) {
	g, err := dot.Parse([]byte(`
digraph G {
  graph [goal="test"]
  start [shape=Mdiamond]
  exit  [shape=Msquare]
  a [shape=diamond]
  b [shape=box, llm_provider=openai, llm_model=gpt-5.2]
  c [shape=box, llm_provider=openai, llm_model=gpt-5.2]
  start -> a
  a -> b [condition="outcome=success"]
  a -> c [condition="outcome=fail"]
  b -> exit
  c -> exit
}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	// Capture stderr to observe the step-5 fallback log entry.
	oldStderr := os.Stderr
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stderr = pw
	defer func() { os.Stderr = oldStderr }()

	// outcome=partial_success matches neither "outcome=success" nor "outcome=fail".
	// All edges are conditional, none match — step-5 fallback fires.
	out := runtime.Outcome{Status: runtime.StatusPartialSuccess}
	ctx := runtime.NewContext()
	edges, selErr := selectAllEligibleEdges(g, "a", out, ctx)

	_ = pw.Close()
	captured, _ := io.ReadAll(pr)
	stderr := string(captured)

	if selErr != nil {
		t.Fatalf("selectAllEligibleEdges: %v", selErr)
	}
	// Fallback returns all edges (spec §3.3).
	if len(edges) != 2 {
		t.Fatalf("got %d edges, want 2 (step-5 fallback returns all edges)", len(edges))
	}

	// Structured log must contain the event key and the node ID.
	if !strings.Contains(stderr, "step5_all_conditional_fallback") {
		t.Errorf("expected step5_all_conditional_fallback in stderr; got: %q", stderr)
	}
	if !strings.Contains(stderr, `"a"`) {
		t.Errorf("expected node id %q in stderr log; got: %q", "a", stderr)
	}
}

// TestSelectAllEligibleEdges_Step5FallbackNotEmitted_WhenConditionMatches verifies
// that the step-5 fallback log is NOT emitted when a condition matches (step-1 path).
func TestSelectAllEligibleEdges_Step5FallbackNotEmitted_WhenConditionMatches(t *testing.T) {
	g, err := dot.Parse([]byte(`
digraph G {
  graph [goal="test"]
  start [shape=Mdiamond]
  exit  [shape=Msquare]
  a [shape=diamond]
  b [shape=box, llm_provider=openai, llm_model=gpt-5.2]
  c [shape=box, llm_provider=openai, llm_model=gpt-5.2]
  start -> a
  a -> b [condition="outcome=success"]
  a -> c [condition="outcome=fail"]
  b -> exit
  c -> exit
}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	oldStderr := os.Stderr
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stderr = pw
	defer func() { os.Stderr = oldStderr }()

	// outcome=success matches "outcome=success" — step-1 returns early, no fallback.
	out := runtime.Outcome{Status: runtime.StatusSuccess}
	ctx := runtime.NewContext()
	_, selErr := selectAllEligibleEdges(g, "a", out, ctx)

	_ = pw.Close()
	captured, _ := io.ReadAll(pr)
	stderr := string(captured)

	if selErr != nil {
		t.Fatalf("selectAllEligibleEdges: %v", selErr)
	}
	if strings.Contains(stderr, "step5_all_conditional_fallback") {
		t.Errorf("unexpected step5_all_conditional_fallback in stderr (condition matched, no fallback expected); got: %q", stderr)
	}
}
