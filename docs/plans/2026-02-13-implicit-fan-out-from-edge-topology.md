# Implicit Fan-Out from Edge Topology Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make the engine auto-detect fan-out/fan-in from edge topology (multiple eligible edges → parallel dispatch), while preserving explicit `component`/`tripleoctagon` support.

**Architecture:** When `selectNextEdge` finds multiple eligible edges whose targets converge at a common downstream node, the engine dispatches them in parallel using the existing `ParallelHandler` machinery. Explicit `shape=component` and `shape=tripleoctagon` nodes remain supported and take precedence (tripleoctagon is preferred in join detection). The change touches three layers: edge selection (returns multiple edges), join detection (finds any convergence node, prefers tripleoctagon), and the main loop (dispatches implicit fan-out between checkpoint and next-hop).

**Tech Stack:** Go, existing `internal/attractor/engine` package, `model.Graph` topology queries.

---

### Task 1: New function `selectAllEligibleEdges` — extract multi-edge resolution

The current `selectNextEdge` returns one edge. We need a variant that returns ALL eligible edges so the caller can decide whether it's a fan-out. This function contains the same logic as `selectNextEdge` but returns the full set instead of calling `bestEdge`.

**Files:**
- Modify: `internal/attractor/engine/engine.go:1599-1665`
- Test: `internal/attractor/engine/edge_selection_test.go`

**Step 1: Write the failing tests**

Add to `edge_selection_test.go`:

```go
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
```

**Step 2: Run tests to verify they fail**

Run: `cd /home/user/code/kilroy && go test ./internal/attractor/engine/ -run 'TestSelectAllEligibleEdges' -v -count=1`
Expected: FAIL — `selectAllEligibleEdges` undefined.

**Step 3: Implement `selectAllEligibleEdges`**

Add to `engine.go` after `selectNextEdge` (around line 1665):

```go
// selectAllEligibleEdges returns all edges that are eligible for traversal from the given node.
// When multiple edges are returned, the caller should treat this as an implicit fan-out.
// Preferred-label and suggested-next-ID narrowing still apply — if they narrow to a single edge,
// only that edge is returned (no fan-out).
func selectAllEligibleEdges(g *model.Graph, from string, out runtime.Outcome, ctx *runtime.Context) ([]*model.Edge, error) {
	edges := g.Outgoing(from)
	if len(edges) == 0 {
		return nil, nil
	}

	// Eligible conditional edges.
	var condMatched []*model.Edge
	for _, e := range edges {
		if e == nil {
			continue
		}
		c := strings.TrimSpace(e.Condition())
		if c == "" {
			continue
		}
		ok, err := cond.Evaluate(c, out, ctx)
		if err != nil {
			return nil, err
		}
		if ok {
			condMatched = append(condMatched, e)
		}
	}
	if len(condMatched) > 0 {
		return condMatched, nil
	}

	// Unconditional edges are eligible when no condition matched.
	var uncond []*model.Edge
	for _, e := range edges {
		if e == nil {
			continue
		}
		if strings.TrimSpace(e.Condition()) == "" {
			uncond = append(uncond, e)
		}
	}
	if len(uncond) == 0 {
		return nil, nil
	}

	// Preferred label match narrows to one.
	if strings.TrimSpace(out.PreferredLabel) != "" {
		want := normalizeLabel(out.PreferredLabel)
		sort.SliceStable(uncond, func(i, j int) bool { return uncond[i].Order < uncond[j].Order })
		for _, e := range uncond {
			if normalizeLabel(e.Label()) == want {
				return []*model.Edge{e}, nil
			}
		}
	}

	// Suggested next IDs narrow to one.
	if len(out.SuggestedNextIDs) > 0 {
		sort.SliceStable(uncond, func(i, j int) bool { return uncond[i].Order < uncond[j].Order })
		for _, suggested := range out.SuggestedNextIDs {
			for _, e := range uncond {
				if e.To == suggested {
					return []*model.Edge{e}, nil
				}
			}
		}
	}

	return uncond, nil
}
```

**Step 4: Run tests to verify they pass**

Run: `cd /home/user/code/kilroy && go test ./internal/attractor/engine/ -run 'TestSelectAllEligibleEdges' -v -count=1`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/attractor/engine/engine.go internal/attractor/engine/edge_selection_test.go
git commit -m "feat(engine): add selectAllEligibleEdges for multi-edge fan-out detection

