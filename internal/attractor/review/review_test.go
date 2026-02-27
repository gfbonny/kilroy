package review

import (
	"testing"

	"github.com/danshapiro/kilroy/internal/attractor/dot"
	"github.com/danshapiro/kilroy/internal/attractor/model"
)

func mustParse(t *testing.T, src string) *model.Graph {
	t.Helper()
	g, err := dot.Parse([]byte(src))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	return g
}

func TestDetectCycles_Linear(t *testing.T) {
	src := `digraph test {
		start [shape=Mdiamond]
		a [shape=box]
		b [shape=box]
		exit [shape=Msquare]
		start -> a
		a -> b
		b -> exit
	}`
	g := mustParse(t, src)
	cycles := detectCycles(g)
	if len(cycles) != 0 {
		t.Errorf("expected 0 cycles, got %d", len(cycles))
	}
}

func TestDetectCycles_LoopRestart(t *testing.T) {
	src := `digraph test {
		start [shape=Mdiamond]
		work [shape=box, auto_status=true]
		exit [shape=Msquare]
		start -> work
		work -> exit [condition="outcome=success"]
		work -> work [condition="outcome=fail && context.failure_class=transient_infra", loop_restart=true]
	}`
	g := mustParse(t, src)
	cycles := detectCycles(g)
	if len(cycles) != 1 {
		t.Fatalf("expected 1 cycle, got %d", len(cycles))
	}
	if !cycles[0].LoopRestart {
		t.Error("expected LoopRestart=true")
	}
	if cycles[0].From != "work" || cycles[0].To != "work" {
		t.Errorf("expected work→work back edge, got %s→%s", cycles[0].From, cycles[0].To)
	}
}

func TestDetectCycles_PostmortemRecovery(t *testing.T) {
	src := `digraph test {
		start [shape=Mdiamond]
		implement [shape=box]
		postmortem [shape=box, auto_status=true]
		exit [shape=Msquare]
		start -> implement
		implement -> exit [condition="outcome=success"]
		implement -> postmortem [condition="outcome=fail && context.failure_class!=transient_infra"]
		implement -> postmortem
		postmortem -> implement [condition="outcome=impl_repair"]
		postmortem -> implement
	}`
	g := mustParse(t, src)
	cycles := detectCycles(g)
	if len(cycles) < 1 {
		t.Fatalf("expected at least 1 cycle, got %d", len(cycles))
	}
	// Both conditional and unconditional edges from postmortem → implement are back edges.
	for _, c := range cycles {
		if c.To != "implement" {
			t.Errorf("expected all cycles to target implement, got back edge to %s", c.To)
		}
	}
}

func TestDetectCycles_WorkQueueLoop(t *testing.T) {
	src := `digraph test {
		start [shape=Mdiamond]
		plan_work [shape=box, auto_status=true]
		work_pool [shape=component]
		worker_0 [shape=box, auto_status=true]
		check_work_complete [shape=box, auto_status=true]
		merge_implementation [shape=box, auto_status=true]
		exit [shape=Msquare]
		start -> plan_work
		plan_work -> work_pool
		work_pool -> worker_0
		worker_0 -> check_work_complete
		check_work_complete -> work_pool [condition="outcome=more_work"]
		check_work_complete -> merge_implementation [condition="outcome=all_done"]
		merge_implementation -> exit
	}`
	g := mustParse(t, src)
	cycles := detectCycles(g)
	if len(cycles) != 1 {
		t.Fatalf("expected 1 cycle, got %d", len(cycles))
	}
	bodySet := map[string]bool{}
	for _, id := range cycles[0].LoopBody {
		bodySet[id] = true
	}
	for _, expected := range []string{"work_pool", "worker_0", "check_work_complete"} {
		if !bodySet[expected] {
			t.Errorf("expected %q in loop body, got: %v", expected, cycles[0].LoopBody)
		}
	}
}

func TestDetectCycles_TwoIndependentCycles(t *testing.T) {
	src := `digraph test {
		start [shape=Mdiamond]
		a [shape=box]
		b [shape=box]
		c [shape=box]
		d [shape=box]
		exit [shape=Msquare]
		start -> a
		a -> b
		b -> a [condition="outcome=retry"]
		b -> c [condition="outcome=success"]
		c -> d
		d -> c [condition="outcome=retry"]
		d -> exit [condition="outcome=success"]
	}`
	g := mustParse(t, src)
	cycles := detectCycles(g)
	if len(cycles) != 2 {
		t.Errorf("expected 2 cycles, got %d: %+v", len(cycles), cycles)
	}
}
