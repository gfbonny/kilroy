package dot

import "testing"

// FuzzParse exercises the DOT parser with arbitrary byte inputs.
// Seed corpus is drawn from existing test cases. The invariant is that
// Parse must never panic — it must always return either a valid *model.Graph
// or a well-typed error.
//
// Run the seed corpus as a regular test (no fuzzing):
//
//	go test ./internal/attractor/dot/... -run FuzzParse -count=1
//
// Run with active fuzzing:
//
//	go test ./internal/attractor/dot/... -fuzz=FuzzParse -fuzztime=30s
func FuzzParse(f *testing.F) {
	// Seed: minimal valid digraph.
	f.Add([]byte(`digraph G { a -> b }`))

	// Seed: graph with node attributes and chained edges (from TestParse_SimpleChainedEdgesAndDefaults).
	f.Add([]byte(`
digraph Simple {
    graph [goal="Run tests and report"]
    rankdir=LR
    node [shape=box, timeout=900s]
    edge [weight=0]
    start [shape=Mdiamond, label="Start"]
    exit  [shape=Msquare, label="Exit"]
    run_tests [label="Run Tests", prompt="Run the test suite and report results"]
    report    [label="Report", prompt="Summarize the test results"]
    start -> run_tests -> report -> exit
}`))

	// Seed: block comments, qualified keys, multiline string attrs
	// (from TestParse_MultilineAttrsAndComments).
	f.Add([]byte(`
digraph X {
    /* block comment with -> and [ ] */
    start [shape=Mdiamond]
    node1 [
        label="Node 1",
        prompt="line1\nline2",
        tool_hooks.pre="echo hi"
    ]
    // trailing comment
    exit [shape=Msquare]
    start -> node1 -> exit
}`))

	// Seed: subgraph with label-derived class (from TestParse_SubgraphLabelDerivesClass).
	f.Add([]byte(`
digraph G {
    start [shape=Mdiamond]
    exit [shape=Msquare]
    subgraph cluster_loop {
        label="Loop A"
        node [thread_id="loop-a"]
        Plan      [label="Plan next step"]
        Implement [label="Implement"]
    }
    start -> Plan -> Implement -> exit
}`))

	// Seed: top-level negative numeric values (from TestParse_TopLevelNegativeNumericValue).
	f.Add([]byte(`
digraph G {
    some_threshold = -1
    float_val = -3.5
    start [shape=Mdiamond]
    exit  [shape=Msquare]
    start -> exit
}`))

	// Seed: optional trailing semicolon (from TestParse_AllowsOptionalTrailingSemicolon).
	f.Add([]byte(`digraph A { start [shape=Mdiamond] exit [shape=Msquare] start -> exit };`))

	// Seed: ASCII identifiers with underscore and digit suffixes
	// (from TestLexer_AcceptsASCIIIdentifiers).
	f.Add([]byte(`digraph G {
    start [shape=Mdiamond]
    my_node_1 [shape=box]
    _private [shape=box]
    exit [shape=Msquare]
    start -> my_node_1 -> _private -> exit
}`))

	// Seed: missing comma between attrs (expected to return error, not panic).
	f.Add([]byte(`
digraph G {
  start [shape=Mdiamond]
  exit  [shape=Msquare]
  a [label="A" prompt="missing comma"]
  start -> a -> exit
}`))

	// Seed: undirected edge (expected to return error, not panic).
	f.Add([]byte(`
digraph G {
  start [shape=Mdiamond]
  exit  [shape=Msquare]
  start -- exit
}`))

	// Seed: two digraphs in one input (expected to return error, not panic).
	f.Add([]byte(`
digraph A { start [shape=Mdiamond] exit [shape=Msquare] start -> exit }
digraph B { start [shape=Mdiamond] exit [shape=Msquare] start -> exit }
`))

	// Seed: empty input.
	f.Add([]byte(``))

	// Seed: only whitespace.
	f.Add([]byte("   \n\t  "))

	// Seed: nested subgraphs.
	f.Add([]byte(`digraph G {
    subgraph outer {
        label="Outer"
        subgraph inner {
            label="Inner"
            a [shape=box]
        }
        b [shape=box]
    }
    a -> b
}`))

	// Seed: edge with condition attr.
	f.Add([]byte(`digraph G {
    start [shape=Mdiamond]
    ok    [shape=box]
    fail  [shape=box]
    exit  [shape=Msquare]
    start -> ok   [condition="outcome=success"]
    start -> fail [condition="outcome=fail"]
    ok    -> exit
    fail  -> exit
}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		// The invariant: Parse must never panic.
		// It may return an error for invalid inputs — that is correct behavior.
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("Parse panicked on input %q: %v", data, r)
			}
		}()
		_, _ = Parse(data)
	})
}