Extracts the edge eligibility logic from selectNextEdge into a new function
that returns ALL matching edges instead of picking a single winner. When
multiple edges match (multiple conditions or multiple unconditional), the
caller can detect implicit fan-out. Preferred-label and suggested-next-ID
narrowing still collapse to a single edge when applicable."
```

---

### Task 2: Generalize `bfsFanInDistances` to find any convergence node

Currently `bfsFanInDistances` only records nodes with `shapeToType == "parallel.fan_in"`. We need it to find ANY convergence node, but prefer `tripleoctagon` nodes when present.

**Files:**
- Modify: `internal/attractor/engine/parallel_handlers.go:603-665,667-701`
- Test: `internal/attractor/engine/parallel_test.go` (new tests)

**Step 1: Write the failing tests**

Add to a new file `internal/attractor/engine/implicit_fanout_test.go`:

```go
package engine

import (
	"testing"

	"github.com/strongdm/kilroy/internal/attractor/dot"
)

func TestFindJoinNode_PrefersTripleoctagon(t *testing.T) {
	// When both a tripleoctagon and a box convergence exist, prefer tripleoctagon.
	g, err := dot.Parse([]byte(`
digraph G {
  graph [goal="test"]
  start [shape=Mdiamond]
  exit  [shape=Msquare]
  a [shape=box, llm_provider=openai, llm_model=gpt-5.2]
  b [shape=box, llm_provider=openai, llm_model=gpt-5.2]
  join [shape=tripleoctagon]
  synth [shape=box, llm_provider=openai, llm_model=gpt-5.2]
  start -> a
  start -> b
  a -> join
  b -> join
  join -> synth
  synth -> exit
}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	branches := g.Outgoing("start")
	// Filter to non-nil with To targets
	var branchEdges []*model.Edge
	for _, e := range branches {
		if e != nil {
			branchEdges = append(branchEdges, e)
		}
	}
	joinID, err := findJoinNode(g, branchEdges)
	if err != nil {
		t.Fatalf("findJoinNode: %v", err)
	}
	if joinID != "join" {
		t.Fatalf("got %q, want join (tripleoctagon preferred)", joinID)
	}
}

func TestFindJoinNode_FallsBackToBoxConvergence(t *testing.T) {
	// When no tripleoctagon exists, find the first box convergence node.
	g, err := dot.Parse([]byte(`
digraph G {
  graph [goal="test"]
  start [shape=Mdiamond]
  exit  [shape=Msquare]
  a [shape=box, llm_provider=openai, llm_model=gpt-5.2]
  b [shape=box, llm_provider=openai, llm_model=gpt-5.2]
  c [shape=box, llm_provider=openai, llm_model=gpt-5.2]
  synth [shape=box, llm_provider=openai, llm_model=gpt-5.2]
  start -> a
  start -> b
  start -> c
  a -> synth
  b -> synth
  c -> synth
  synth -> exit
}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	branches := g.Outgoing("start")
	joinID, err := findJoinNode(g, branches)
	if err != nil {
		t.Fatalf("findJoinNode: %v", err)
	}
	if joinID != "synth" {
		t.Fatalf("got %q, want synth (box convergence fallback)", joinID)
	}
}

func TestFindJoinNode_NoBranches_Error(t *testing.T) {
	g, err := dot.Parse([]byte(`
digraph G {
  graph [goal="test"]
  start [shape=Mdiamond]
  exit  [shape=Msquare]
  start -> exit
}
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	_, err = findJoinNode(g, nil)
	if err == nil {
		t.Fatal("expected error for nil branches")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /home/user/code/kilroy && go test ./internal/attractor/engine/ -run 'TestFindJoinNode_' -v -count=1`
Expected: FAIL — `findJoinNode` undefined (only `findJoinFanInNode` exists).

**Step 3: Implement `findJoinNode` and `bfsConvergenceDistances`**

Add new functions to `parallel_handlers.go`, keeping the originals untouched (they're still used by explicit `ParallelHandler`):

```go
// findJoinNode finds the convergence point for a set of branches.
// Prefers tripleoctagon (explicit fan-in) nodes. Falls back to any node
// reachable from ALL branches (topological convergence).
func findJoinNode(g *model.Graph, branches []*model.Edge) (string, error) {
	if g == nil {
		return "", fmt.Errorf("graph is nil")
	}
	if len(branches) == 0 {
		return "", fmt.Errorf("no branches")
	}

	// First, try the existing fan-in-only search — if a tripleoctagon exists, prefer it.
	joinID, err := findJoinFanInNode(g, branches)
	if err == nil && joinID != "" {
		return joinID, nil
	}

	// Fallback: find any convergence node reachable from all branches.
	type cand struct {
		id      string
		maxDist int
		sumDist int
	}

	reachable := make([]map[string]int, 0, len(branches))
	for _, e := range branches {
		if e == nil {
			continue
		}
		dists := bfsAllDistances(g, e.To)
		reachable = append(reachable, dists)
	}
	if len(reachable) == 0 {
		return "", fmt.Errorf("no valid branches")
	}

	// Intersection: nodes reachable from all branches.
	var cands []cand
	for id, d0 := range reachable[0] {
		maxD := d0
		sumD := d0
		ok := true
		for i := 1; i < len(reachable); i++ {
			d, exists := reachable[i][id]
			if !exists {
				ok = false
				break
			}
			sumD += d
			if d > maxD {
				maxD = d
			}
		}
		if ok {
			cands = append(cands, cand{id: id, maxDist: maxD, sumDist: sumD})
		}
	}
	if len(cands) == 0 {
		return "", fmt.Errorf("no convergence node reachable from all branches")
	}

	sort.SliceStable(cands, func(i, j int) bool {
		if cands[i].maxDist != cands[j].maxDist {
			return cands[i].maxDist < cands[j].maxDist
		}
		if cands[i].sumDist != cands[j].sumDist {
			return cands[i].sumDist < cands[j].sumDist
		}
		return cands[i].id < cands[j].id
	})
	return cands[0].id, nil
}

// bfsAllDistances returns distances from start to ALL reachable nodes (not just fan-in nodes).
func bfsAllDistances(g *model.Graph, start string) map[string]int {
	type item struct {
		id   string
		dist int
	}
	seen := map[string]bool{start: true}
	queue := []item{{id: start, dist: 0}}
	out := map[string]int{}

	for len(queue) > 0 {
		it := queue[0]
		queue = queue[1:]
		// Record first (shortest) distance for every node except start itself.
		if it.id != start {
			if _, exists := out[it.id]; !exists {
				out[it.id] = it.dist
			}
		}
		for _, e := range g.Outgoing(it.id) {
			if e == nil || seen[e.To] {
				continue
			}
			seen[e.To] = true
			queue = append(queue, item{id: e.To, dist: it.dist + 1})
		}
	}
	return out
}
```

Note: `import "sort"` is already present in `parallel_handlers.go`.

**Step 4: Run tests to verify they pass**

Run: `cd /home/user/code/kilroy && go test ./internal/attractor/engine/ -run 'TestFindJoinNode_' -v -count=1`
Expected: PASS

**Step 5: Run existing parallel tests to verify no regression**

Run: `cd /home/user/code/kilroy && go test ./internal/attractor/engine/ -run 'TestRun_Parallel|TestParallel' -v -count=1`
Expected: PASS (existing tests still use explicit `component`/`tripleoctagon` and are unaffected).

**Step 6: Commit**

```bash
git add internal/attractor/engine/parallel_handlers.go internal/attractor/engine/implicit_fanout_test.go
git commit -m "feat(engine): add findJoinNode with tripleoctagon-preferred convergence detection

Adds findJoinNode that first tries findJoinFanInNode (tripleoctagon-only BFS),
and falls back to bfsAllDistances which finds ANY convergence node reachable
from all branches. This enables implicit fan-in detection from edge topology
while preserving tripleoctagon priority."
```

---

### Task 3: Implicit fan-out dispatch in the engine main loop

This is the core change. After checkpoint and before `resolveNextHop`, the main loop should check for implicit fan-out: call `selectAllEligibleEdges`, and if multiple edges are returned and a join node exists, dispatch them in parallel using the `ParallelHandler` machinery.

**Files:**
- Modify: `internal/attractor/engine/engine.go:570-642` (main loop, between checkpoint and resolveNextHop)
- Modify: `internal/attractor/engine/parallel_handlers.go` (extract reusable dispatch function)
- Test: `internal/attractor/engine/implicit_fanout_test.go`

**Step 1: Write the failing integration test**

Add to `implicit_fanout_test.go`:

```go
func TestRun_ImplicitFanOut_EdgeTopology(t *testing.T) {
	repo := t.TempDir()
	runCmd(t, repo, "git", "init")
	runCmd(t, repo, "git", "config", "user.name", "tester")
	runCmd(t, repo, "git", "config", "user.email", "tester@example.com")
	_ = os.WriteFile(filepath.Join(repo, "README.md"), []byte("hello\n"), 0o644)
	runCmd(t, repo, "git", "add", "-A")
	runCmd(t, repo, "git", "commit", "-m", "init")

	dotSrc := []byte(`
digraph G {
  graph [goal="test implicit fan-out"]
  start [shape=Mdiamond]
  exit  [shape=Msquare]
  source [shape=box, llm_provider=openai, llm_model=gpt-5.2, prompt="source", auto_status=true]
  branch_a [shape=box, llm_provider=openai, llm_model=gpt-5.2, prompt="a"]
  branch_b [shape=box, llm_provider=openai, llm_model=gpt-5.2, prompt="b"]
  branch_c [shape=box, llm_provider=openai, llm_model=gpt-5.2, prompt="c"]
  synth [shape=box, llm_provider=openai, llm_model=gpt-5.2, prompt="synth"]

  start -> source
  source -> branch_a
  source -> branch_b
  source -> branch_c
  branch_a -> synth
  branch_b -> synth
  branch_c -> synth
  synth -> exit
}
`)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	res, err := Run(ctx, dotSrc, RunOptions{RepoPath: repo})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if res.FinalStatus != runtime.FinalSuccess {
		t.Fatalf("status: got %v, want success", res.FinalStatus)
	}
	// All three branches should have run (parallel results file should exist).
	resultsPath := filepath.Join(res.LogsRoot, "source", "parallel_results.json")
	b, err := os.ReadFile(resultsPath)
	if err != nil {
		t.Fatalf("missing parallel_results.json at %s — branches likely did not fan out", resultsPath)
	}
	// Should contain all 3 branch keys.
	content := string(b)
	for _, key := range []string{"branch_a", "branch_b", "branch_c"} {
		if !strings.Contains(content, key) {
			t.Fatalf("parallel_results.json missing branch %q", key)
		}
	}
}

func TestRun_ImplicitFanOut_WithTripleoctagonJoin(t *testing.T) {
	repo := t.TempDir()
	runCmd(t, repo, "git", "init")
	runCmd(t, repo, "git", "config", "user.name", "tester")
	runCmd(t, repo, "git", "config", "user.email", "tester@example.com")
	_ = os.WriteFile(filepath.Join(repo, "README.md"), []byte("hello\n"), 0o644)
	runCmd(t, repo, "git", "add", "-A")
	runCmd(t, repo, "git", "commit", "-m", "init")

	dotSrc := []byte(`
digraph G {
  graph [goal="test implicit fan-out with tripleoctagon join"]
  start [shape=Mdiamond]
  exit  [shape=Msquare]
  source [shape=box, llm_provider=openai, llm_model=gpt-5.2, prompt="source", auto_status=true]
  branch_a [shape=box, llm_provider=openai, llm_model=gpt-5.2, prompt="a"]
  branch_b [shape=box, llm_provider=openai, llm_model=gpt-5.2, prompt="b"]
  join [shape=tripleoctagon]
  synth [shape=box, llm_provider=openai, llm_model=gpt-5.2, prompt="synth"]

  start -> source
  source -> branch_a
  source -> branch_b
  branch_a -> join
  branch_b -> join
  join -> synth
  synth -> exit
}
`)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	res, err := Run(ctx, dotSrc, RunOptions{RepoPath: repo})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if res.FinalStatus != runtime.FinalSuccess {
		t.Fatalf("status: got %v, want success", res.FinalStatus)
	}
	// Tripleoctagon join should have been used (fan-in status.json should exist).
	assertExists(t, filepath.Join(res.LogsRoot, "join", "status.json"))
}

func TestRun_ImplicitFanOut_ConditionalEdges(t *testing.T) {
	repo := t.TempDir()
	runCmd(t, repo, "git", "init")
	runCmd(t, repo, "git", "config", "user.name", "tester")
	runCmd(t, repo, "git", "config", "user.email", "tester@example.com")
	_ = os.WriteFile(filepath.Join(repo, "README.md"), []byte("hello\n"), 0o644)
	runCmd(t, repo, "git", "add", "-A")
	runCmd(t, repo, "git", "commit", "-m", "init")

	dotSrc := []byte(`
digraph G {
  graph [goal="test conditional fan-out"]
  start [shape=Mdiamond]
  exit  [shape=Msquare]
  check [shape=diamond]
  branch_a [shape=box, llm_provider=openai, llm_model=gpt-5.2, prompt="a"]
  branch_b [shape=box, llm_provider=openai, llm_model=gpt-5.2, prompt="b"]
  fallback [shape=box, llm_provider=openai, llm_model=gpt-5.2, prompt="fallback"]
  synth [shape=box, llm_provider=openai, llm_model=gpt-5.2, prompt="synth"]

  start -> check
  check -> branch_a [condition="outcome=success"]
  check -> branch_b [condition="outcome=success"]
  check -> fallback [condition="outcome=fail"]
  branch_a -> synth
  branch_b -> synth
  fallback -> exit
  synth -> exit
}
`)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	res, err := Run(ctx, dotSrc, RunOptions{RepoPath: repo})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if res.FinalStatus != runtime.FinalSuccess {
		t.Fatalf("status: got %v, want success", res.FinalStatus)
	}
	// Both branch_a and branch_b should have run (both match outcome=success).
	resultsPath := filepath.Join(res.LogsRoot, "check", "parallel_results.json")
	b, err := os.ReadFile(resultsPath)
	if err != nil {
		t.Fatalf("missing parallel_results.json — conditional fan-out did not trigger")
	}
	content := string(b)
	for _, key := range []string{"branch_a", "branch_b"} {
		if !strings.Contains(content, key) {
			t.Fatalf("parallel_results.json missing branch %q", key)
		}
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /home/user/code/kilroy && go test ./internal/attractor/engine/ -run 'TestRun_ImplicitFanOut' -v -count=1 -timeout 120s`
Expected: FAIL — implicit fan-out not implemented, engine follows single edge.

**Step 3: Extract `dispatchParallelBranches` from `ParallelHandler.Execute`**

Refactor `parallel_handlers.go` to extract the branch dispatch logic into a standalone function that both `ParallelHandler.Execute` and the implicit fan-out path can call. The function signature:

```go
// dispatchParallelBranches runs branches in parallel and returns the results.
// It creates a checkpoint commit, spawns worktrees for each branch, runs subgraphs,
// and collects results. This is the shared core used by both explicit ParallelHandler
// and implicit edge-topology fan-out.
func dispatchParallelBranches(
	ctx context.Context,
	exec *Execution,
	sourceNodeID string,
	branches []*model.Edge,
	joinID string,
) ([]parallelBranchResult, string, error) {
	// ... extracted from ParallelHandler.Execute lines 96-157 ...
	// Returns (results, baseSHA, error)
}
```

Then `ParallelHandler.Execute` calls `dispatchParallelBranches` and builds the outcome. The implicit path in `engine.go` calls `dispatchParallelBranches` directly.

Extract lines 96–173 of `ParallelHandler.Execute` into `dispatchParallelBranches`, leaving `ParallelHandler.Execute` as a thin wrapper:

```go
func (h *ParallelHandler) Execute(ctx context.Context, exec *Execution, node *model.Node) (runtime.Outcome, error) {
	if exec == nil || exec.Engine == nil || exec.Graph == nil {
		return runtime.Outcome{Status: runtime.StatusFail, FailureReason: "parallel handler missing execution context"}, nil
	}

	branches := exec.Graph.Outgoing(node.ID)
	if len(branches) == 0 {
		return runtime.Outcome{Status: runtime.StatusFail, FailureReason: "parallel node has no outgoing edges"}, nil
	}

	joinID, err := findJoinFanInNode(exec.Graph, branches)
	if err != nil {
		return runtime.Outcome{Status: runtime.StatusFail, FailureReason: err.Error()}, nil
	}

	results, baseSHA, err := dispatchParallelBranches(ctx, exec, node.ID, branches, joinID)
	if err != nil {
		return runtime.Outcome{Status: runtime.StatusFail, FailureReason: err.Error()}, err
	}

	stageDir := filepath.Join(exec.LogsRoot, node.ID)
	_ = os.MkdirAll(stageDir, 0o755)
	_ = writeJSON(filepath.Join(stageDir, "parallel_results.json"), results)

	return runtime.Outcome{
		Status: runtime.StatusSuccess,
		Notes:  fmt.Sprintf("parallel fan-out complete (%d branches), join=%s", len(results), joinID),
		ContextUpdates: map[string]any{
			"parallel.join_node": joinID,
			"parallel.results":   results,
		},
		Meta: map[string]any{
			"kilroy.git_checkpoint_sha": baseSHA,
		},
	}, nil
}
```

**Step 4: Add implicit fan-out dispatch to `engine.go` main loop**

Insert after the explicit `parallel` check (line 580) and before `resolveNextHop` (line 583). This is the key change:

```go
		// Implicit fan-out: when a non-parallel node has multiple eligible outgoing
		// edges that converge at a common downstream node, dispatch them in parallel.
		// This enables edge-topology fan-out per the canonical dot spec patterns.
		if !isExplicitParallel {
			allEdges, edgeErr := selectAllEligibleEdges(e.Graph, node.ID, out, e.Context)
			if edgeErr != nil {
				return nil, edgeErr
			}
			if len(allEdges) > 1 {
				joinID, joinErr := findJoinNode(e.Graph, allEdges)
				if joinErr == nil && joinID != "" {
					exec := &Execution{
						Graph:       e.Graph,
						Context:     e.Context,
						LogsRoot:    e.LogsRoot,
						WorktreeDir: e.WorktreeDir,
						Engine:      e,
					}
					results, baseSHA, dispatchErr := dispatchParallelBranches(ctx, exec, node.ID, allEdges, joinID)
					if dispatchErr != nil {
						return nil, dispatchErr
					}
					stageDir := filepath.Join(e.LogsRoot, node.ID)
					_ = os.MkdirAll(stageDir, 0o755)
					_ = writeJSON(filepath.Join(stageDir, "parallel_results.json"), results)

					e.Context.ApplyUpdates(map[string]any{
						"parallel.join_node": joinID,
						"parallel.results":   results,
					})
					e.appendProgress(map[string]any{
						"event":       "implicit_fan_out",
						"source_node": node.ID,
						"join_node":   joinID,
						"branches":    len(results),
						"base_sha":    baseSHA,
					})

					e.incomingEdge = nil
					current = joinID
					continue
				}
				// If no convergence node found, fall through to single-edge selection.
			}
		}
```

The `isExplicitParallel` variable should be set from the existing check at line 572:

```go
		isExplicitParallel := false
		if t := strings.TrimSpace(node.TypeOverride()); t == "parallel" || (t == "" && shapeToType(node.Shape()) == "parallel") {
			isExplicitParallel = true
			join := strings.TrimSpace(e.Context.GetString("parallel.join_node", ""))
			if join == "" {
				return nil, fmt.Errorf("parallel node missing parallel.join_node in context")
			}
			e.incomingEdge = nil
			current = join
			continue
		}
```

**Step 5: Run the failing tests again**

Run: `cd /home/user/code/kilroy && go test ./internal/attractor/engine/ -run 'TestRun_ImplicitFanOut' -v -count=1 -timeout 120s`
Expected: PASS

**Step 6: Run the FULL test suite to check for regressions**

Run: `cd /home/user/code/kilroy && go test ./internal/attractor/engine/ -v -count=1 -timeout 300s`
Expected: PASS (all existing tests including explicit parallel, edge selection, etc.)

**Step 7: Commit**

```bash
git add internal/attractor/engine/engine.go internal/attractor/engine/parallel_handlers.go internal/attractor/engine/implicit_fanout_test.go
git commit -m "feat(engine): implicit fan-out from edge topology

When a node has multiple eligible outgoing edges (multiple matching conditions
or multiple unconditional edges) that converge at a common downstream node,
the engine now dispatches them in parallel automatically. This aligns engine
behavior with the canonical dot spec patterns (consensus_task.dot, SKILL.md
fan-out examples) where fan-out is expressed via edge topology, not explicit
component/tripleoctagon shapes.

Explicit shape=component and shape=tripleoctagon are still fully supported
and take precedence — tripleoctagon nodes are preferred in join detection.

The dispatchParallelBranches function is extracted from ParallelHandler.Execute
so both explicit and implicit paths share the same branch execution machinery."
```

---

### Task 4: Verify existing explicit parallel tests still pass

Sanity check: the explicit `component`/`tripleoctagon` path must be untouched.

**Files:**
- Read (no changes): `internal/attractor/engine/parallel_test.go`, `internal/attractor/engine/parallel_guardrails_test.go`

**Step 1: Run existing parallel tests**

Run: `cd /home/user/code/kilroy && go test ./internal/attractor/engine/ -run 'TestRun_Parallel|TestParallel|TestFanIn' -v -count=1 -timeout 120s`
Expected: PASS

**Step 2: Run the reference compat tests**

Run: `cd /home/user/code/kilroy && go test ./internal/attractor/engine/ -run 'TestReference' -v -count=1 -timeout 120s`
Expected: PASS

**Step 3: Run full engine test suite**

Run: `cd /home/user/code/kilroy && go test ./internal/attractor/engine/ -count=1 -timeout 300s`
Expected: PASS

---

### Task 5: Add edge case test — single-edge remains non-parallel

Verify that a node with exactly one outgoing edge does NOT trigger implicit fan-out.

**Files:**
- Modify: `internal/attractor/engine/implicit_fanout_test.go`

**Step 1: Write the test**

```go
func TestRun_SingleEdge_NoImplicitFanOut(t *testing.T) {
	repo := t.TempDir()
	runCmd(t, repo, "git", "init")
	runCmd(t, repo, "git", "config", "user.name", "tester")
	runCmd(t, repo, "git", "config", "user.email", "tester@example.com")
	_ = os.WriteFile(filepath.Join(repo, "README.md"), []byte("hello\n"), 0o644)
	runCmd(t, repo, "git", "add", "-A")
	runCmd(t, repo, "git", "commit", "-m", "init")

	dotSrc := []byte(`
digraph G {
  graph [goal="test no fan-out"]
  start [shape=Mdiamond]
  exit  [shape=Msquare]
  a [shape=box, llm_provider=openai, llm_model=gpt-5.2, prompt="a"]
  b [shape=box, llm_provider=openai, llm_model=gpt-5.2, prompt="b"]
  start -> a
  a -> b
  b -> exit
}
`)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	res, err := Run(ctx, dotSrc, RunOptions{RepoPath: repo})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if res.FinalStatus != runtime.FinalSuccess {
		t.Fatalf("status: got %v, want success", res.FinalStatus)
	}
	// No parallel_results.json should exist — single edge, no fan-out.
	resultsPath := filepath.Join(res.LogsRoot, "a", "parallel_results.json")
	if _, err := os.Stat(resultsPath); err == nil {
		t.Fatalf("parallel_results.json should NOT exist for single-edge traversal")
	}
}
```

**Step 2: Run the test**

Run: `cd /home/user/code/kilroy && go test ./internal/attractor/engine/ -run 'TestRun_SingleEdge_NoImplicitFanOut' -v -count=1 -timeout 60s`
Expected: PASS

**Step 3: Commit**

```bash
git add internal/attractor/engine/implicit_fanout_test.go
git commit -m "test(engine): verify single-edge paths don't trigger implicit fan-out"
```

---

### Task 6: Add edge case test — different conditions don't fan out

Verify that a diamond with `outcome=success` and `outcome=fail` edges does NOT trigger fan-out (only one condition matches at a time).

**Files:**
- Modify: `internal/attractor/engine/implicit_fanout_test.go`

**Step 1: Write the test**

```go
func TestRun_DifferentConditions_NoFanOut(t *testing.T) {
	repo := t.TempDir()
	runCmd(t, repo, "git", "init")
	runCmd(t, repo, "git", "config", "user.name", "tester")
	runCmd(t, repo, "git", "config", "user.email", "tester@example.com")
	_ = os.WriteFile(filepath.Join(repo, "README.md"), []byte("hello\n"), 0o644)
	runCmd(t, repo, "git", "add", "-A")
	runCmd(t, repo, "git", "commit", "-m", "init")

	dotSrc := []byte(`
digraph G {
  graph [goal="test no fan-out on different conditions"]
  start [shape=Mdiamond]
  exit  [shape=Msquare]
  check [shape=diamond]
  pass [shape=box, llm_provider=openai, llm_model=gpt-5.2, prompt="pass"]
  fail_path [shape=box, llm_provider=openai, llm_model=gpt-5.2, prompt="fail_path"]

  start -> check
  check -> pass      [condition="outcome=success"]
  check -> fail_path [condition="outcome=fail"]
  pass -> exit
  fail_path -> exit
}
`)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	res, err := Run(ctx, dotSrc, RunOptions{RepoPath: repo})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if res.FinalStatus != runtime.FinalSuccess {
		t.Fatalf("status: got %v, want success", res.FinalStatus)
	}
	// No parallel_results.json — only one condition matches.
	resultsPath := filepath.Join(res.LogsRoot, "check", "parallel_results.json")
	if _, err := os.Stat(resultsPath); err == nil {
		t.Fatalf("parallel_results.json should NOT exist — different conditions, not fan-out")
	}
}
```

**Step 2: Run the test**

Run: `cd /home/user/code/kilroy && go test ./internal/attractor/engine/ -run 'TestRun_DifferentConditions_NoFanOut' -v -count=1 -timeout 60s`
Expected: PASS

**Step 3: Commit**

```bash
git add internal/attractor/engine/implicit_fanout_test.go
git commit -m "test(engine): verify different conditions don't trigger implicit fan-out"
```

---

### Task 7: Update `selectNextEdge` to delegate to `selectAllEligibleEdges`

Now that `selectAllEligibleEdges` exists, refactor `selectNextEdge` to call it and pick the best from the result. This keeps the two functions in sync and eliminates logic duplication.

**Files:**
- Modify: `internal/attractor/engine/engine.go:1599-1665`

**Step 1: Refactor `selectNextEdge`**

Replace the body of `selectNextEdge` with:

```go
func selectNextEdge(g *model.Graph, from string, out runtime.Outcome, ctx *runtime.Context) (*model.Edge, error) {
	edges, err := selectAllEligibleEdges(g, from, out, ctx)
	if err != nil {
		return nil, err
	}
	if len(edges) == 0 {
		return nil, nil
	}
	return bestEdge(edges), nil
}
```

**Step 2: Run ALL edge selection tests**

Run: `cd /home/user/code/kilroy && go test ./internal/attractor/engine/ -run 'TestSelectNextEdge' -v -count=1`
Expected: PASS (all 4 existing tests pass — behavior preserved).

**Step 3: Run full engine test suite**

Run: `cd /home/user/code/kilroy && go test ./internal/attractor/engine/ -count=1 -timeout 300s`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/attractor/engine/engine.go
git commit -m "refactor(engine): selectNextEdge delegates to selectAllEligibleEdges

Eliminates duplicated edge-selection logic. selectNextEdge is now a thin
wrapper that calls selectAllEligibleEdges and picks the best edge from
the result."
```

---

### Summary of changes

| File | Change |
|------|--------|
| `engine.go` | Add `selectAllEligibleEdges`, refactor `selectNextEdge` to delegate, add implicit fan-out dispatch between checkpoint and `resolveNextHop` |
| `parallel_handlers.go` | Extract `dispatchParallelBranches`, add `findJoinNode` and `bfsAllDistances` |
| `edge_selection_test.go` | Tests for `selectAllEligibleEdges` |
| `implicit_fanout_test.go` (new) | Integration tests for implicit fan-out, convergence detection, edge cases |

Plan complete and saved to `docs/plans/2026-02-13-implicit-fan-out-from-edge-topology.md`. Two execution options:

**1. Subagent-Driven (this session)** - I dispatch fresh subagent per task, review between tasks, fast iteration

**2. Parallel Session (separate)** - Open new session with executing-plans, batch execution with checkpoints

Which approach?