package engine

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestInputMaterializationResume_RestoresInputsFromSnapshotWithoutSourceWorkspace(t *testing.T) {
	sourceRepo := t.TempDir()
	worktree := t.TempDir()
	logsRoot := t.TempDir()

	mustWriteInputFile(t, filepath.Join(sourceRepo, ".ai", "definition_of_done.md"), "See [tests](../tests.md)")
	mustWriteInputFile(t, filepath.Join(sourceRepo, "tests.md"), "tests")

	eng := &Engine{
		Options:     RunOptions{RepoPath: sourceRepo, RunID: "resume-materialization-test"},
		WorktreeDir: worktree,
		LogsRoot:    logsRoot,
		InputMaterializationPolicy: InputMaterializationPolicy{
			Enabled:          true,
			Include:          nil,
			DefaultInclude:   []string{".ai/**"},
			FollowReferences: true,
			InferWithLLM:     false,
		},
		InputInferenceCache:  map[string][]InferredReference{},
		InputSourceTargetMap: map[string]string{},
	}

	if err := eng.materializeRunStartupInputs(context.Background()); err != nil {
		t.Fatalf("materializeRunStartupInputs: %v", err)
	}
	assertExists(t, filepath.Join(worktree, ".ai", "definition_of_done.md"))
	assertExists(t, filepath.Join(worktree, "tests.md"))
	assertExists(t, inputRunManifestPath(logsRoot))
	assertExists(t, inputSnapshotFilesRoot(logsRoot))

	if err := os.RemoveAll(filepath.Join(sourceRepo, ".ai")); err != nil {
		t.Fatalf("remove source .ai: %v", err)
	}
	if err := os.Remove(filepath.Join(sourceRepo, "tests.md")); err != nil {
		t.Fatalf("remove source tests.md: %v", err)
	}
	if err := os.RemoveAll(filepath.Join(worktree, ".ai")); err != nil {
		t.Fatalf("remove worktree .ai: %v", err)
	}
	if err := os.Remove(filepath.Join(worktree, "tests.md")); err != nil {
		t.Fatalf("remove worktree tests.md: %v", err)
	}

	if err := eng.materializeResumeStartupInputs(context.Background()); err != nil {
		t.Fatalf("materializeResumeStartupInputs: %v", err)
	}
	assertExists(t, filepath.Join(worktree, ".ai", "definition_of_done.md"))
	assertExists(t, filepath.Join(worktree, "tests.md"))
}
