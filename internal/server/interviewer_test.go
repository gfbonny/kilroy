package server

import (
	"testing"
	"time"

	"github.com/strongdm/kilroy/internal/attractor/engine"
)

func TestWebInterviewer_AskAndAnswer(t *testing.T) {
	wi := NewWebInterviewer(5 * time.Second)

	done := make(chan engine.Answer, 1)
	go func() {
		ans := wi.Ask(engine.Question{
			Type: engine.QuestionSingleSelect,
			Text: "Approve?",
			Options: []engine.Option{
				{Key: "y", Label: "Yes"},
				{Key: "n", Label: "No"},
			},
			Stage: "review",
		})
		done <- ans
	}()

	// Wait for question to be parked.
	var pq *PendingQuestion
	for i := 0; i < 50; i++ {
		pq = wi.Pending()
		if pq != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if pq == nil {
		t.Fatal("expected pending question")
	}
	if pq.Text != "Approve?" {
		t.Fatalf("unexpected question text: %s", pq.Text)
	}
	if len(pq.Options) != 2 {
		t.Fatalf("expected 2 options, got %d", len(pq.Options))
	}
	if pq.Stage != "review" {
		t.Fatalf("unexpected stage: %s", pq.Stage)
	}

	// Answer it.
	ok := wi.Answer(pq.QuestionID, engine.Answer{Value: "y"})
	if !ok {
		t.Fatal("answer should have succeeded")
	}

	select {
	case ans := <-done:
		if ans.Value != "y" {
			t.Fatalf("unexpected answer value: %s", ans.Value)
		}
		if ans.TimedOut {
			t.Fatal("answer should not have timed out")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Ask to return")
	}

	// After answering, Pending should be nil.
	if wi.Pending() != nil {
		t.Fatal("expected no pending question after answer")
	}
}

func TestWebInterviewer_Timeout(t *testing.T) {
	wi := NewWebInterviewer(50 * time.Millisecond)

	start := time.Now()
	ans := wi.Ask(engine.Question{
		Type: engine.QuestionSingleSelect,
		Text: "Will timeout",
	})
	elapsed := time.Since(start)

	if !ans.TimedOut {
		t.Fatal("expected timeout")
	}
	if elapsed > 2*time.Second {
		t.Fatalf("timeout took too long: %v", elapsed)
	}
}

func TestWebInterviewer_AnswerWrongQID(t *testing.T) {
	wi := NewWebInterviewer(5 * time.Second)

	go func() {
		wi.Ask(engine.Question{Text: "test"})
	}()

	// Wait for question to be parked.
	for i := 0; i < 50; i++ {
		if wi.Pending() != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Answer with wrong ID.
	ok := wi.Answer("wrong-id", engine.Answer{Value: "x"})
	if ok {
		t.Fatal("answer with wrong QID should return false")
	}
}

func TestWebInterviewer_NoPending(t *testing.T) {
	wi := NewWebInterviewer(5 * time.Second)
	if wi.Pending() != nil {
		t.Fatal("expected no pending question initially")
	}

	ok := wi.Answer("q-1", engine.Answer{Value: "x"})
	if ok {
		t.Fatal("answer with no pending question should return false")
	}
}
