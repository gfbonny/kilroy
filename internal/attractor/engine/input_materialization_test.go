package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInputMaterialization_DefaultIncludeSeedsDoD(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	if err := os.MkdirAll(filepath.Join(source, ".ai"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWriteInputFile(t, filepath.Join(source, ".ai", "definition_of_done.md"), "line-by-line acceptance criteria\n")

	manifest, err := materializeInputClosure(context.Background(), InputMaterializationOptions{
		SourceRoots:      []string{source},
		Include:          nil,
		DefaultInclude:   []string{".ai/**"},
		FollowReferences: true,
		TargetRoot:       target,
	})
	if err != nil {
		t.Fatalf("materializeInputClosure: %v", err)
	}
	if manifest == nil {
		t.Fatal("expected manifest")
	}
	assertExists(t, filepath.Join(target, ".ai", "definition_of_done.md"))
}

func TestInputMaterialization_TransitiveReferenceClosureIncludesReferencedFiles(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	mustWriteInputFile(t, filepath.Join(source, ".ai", "definition_of_done.md"), "See [tests](../tests.md)\n")
	mustWriteInputFile(t, filepath.Join(source, "tests.md"), "integration checks\n")

	_, err := materializeInputClosure(context.Background(), InputMaterializationOptions{
		SourceRoots:      []string{source},
		Include:          []string{".ai/**"},
		DefaultInclude:   nil,
		FollowReferences: true,
		TargetRoot:       target,
	})
	if err != nil {
		t.Fatalf("materializeInputClosure: %v", err)
	}
	assertExists(t, filepath.Join(target, ".ai", "definition_of_done.md"))
	assertExists(t, filepath.Join(target, "tests.md"))
}

func TestInputMaterialization_RecursiveChainIncludesAllFiles(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	mustWriteInputFile(t, filepath.Join(source, "docs", "a.md"), "[b](b.md)\n")
	mustWriteInputFile(t, filepath.Join(source, "docs", "b.md"), "[c](c.md)\n")
	mustWriteInputFile(t, filepath.Join(source, "docs", "c.md"), "done\n")

	_, err := materializeInputClosure(context.Background(), InputMaterializationOptions{
		SourceRoots:      []string{source},
		Include:          []string{"docs/a.md"},
		DefaultInclude:   nil,
		FollowReferences: true,
		TargetRoot:       target,
	})
	if err != nil {
		t.Fatalf("materializeInputClosure: %v", err)
	}
	assertExists(t, filepath.Join(target, "docs", "a.md"))
	assertExists(t, filepath.Join(target, "docs", "b.md"))
	assertExists(t, filepath.Join(target, "docs", "c.md"))
}

func TestInputMaterialization_DeepRecursionHasNoFixedDepthCap(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	const count = 1500
	for i := 1; i <= count; i++ {
		name := fmt.Sprintf("doc_%04d.md", i)
		content := "done\n"
		if i < count {
			next := fmt.Sprintf("doc_%04d.md", i+1)
			content = fmt.Sprintf("see [%s](%s)\n", next, next)
		}
		mustWriteInputFile(t, filepath.Join(source, "docs", name), content)
	}

	_, err := materializeInputClosure(context.Background(), InputMaterializationOptions{
		SourceRoots:      []string{source},
		Include:          []string{"docs/doc_0001.md"},
		FollowReferences: true,
		TargetRoot:       target,
	})
	if err != nil {
		t.Fatalf("materializeInputClosure: %v", err)
	}
	assertExists(t, filepath.Join(target, "docs", "doc_1500.md"))
}

func TestInputMaterialization_IncludePatternWithoutMatchesFailsDeterministically(t *testing.T) {
	source := t.TempDir()
	_, err := materializeInputClosure(context.Background(), InputMaterializationOptions{
		SourceRoots:      []string{source},
		Include:          []string{"missing/**/*.md"},
		FollowReferences: true,
	})
	if err == nil {
		t.Fatal("expected include-missing error")
	}
	missingErr, ok := err.(*inputIncludeMissingError)
	if !ok {
		t.Fatalf("expected *inputIncludeMissingError, got %T (%v)", err, err)
	}
	if len(missingErr.Patterns) != 1 || missingErr.Patterns[0] != "missing/**/*.md" {
		t.Fatalf("unexpected missing patterns: %+v", missingErr.Patterns)
	}
	if !strings.Contains(err.Error(), "input_include_missing") {
		t.Fatalf("error should contain deterministic reason, got: %v", err)
	}
}

func TestInputMaterialization_DefaultIncludeWithoutMatchesDoesNotFail(t *testing.T) {
	source := t.TempDir()
	manifest, err := materializeInputClosure(context.Background(), InputMaterializationOptions{
		SourceRoots:      []string{source},
		Include:          nil,
		DefaultInclude:   []string{"missing/**/*.md"},
		FollowReferences: true,
	})
	if err != nil {
		t.Fatalf("materializeInputClosure: %v", err)
	}
	if manifest == nil {
		t.Fatal("expected manifest")
	}
	if len(manifest.ResolvedFiles) != 0 {
		t.Fatalf("expected no resolved files, got: %+v", manifest.ResolvedFiles)
	}
}

func TestInputMaterialization_DefaultIncludeSkipsArtifactDocsForReferenceTraversal(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()

	mustWriteInputFile(t, filepath.Join(source, ".ai", "definition_of_done.md"), "See [tests](../tests.md)\n")
	mustWriteInputFile(t, filepath.Join(source, "tests.md"), "integration checks\n")
	mustWriteInputFile(t, filepath.Join(source, ".ai", "benchmarks", "run1", "logs", "prompt.md"), "See [noise](../../../../docs/noise.md)\n")
	mustWriteInputFile(t, filepath.Join(source, "docs", "noise.md"), "artifact noise\n")

	_, err := materializeInputClosure(context.Background(), InputMaterializationOptions{
		SourceRoots:      []string{source},
		DefaultInclude:   []string{".ai/**"},
		FollowReferences: true,
		TargetRoot:       target,
	})
	if err != nil {
		t.Fatalf("materializeInputClosure: %v", err)
	}

	assertExists(t, filepath.Join(target, ".ai", "definition_of_done.md"))
	assertExists(t, filepath.Join(target, "tests.md"))
	if _, statErr := os.Stat(filepath.Join(target, "docs", "noise.md")); !os.IsNotExist(statErr) {
		t.Fatalf("expected artifact-only reference to be skipped, stat err=%v", statErr)
	}
}

func TestInputMaterialization_ExplicitIncludeTraversesArtifactDocs(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()

	mustWriteInputFile(t, filepath.Join(source, ".ai", "benchmarks", "run1", "logs", "prompt.md"), "See [required](../../../../docs/required.md)\n")
	mustWriteInputFile(t, filepath.Join(source, "docs", "required.md"), "must materialize\n")

	_, err := materializeInputClosure(context.Background(), InputMaterializationOptions{
		SourceRoots:      []string{source},
		Include:          []string{".ai/benchmarks/**/prompt.md"},
		FollowReferences: true,
		TargetRoot:       target,
	})
	if err != nil {
		t.Fatalf("materializeInputClosure: %v", err)
	}

	assertExists(t, filepath.Join(target, "docs", "required.md"))
}

func mustWriteInputFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
