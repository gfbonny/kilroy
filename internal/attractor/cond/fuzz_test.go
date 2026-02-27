package cond

import (
	"testing"

	"github.com/danshapiro/kilroy/internal/attractor/runtime"
)

// FuzzConditionEval exercises the condition expression evaluator with arbitrary
// string inputs. Seed corpus is drawn from existing test cases. The invariant
// is that Evaluate must never panic — it must always return either a bool or a
// well-typed error.
//
// Run the seed corpus as a regular test (no fuzzing):
//
//	go test ./internal/attractor/cond/... -run FuzzConditionEval -count=1
//
// Run with active fuzzing:
//
//	go test ./internal/attractor/cond/... -fuzz=FuzzConditionEval -fuzztime=30s
func FuzzConditionEval(f *testing.F) {
	// Seed: empty condition (always true).
	f.Add("")

	// Seeds from TestEvaluate.
	f.Add("outcome=success")
	f.Add("outcome!=fail")
	f.Add("preferred_label=Yes")
	f.Add("context.tests_passed=true")
	f.Add("context.loop_state!=exhausted")
	f.Add("outcome=fail")
	f.Add("context.missing=foo")

	// Seeds from TestEvaluate_CustomOutcome.
	f.Add("outcome=process")
	f.Add("outcome=done")
	f.Add("outcome!=process")
	f.Add("outcome!=done")

	// Seeds from TestEvaluate_OutcomeAliasesMatch.
	f.Add("outcome=skip")
	f.Add("outcome=skipped")
	f.Add("outcome!=skip")
	f.Add("outcome=failure")
	f.Add("outcome!=failure")
	f.Add("outcome=error")
	f.Add("outcome=ok")

	// Seed: compound AND condition.
	f.Add("outcome=success && context.tests_passed=true")
	f.Add("outcome!=fail && preferred_label=Yes")

	// Seed: bare key (truthy check).
	f.Add("outcome")
	f.Add("preferred_label")
	f.Add("context.some_key")

	// Seed: whitespace-only.
	f.Add("   ")
	f.Add("\t\n")

	// Seed: malformed inputs (expected to return errors, not panic).
	f.Add("=")
	f.Add("!=")
	f.Add("&&")
	f.Add("outcome=")
	f.Add("=value")
	f.Add("&&&&")
	f.Add("outcome = success")    // spaces around operator
	f.Add("outcome!=fail&&x=y")   // no spaces around &&
	f.Add("a && && b")            // double &&
	f.Add("context.")             // incomplete context key

	f.Fuzz(func(t *testing.T, condition string) {
		// The invariant: Evaluate must never panic.
		// It may return an error for malformed conditions — that is correct behavior.
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("Evaluate panicked on condition %q: %v", condition, r)
			}
		}()

		// Use a fixed outcome and context so the fuzz target is deterministic
		// with respect to the evaluator logic itself (not runtime state).
		out := runtime.Outcome{
			Status:         runtime.StatusSuccess,
			PreferredLabel: "Yes",
		}
		ctx := runtime.NewContext()
		ctx.Set("tests_passed", true)
		ctx.Set("context.loop_state", "active")

		_, _ = Evaluate(condition, out, ctx)
	})
}
