package engine

import (
	"context"
	"testing"

	"github.com/strongdm/kilroy/internal/attractor/model"
	"github.com/strongdm/kilroy/internal/attractor/runtime"
)

type scriptedOutcomeHandler struct {
	outcomes []runtime.Outcome
	calls    int
}

func (h *scriptedOutcomeHandler) Execute(ctx context.Context, exec *Execution, node *model.Node) (runtime.Outcome, error) {
	_ = ctx
	_ = exec
	_ = node
	if len(h.outcomes) == 0 {
		h.calls++
		return runtime.Outcome{Status: runtime.StatusSuccess}, nil
	}
	idx := h.calls
	if idx >= len(h.outcomes) {
		idx = len(h.outcomes) - 1
	}
	h.calls++
	return h.outcomes[idx], nil
}

func newRetryTestEngine(t *testing.T, h Handler) (*Engine, *model.Node) {
	t.Helper()

	graph := model.NewGraph("retry-test")
	node := model.NewNode("work")
	node.Attrs["shape"] = "box"
	node.Attrs["type"] = "scripted"
	node.Attrs["max_retries"] = "3"
	node.Attrs["retry.backoff.initial_delay_ms"] = "0"
	graph.Nodes[node.ID] = node

	reg := NewDefaultRegistry()
	reg.Register("scripted", h)

	eng := &Engine{
		Graph:       graph,
		Options:     RunOptions{RunID: "retry-test"},
		LogsRoot:    t.TempDir(),
		WorktreeDir: t.TempDir(),
		Context:     runtime.NewContext(),
		Registry:    reg,
	}
	return eng, node
}

func TestExecuteWithRetry_DeterministicFailureClass_NoAdditionalAttempts(t *testing.T) {
	handler := &scriptedOutcomeHandler{
		outcomes: []runtime.Outcome{
			{
				Status:        runtime.StatusFail,
				FailureReason: "unknown flag: --verbose",
			},
		},
	}
	eng, node := newRetryTestEngine(t, handler)

	out, err := eng.executeWithRetry(context.Background(), node, map[string]int{})
	if err != nil {
		t.Fatalf("executeWithRetry error: %v", err)
	}
	if out.Status != runtime.StatusFail {
		t.Fatalf("status=%q want=%q", out.Status, runtime.StatusFail)
	}
	if handler.calls != 1 {
		t.Fatalf("attempts=%d want=1", handler.calls)
	}
}

func TestExecuteWithRetry_TransientFailureClass_RetriesUntilSuccess(t *testing.T) {
	handler := &scriptedOutcomeHandler{
		outcomes: []runtime.Outcome{
			{
				Status:        runtime.StatusRetry,
				FailureReason: "request timeout after 30s",
			},
			{
				Status: runtime.StatusSuccess,
			},
		},
	}
	eng, node := newRetryTestEngine(t, handler)

	out, err := eng.executeWithRetry(context.Background(), node, map[string]int{})
	if err != nil {
		t.Fatalf("executeWithRetry error: %v", err)
	}
	if out.Status != runtime.StatusSuccess {
		t.Fatalf("status=%q want=%q", out.Status, runtime.StatusSuccess)
	}
	if handler.calls != 2 {
		t.Fatalf("attempts=%d want=2", handler.calls)
	}
}
