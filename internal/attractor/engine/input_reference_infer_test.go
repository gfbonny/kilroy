package engine

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeInputReferenceInferer struct {
	results []InferredReference
	err     error
	calls   int
	opts    []InputInferenceOptions
}

func (f *fakeInputReferenceInferer) Infer(ctx context.Context, docs []InputDocForInference, opts InputInferenceOptions) ([]InferredReference, error) {
	_ = ctx
	_ = docs
	f.calls++
	f.opts = append(f.opts, opts)
	if f.err != nil {
		return nil, f.err
	}
	return append([]InferredReference{}, f.results...), nil
}

func TestInputReferenceInfer_AddsImplicitGlobReferencesToClosure(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	mustWriteInputFile(t, filepath.Join(source, "definition_of_done.md"), "also run all tests in markdown files in c drive root")
	mustWriteInputFile(t, filepath.Join(source, "c_drive", "test_a.md"), "a")
	mustWriteInputFile(t, filepath.Join(source, "c_drive", "test_b.md"), "b")

	fake := &fakeInputReferenceInferer{
		results: []InferredReference{{
			Pattern:    filepath.ToSlash(filepath.Join(source, "c_drive", "*.md")),
			Rationale:  "natural language requirement",
			Confidence: "high",
		}},
	}

	_, err := materializeInputClosure(context.Background(), InputMaterializationOptions{
		SourceRoots:      []string{source},
		Include:          []string{"definition_of_done.md"},
		FollowReferences: true,
		TargetRoot:       target,
		InferWithLLM:     true,
		Inferer:          fake,
		InferProvider:    "openai",
		InferModel:       "gpt-5",
		InferenceCache:   map[string][]InferredReference{},
	})
	if err != nil {
		t.Fatalf("materializeInputClosure: %v", err)
	}
	assertExists(t, filepath.Join(target, "c_drive", "test_a.md"))
	assertExists(t, filepath.Join(target, "c_drive", "test_b.md"))
}

func TestInputReferenceInfer_FailureIsWarningAndFallsBackToDeterministic(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	mustWriteInputFile(t, filepath.Join(source, "definition_of_done.md"), "see [tests](tests.md)")
	mustWriteInputFile(t, filepath.Join(source, "tests.md"), "ok")

	fake := &fakeInputReferenceInferer{err: errors.New("inference unavailable")}
	manifest, err := materializeInputClosure(context.Background(), InputMaterializationOptions{
		SourceRoots:      []string{source},
		Include:          []string{"definition_of_done.md"},
		FollowReferences: true,
		TargetRoot:       target,
		InferWithLLM:     true,
		Inferer:          fake,
		InferProvider:    "openai",
		InferModel:       "gpt-5",
		InferenceCache:   map[string][]InferredReference{},
	})
	if err != nil {
		t.Fatalf("materializeInputClosure: %v", err)
	}
	assertExists(t, filepath.Join(target, "tests.md"))
	if manifest == nil {
		t.Fatal("expected manifest")
	}
	if !hasWarningContaining(manifest.Warnings, "input inference failed") {
		t.Fatalf("expected inferer warning, got: %+v", manifest.Warnings)
	}
}

func TestInputReferenceInfer_PassesProviderModelOverridesToInferer(t *testing.T) {
	source := t.TempDir()
	mustWriteInputFile(t, filepath.Join(source, "definition_of_done.md"), "infer references")

	fake := &fakeInputReferenceInferer{}
	_, err := materializeInputClosure(context.Background(), InputMaterializationOptions{
		SourceRoots:      []string{source},
		Include:          []string{"definition_of_done.md"},
		FollowReferences: true,
		InferWithLLM:     true,
		Inferer:          fake,
		InferProvider:    "anthropic",
		InferModel:       "claude-sonnet-4",
		InferenceCache:   map[string][]InferredReference{},
	})
	if err != nil {
		t.Fatalf("materializeInputClosure: %v", err)
	}
	if fake.calls != 1 {
		t.Fatalf("inferer calls: got %d want 1", fake.calls)
	}
	if len(fake.opts) != 1 {
		t.Fatalf("inferer opts calls: got %d want 1", len(fake.opts))
	}
	if got := fake.opts[0].Provider; got != "anthropic" {
		t.Fatalf("provider override: got %q want %q", got, "anthropic")
	}
	if got := fake.opts[0].Model; got != "claude-sonnet-4" {
		t.Fatalf("model override: got %q want %q", got, "claude-sonnet-4")
	}
}

