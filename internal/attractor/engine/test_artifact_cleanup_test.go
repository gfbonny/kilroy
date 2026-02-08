package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func cleanupStrayEngineArtifacts(t *testing.T) {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for _, name := range []string{"cli_wrote.txt", "status.json"} {
		_ = os.Remove(filepath.Join(cwd, name))
	}
}
