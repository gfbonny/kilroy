package engine

import (
	"testing"

	"github.com/danshapiro/kilroy/internal/attractor/dot"
	"github.com/danshapiro/kilroy/internal/attractor/model"
	"github.com/danshapiro/kilroy/internal/attractor/runtime"
)

func TestSelectNextEdge_ConditionBeatsUnconditionalWeight(t *testing.T) {
	g, err := dot.Parse([]byte(`
digraph G {
  start [shape=Mdiamond]
  exit  [shape=Msquare]
  a [shape=box, llm_provider=openai, llm_model=gpt-5.2]
  b [shape=box, llm_provider=openai, llm_model=gpt-5.2]
  c [shape=box, llm_provider=openai, llm_model=gpt-5.2]
  start -> a
  a -> b [condition="outcome=success", weight=0]
  a -> c [weight=100]
  b -> exit
  c -> exit
}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out := runtime.Outcome{Status: runtime.StatusSuccess}
	ctx := runtime.NewContext()
	e, err := selectNextEdge(g, "a", out, ctx)
	if err != nil {
		t.Fatalf("selectNextEdge: %v", err)
	}
	if e == nil || e.To != "b" {
		t.Fatalf("edge: got %+v want to=b", e)
	}
}

func TestSelectNextEdge_PreferredLabelBeatsWeightAmongUnconditional(t *testing.T) {
	g, err := dot.Parse([]byte(`
digraph G {
  start [shape=Mdiamond]
  exit  [shape=Msquare]
  a [shape=box, llm_provider=openai, llm_model=gpt-5.2]
  b [shape=box, llm_provider=openai, llm_model=gpt-5.2]
  c [shape=box, llm_provider=openai, llm_model=gpt-5.2]
  start -> a
  a -> b [label="[A] Approve", weight=0]
  a -> c [label="[F] Fix", weight=100]
  b -> exit
  c -> exit
}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out := runtime.Outcome{Status: runtime.StatusSuccess, PreferredLabel: "Approve"}
	ctx := runtime.NewContext()
	e, err := selectNextEdge(g, "a", out, ctx)
	if err != nil {
		t.Fatalf("selectNextEdge: %v", err)
	}
	if e == nil || e.To != "b" {
		t.Fatalf("edge: got %+v want to=b", e)
	}
}

func TestSelectNextEdge_SuggestedNextIDsBeatsWeightAmongUnconditional(t *testing.T) {
	g, err := dot.Parse([]byte(`
digraph G {
  start [shape=Mdiamond]
  exit  [shape=Msquare]
  a [shape=box, llm_provider=openai, llm_model=gpt-5.2]
  b [shape=box, llm_provider=openai, llm_model=gpt-5.2]
  c [shape=box, llm_provider=openai, llm_model=gpt-5.2]
  start -> a
  a -> b [weight=100]
  a -> c [weight=0]
  b -> exit
  c -> exit
}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out := runtime.Outcome{Status: runtime.StatusSuccess, SuggestedNextIDs: []string{"c"}}
	ctx := runtime.NewContext()
	e, err := selectNextEdge(g, "a", out, ctx)
	if err != nil {
		t.Fatalf("selectNextEdge: %v", err)
	}
	if e == nil || e.To != "c" {
		t.Fatalf("edge: got %+v want to=c", e)
	}
}

func TestSelectNextEdge_WeightThenLexicalThenOrder(t *testing.T) {
	g, err := dot.Parse([]byte(`
digraph G {
  start [shape=Mdiamond]
  exit  [shape=Msquare]
  a [shape=box, llm_provider=openai, llm_model=gpt-5.2]
  b [shape=box, llm_provider=openai, llm_model=gpt-5.2]
  c [shape=box, llm_provider=openai, llm_model=gpt-5.2]
  d [shape=box, llm_provider=openai, llm_model=gpt-5.2]
  start -> a
  a -> d [weight=2]
  a -> c [weight=2]
  a -> b [weight=2]
  b -> exit
  c -> exit
  d -> exit
}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out := runtime.Outcome{Status: runtime.StatusSuccess}
	ctx := runtime.NewContext()
	e, err := selectNextEdge(g, "a", out, ctx)
	if err != nil {
		t.Fatalf("selectNextEdge: %v", err)
	}
	// All weights tied; lexical by to_node chooses "b".
	if e == nil || e.To != "b" {
		t.Fatalf("edge: got %+v want to=b", e)
	}
}

func TestSelectAllEligibleEdges_MultipleUnconditional(t *testing.T) {
	g, err := dot.Parse([]byte(`
digraph G {
  graph [goal="test"]
  start [shape=Mdiamond]
  exit  [shape=Msquare]
  a [shape=box, llm_provider=openai, llm_model=gpt-5.2]
  b [shape=box, llm_provider=openai, llm_model=gpt-5.2]
  c [shape=box, llm_provider=openai, llm_model=gpt-5.2]
  d [shape=box, llm_provider=openai, llm_model=gpt-5.2]
  start -> a
  a -> b
  a -> c
  a -> d
  b -> exit
  c -> exit
  d -> exit
}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out := runtime.Outcome{Status: runtime.StatusSuccess}
	ctx := runtime.NewContext()
	edges, err := selectAllEligibleEdges(g, "a", out, ctx)
	if err != nil {
		t.Fatalf("selectAllEligibleEdges: %v", err)
	}
	if len(edges) != 3 {
		t.Fatalf("got %d edges, want 3", len(edges))
	}
	targets := map[string]bool{}
	for _, e := range edges {
		targets[e.To] = true
	}
	for _, want := range []string{"b", "c", "d"} {
		if !targets[want] {
			t.Fatalf("missing target %q", want)
		}
	}
}

func TestSelectAllEligibleEdges_MultipleMatchingConditions(t *testing.T) {
	g, err := dot.Parse([]byte(`
digraph G {
  graph [goal="test"]
  start [shape=Mdiamond]
  exit  [shape=Msquare]
  a [shape=diamond]
  b [shape=box, llm_provider=openai, llm_model=gpt-5.2]
  c [shape=box, llm_provider=openai, llm_model=gpt-5.2]
  d [shape=box, llm_provider=openai, llm_model=gpt-5.2]
  start -> a
  a -> b [condition="outcome=success"]
  a -> c [condition="outcome=success"]
  a -> d [condition="outcome=fail"]
  b -> exit
  c -> exit
  d -> exit
}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out := runtime.Outcome{Status: runtime.StatusSuccess}
	ctx := runtime.NewContext()
	edges, err := selectAllEligibleEdges(g, "a", out, ctx)
	if err != nil {
		t.Fatalf("selectAllEligibleEdges: %v", err)
	}
	if len(edges) != 2 {
		t.Fatalf("got %d edges, want 2 (b and c)", len(edges))
	}
}

func TestSelectAllEligibleEdges_SingleEdge(t *testing.T) {
	g, err := dot.Parse([]byte(`
digraph G {
  graph [goal="test"]
  start [shape=Mdiamond]
  exit  [shape=Msquare]
  a [shape=box, llm_provider=openai, llm_model=gpt-5.2]
  b [shape=box, llm_provider=openai, llm_model=gpt-5.2]
  start -> a
  a -> b
  b -> exit
}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out := runtime.Outcome{Status: runtime.StatusSuccess}
	ctx := runtime.NewContext()
	edges, err := selectAllEligibleEdges(g, "a", out, ctx)
	if err != nil {
		t.Fatalf("selectAllEligibleEdges: %v", err)
	}
	if len(edges) != 1 {
		t.Fatalf("got %d edges, want 1", len(edges))
	}
	if edges[0].To != "b" {
		t.Fatalf("got %q, want b", edges[0].To)
	}
}

func TestSelectAllEligibleEdges_PreferredLabelNarrowsToOne(t *testing.T) {
	g, err := dot.Parse([]byte(`
digraph G {
  graph [goal="test"]
  start [shape=Mdiamond]
  exit  [shape=Msquare]
  a [shape=box, llm_provider=openai, llm_model=gpt-5.2]
  b [shape=box, llm_provider=openai, llm_model=gpt-5.2]
  c [shape=box, llm_provider=openai, llm_model=gpt-5.2]
  start -> a
  a -> b [label="approve"]
  a -> c [label="reject"]
  b -> exit
  c -> exit
}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out := runtime.Outcome{Status: runtime.StatusSuccess, PreferredLabel: "approve"}
	ctx := runtime.NewContext()
	edges, err := selectAllEligibleEdges(g, "a", out, ctx)
	if err != nil {
		t.Fatalf("selectAllEligibleEdges: %v", err)
	}
	if len(edges) != 1 {
		t.Fatalf("got %d edges, want 1 (preferred label narrows)", len(edges))
	}
	if edges[0].To != "b" {
		t.Fatalf("got %q, want b", edges[0].To)
	}
}

// --- V3.2: No eligible edge when all conditions fail (no fallback) ---

func TestSelectAllEligibleEdges_FallbackAnyEdge_AllConditionsFailed(t *testing.T) {
	// Spec §3.3 fallback: when all edges have conditions and none match,
	// return ALL edges so the caller can apply weight-then-lexical tiebreaking.
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
	// Outcome is partial_success -- neither "outcome=success" nor "outcome=fail" matches.
	out := runtime.Outcome{Status: runtime.StatusPartialSuccess}
	ctx := runtime.NewContext()
	edges, err := selectAllEligibleEdges(g, "a", out, ctx)
	if err != nil {
		t.Fatalf("selectAllEligibleEdges: %v", err)
	}
	// Fallback: all edges returned (spec §3.3 "Fallback: any edge").
	if len(edges) != 2 {
		t.Fatalf("got %d edges, want 2 (fallback returns all edges)", len(edges))
	}
}

func TestSelectNextEdge_FallbackAnyEdge_PicksBestByWeightThenLexical(t *testing.T) {
	// Spec §3.3 fallback: when all conditions fail and no unconditional edge exists,
	// selectNextEdge picks the best edge by weight-then-lexical from ALL edges.
	g, err := dot.Parse([]byte(`
digraph G {
  start [shape=Mdiamond]
  exit  [shape=Msquare]
  a [shape=diamond]
  b [shape=box, llm_provider=openai, llm_model=gpt-5.2]
  c [shape=box, llm_provider=openai, llm_model=gpt-5.2]
  start -> a
  a -> c [condition="outcome=success", weight=10]
  a -> b [condition="outcome=fail", weight=5]
  c -> exit
  b -> exit
}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// partial_success matches neither condition. No unconditional edge exists.
	// Fallback selects best by weight: c (weight=10) beats b (weight=5).
	out := runtime.Outcome{Status: runtime.StatusPartialSuccess}
	ctx := runtime.NewContext()
	e, err := selectNextEdge(g, "a", out, ctx)
	if err != nil {
		t.Fatalf("selectNextEdge: %v", err)
	}
	if e == nil {
		t.Fatalf("expected fallback edge, got nil")
	}
	if e.To != "c" {
		t.Fatalf("expected fallback edge to=c (highest weight), got to=%s", e.To)
	}
}

// --- V3.3: Preferred label searches ALL edges, not just unconditional ---

func TestSelectNextEdge_PreferredLabelMatchesConditionalEdge(t *testing.T) {
	// V3.3: Preferred label match (Step 2) iterates ALL edges per spec section 3.3.
	// A conditional edge whose condition did not pass but whose label matches
	// should still be selected by preferred label.
	g, err := dot.Parse([]byte(`
digraph G {
  start [shape=Mdiamond]
  exit  [shape=Msquare]
  a [shape=box, llm_provider=openai, llm_model=gpt-5.2]
  b [shape=box, llm_provider=openai, llm_model=gpt-5.2]
  c [shape=box, llm_provider=openai, llm_model=gpt-5.2]
  start -> a
  a -> b [condition="outcome=fail", label="[A] Approve", weight=0]
  a -> c [condition="outcome=fail", label="[R] Reject", weight=100]
  b -> exit
  c -> exit
}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// Outcome is success -- neither condition matches (both require outcome=fail).
	// But preferred_label="Approve" matches edge a->b's label.
	out := runtime.Outcome{Status: runtime.StatusSuccess, PreferredLabel: "Approve"}
	ctx := runtime.NewContext()
	e, err := selectNextEdge(g, "a", out, ctx)
	if err != nil {
		t.Fatalf("selectNextEdge: %v", err)
	}
	if e == nil || e.To != "b" {
		t.Fatalf("edge: got %+v want to=b (preferred label match on conditional edge)", e)
	}
}

func TestSelectAllEligibleEdges_PreferredLabelSearchesAllEdges(t *testing.T) {
	// V3.3: When no condition matches and there are only conditional edges,
	// preferred label should still find a match among them.
	g, err := dot.Parse([]byte(`
digraph G {
  graph [goal="test"]
  start [shape=Mdiamond]
  exit  [shape=Msquare]
  a [shape=box, llm_provider=openai, llm_model=gpt-5.2]
  b [shape=box, llm_provider=openai, llm_model=gpt-5.2]
  c [shape=box, llm_provider=openai, llm_model=gpt-5.2]
  start -> a
  a -> b [condition="outcome=fail", label="approve"]
  a -> c [condition="outcome=fail", label="reject"]
  b -> exit
  c -> exit
}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out := runtime.Outcome{Status: runtime.StatusSuccess, PreferredLabel: "reject"}
	ctx := runtime.NewContext()
	edges, err := selectAllEligibleEdges(g, "a", out, ctx)
	if err != nil {
		t.Fatalf("selectAllEligibleEdges: %v", err)
	}
	if len(edges) != 1 {
		t.Fatalf("got %d edges, want 1 (preferred label narrows to one)", len(edges))
	}
	if edges[0].To != "c" {
		t.Fatalf("got %q, want c (preferred label match)", edges[0].To)
	}
}

// --- V3.4: Suggested next IDs searches ALL edges, not just unconditional ---

func TestSelectNextEdge_SuggestedNextIDMatchesConditionalEdge(t *testing.T) {
	// V3.4: Suggested next IDs (Step 3) iterates ALL edges per spec section 3.3.
	// A conditional edge whose condition did not pass but whose target matches
	// a suggested ID should still be selected.
	g, err := dot.Parse([]byte(`
digraph G {
  start [shape=Mdiamond]
  exit  [shape=Msquare]
  a [shape=box, llm_provider=openai, llm_model=gpt-5.2]
  b [shape=box, llm_provider=openai, llm_model=gpt-5.2]
  c [shape=box, llm_provider=openai, llm_model=gpt-5.2]
  start -> a
  a -> b [condition="outcome=fail", weight=100]
  a -> c [condition="outcome=fail", weight=0]
  b -> exit
  c -> exit
}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// Outcome is success -- neither condition matches. SuggestedNextIDs picks "c".
	out := runtime.Outcome{Status: runtime.StatusSuccess, SuggestedNextIDs: []string{"c"}}
	ctx := runtime.NewContext()
	e, err := selectNextEdge(g, "a", out, ctx)
	if err != nil {
		t.Fatalf("selectNextEdge: %v", err)
	}
	if e == nil || e.To != "c" {
		t.Fatalf("edge: got %+v want to=c (suggested next ID match on conditional edge)", e)
	}
}

func TestSelectAllEligibleEdges_SuggestedNextIDSearchesAllEdges(t *testing.T) {
	// V3.4: When no condition matches and there are only conditional edges,
	// suggested next IDs should still find a match among them.
	g, err := dot.Parse([]byte(`
digraph G {
  graph [goal="test"]
  start [shape=Mdiamond]
  exit  [shape=Msquare]
  a [shape=box, llm_provider=openai, llm_model=gpt-5.2]
  b [shape=box, llm_provider=openai, llm_model=gpt-5.2]
  c [shape=box, llm_provider=openai, llm_model=gpt-5.2]
  start -> a
  a -> b [condition="outcome=fail"]
  a -> c [condition="outcome=fail"]
  b -> exit
  c -> exit
}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out := runtime.Outcome{Status: runtime.StatusSuccess, SuggestedNextIDs: []string{"c"}}
	ctx := runtime.NewContext()
	edges, err := selectAllEligibleEdges(g, "a", out, ctx)
	if err != nil {
		t.Fatalf("selectAllEligibleEdges: %v", err)
	}
	if len(edges) != 1 {
		t.Fatalf("got %d edges, want 1 (suggested ID narrows to one)", len(edges))
	}
	if edges[0].To != "c" {
		t.Fatalf("got %q, want c (suggested next ID match)", edges[0].To)
	}
}

// ---------------------------------------------------------------------------
// Direct bestEdge() unit tests — isolated tiebreak algorithm
//
// bestEdge sorts by: weight DESC → to_node lexical ASC → declaration order ASC.
// These tests call bestEdge() directly with hand-crafted model.Edge slices so
// that test failures point precisely to the tiebreak logic rather than to the
// broader edge-selection pipeline.
// ---------------------------------------------------------------------------

// makeEdge builds a minimal *model.Edge for tiebreak testing.
// weight="" means no weight attribute (bestEdge defaults to 0).
func makeEdge(to string, weight string, order int) *model.Edge {
	e := model.NewEdge("src", to)
	e.Order = order
	if weight != "" {
		e.Attrs["weight"] = weight
	}
	return e
}

// TestBestEdge_LexicalTiebreak: equal weight, different targets → lexically
// earlier target wins ("a" beats "z").
func TestBestEdge_LexicalTiebreak(t *testing.T) {
	edges := []*model.Edge{
		makeEdge("z", "5", 0),
		makeEdge("a", "5", 1),
	}
	got := bestEdge(edges)
	if got.To != "a" {
		t.Fatalf("lexical tiebreak: got to=%q, want to=a", got.To)
	}
}

// TestBestEdge_DeclarationOrderTiebreak: equal weight, same target, different
// declaration order → earlier declaration order wins.
func TestBestEdge_DeclarationOrderTiebreak(t *testing.T) {
	edges := []*model.Edge{
		makeEdge("x", "3", 5), // declared later
		makeEdge("x", "3", 1), // declared earlier — should win
	}
	got := bestEdge(edges)
	if got.Order != 1 {
		t.Fatalf("order tiebreak: got order=%d, want 1", got.Order)
	}
}

// TestBestEdge_WeightBeatsLexical: higher weight wins regardless of target
// name ("z" with weight=10 beats "a" with weight=0).
func TestBestEdge_WeightBeatsLexical(t *testing.T) {
	edges := []*model.Edge{
		makeEdge("a", "0", 0),
		makeEdge("z", "10", 1),
	}
	got := bestEdge(edges)
	if got.To != "z" {
		t.Fatalf("weight over lexical: got to=%q, want to=z", got.To)
	}
}

// TestBestEdge_ZeroBeatsNegativeWeight: zero weight (default) beats a
// negative weight edge (higher weight = better).
func TestBestEdge_ZeroBeatsNegativeWeight(t *testing.T) {
	edges := []*model.Edge{
		makeEdge("neg", "-5", 0),
		makeEdge("zer", "0", 1),
	}
	got := bestEdge(edges)
	if got.To != "zer" {
		t.Fatalf("zero vs negative: got to=%q, want to=zer", got.To)
	}
}

// TestBestEdge_AllThreeLevels exercises all three tiebreak levels at once
// with four edges covering every combination of high/low weight and early/late
// declaration order on two distinct target names.
//
//	Edge   weight  to   order
//	  W    10      b    0     ← wins: highest weight
//	  X    10      z    1     ← same weight as W, lexically after b
//	  Y    0       a    2     ← lower weight than W/X, but lexically first
//	  Z    0       a    3     ← same as Y but declared later
//
// Expected winner: W (weight 10 > 0, to="b" irrelevant because weight already
// dominates "a"/"z").
func TestBestEdge_AllThreeLevels(t *testing.T) {
	edgeW := makeEdge("b", "10", 0)
	edgeX := makeEdge("z", "10", 1)
	edgeY := makeEdge("a", "0", 2)
	edgeZ := makeEdge("a", "0", 3)

	// Deliberately supply in an order that would produce wrong results if any
	// sort key were missing or inverted.
	edges := []*model.Edge{edgeZ, edgeY, edgeX, edgeW}
	got := bestEdge(edges)
	if got != edgeW {
		t.Fatalf("all-three-levels: got to=%q order=%d weight=%s, want edgeW (to=b weight=10 order=0)",
			got.To, got.Order, got.Attr("weight", "0"))
	}
}

// TestBestEdge_AllThreeLevels_SecondaryLexical: when the highest-weight
// group has multiple members with different targets, the lexically earlier
// target wins within that group.
func TestBestEdge_AllThreeLevels_SecondaryLexical(t *testing.T) {
	// Both edges have weight=10; targets "apple" < "mango" lexically.
	edgeApple := makeEdge("apple", "10", 1)
	edgeMango := makeEdge("mango", "10", 0) // declared earlier but lexically later
	edges := []*model.Edge{edgeMango, edgeApple}
	got := bestEdge(edges)
	if got.To != "apple" {
		t.Fatalf("secondary lexical: got to=%q, want to=apple", got.To)
	}
}

// TestBestEdge_AllThreeLevels_TertiaryOrder: when weight and target are both
// tied, the edge with the smaller declaration order wins.
func TestBestEdge_AllThreeLevels_TertiaryOrder(t *testing.T) {
	// Same weight, same target, different order.
	edgeEarly := makeEdge("same", "7", 2)
	edgeLate := makeEdge("same", "7", 9)
	edges := []*model.Edge{edgeLate, edgeEarly}
	got := bestEdge(edges)
	if got.Order != 2 {
		t.Fatalf("tertiary order: got order=%d, want 2", got.Order)
	}
}

// TestBestEdge_SingleEdge: a slice with exactly one edge always returns that edge.
func TestBestEdge_SingleEdge(t *testing.T) {
	e := makeEdge("only", "0", 0)
	got := bestEdge([]*model.Edge{e})
	if got != e {
		t.Fatalf("single edge: got %+v, want original edge", got)
	}
}

func TestBestEdge_EmptySlice_Panics(t *testing.T) {
	// bestEdge panics on empty input — callers (selectNextEdge) must guard for len==0.
	// This test documents the contract so future readers know the invariant is
	// maintained by callers, not by bestEdge itself.
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on empty slice, but bestEdge returned normally")
		}
	}()
	bestEdge([]*model.Edge{})
}

func TestBestEdge_MissingWeightEqualsZero(t *testing.T) {
	// An edge with no weight attribute should behave identically to weight="0".
	// The tiebreak falls to declaration order: order=0 wins over order=1.
	edgeExplicit := makeEdge("a", "0", 0) // explicit zero weight
	edgeMissing := makeEdge("a", "", 1)   // no weight attr → defaults to 0
	got := bestEdge([]*model.Edge{edgeMissing, edgeExplicit})
	if got.Order != 0 {
		t.Fatalf("missing weight should equal explicit 0: got Order=%d, want 0", got.Order)
	}
}