func TestInputReferenceInfer_UsesContentHashCacheAcrossCalls(t *testing.T) {
	source := t.TempDir()
	targetA := t.TempDir()
	targetB := t.TempDir()
	mustWriteInputFile(t, filepath.Join(source, "definition_of_done.md"), "also run all markdown tests")
	mustWriteInputFile(t, filepath.Join(source, "tests", "a.md"), "a")

	fake := &fakeInputReferenceInferer{results: []InferredReference{{
		Pattern: filepath.ToSlash(filepath.Join(source, "tests", "*.md")),
	}}}
	cache := map[string][]InferredReference{}

	opts := InputMaterializationOptions{
		SourceRoots:      []string{source},
		Include:          []string{"definition_of_done.md"},
		FollowReferences: true,
		InferWithLLM:     true,
		Inferer:          fake,
		InferProvider:    "openai",
		InferModel:       "gpt-5",
		InferenceCache:   cache,
	}

	opts.TargetRoot = targetA
	if _, err := materializeInputClosure(context.Background(), opts); err != nil {
		t.Fatalf("first materializeInputClosure: %v", err)
	}
	opts.TargetRoot = targetB
	if _, err := materializeInputClosure(context.Background(), opts); err != nil {
		t.Fatalf("second materializeInputClosure: %v", err)
	}
	if fake.calls != 2 {
		t.Fatalf("inferer should be called once per unique content hash, got %d", fake.calls)
	}
}

func TestInputMaterialization_SnapshotSupportsHydrationAfterSourceDeletion(t *testing.T) {
	source := t.TempDir()
	runWorktree := t.TempDir()
	snapshot := t.TempDir()
	branchWorktree := t.TempDir()

	mustWriteInputFile(t, filepath.Join(source, "definition_of_done.md"), "see [tests](tests.md)")
	mustWriteInputFile(t, filepath.Join(source, "tests.md"), "test content")

	if _, err := materializeInputClosure(context.Background(), InputMaterializationOptions{
		SourceRoots:      []string{source},
		Include:          []string{"definition_of_done.md"},
		FollowReferences: true,
		TargetRoot:       runWorktree,
		SnapshotRoot:     snapshot,
	}); err != nil {
		t.Fatalf("startup materializeInputClosure: %v", err)
	}

	if err := os.RemoveAll(source); err != nil {
		t.Fatalf("remove source: %v", err)
	}
	if err := os.Remove(filepath.Join(runWorktree, "tests.md")); err != nil {
		t.Fatalf("remove run worktree test file: %v", err)
	}

	if _, err := materializeInputClosure(context.Background(), InputMaterializationOptions{
		SourceRoots:      []string{runWorktree, snapshot},
		Include:          []string{"definition_of_done.md"},
		FollowReferences: true,
		TargetRoot:       branchWorktree,
	}); err != nil {
		t.Fatalf("branch hydration materializeInputClosure: %v", err)
	}
	assertExists(t, filepath.Join(branchWorktree, "tests.md"))
}

func hasWarningContaining(warnings []string, needle string) bool {
	needle = strings.TrimSpace(needle)
	for _, w := range warnings {
		if strings.Contains(strings.ToLower(w), strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

func TestParseInferredReferencePayload_AcceptsWrappedOrArray(t *testing.T) {
	wrapped := `{"references":[{"pattern":"C:/**/*.md","rationale":"all markdown","confidence":"high"}]}`
	refs, err := parseInferredReferencePayload(wrapped)
	if err != nil {
		t.Fatalf("wrapped parse: %v", err)
	}
	if len(refs) != 1 || refs[0].Pattern != "C:/**/*.md" {
		t.Fatalf("wrapped parse refs: %+v", refs)
	}

	arr := fmt.Sprintf(`[{"pattern":%q,"rationale":"r","confidence":"medium"}]`, filepath.ToSlash("docs/**/*.md"))
	refs, err = parseInferredReferencePayload(arr)
	if err != nil {
		t.Fatalf("array parse: %v", err)
	}
	if len(refs) != 1 {
		t.Fatalf("array parse refs: %+v", refs)
	}
}
