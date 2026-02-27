package modeldb

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSuggest_UsesCacheWhenFresh(t *testing.T) {
	dir := t.TempDir()
	cacheFile := filepath.Join(dir, "modeldb_suggest.json")

	// Verify the embedded catalog loads (precondition).
	cat, err := LoadEmbeddedCatalog()
	if err != nil {
		t.Fatalf("LoadEmbeddedCatalog: %v", err)
	}
	_ = cat

	// Write a cache file that is fresh (fetched 1 minute ago).
	entry := suggestCache{
		FetchedAt: time.Now().Add(-1 * time.Minute),
		Data:      embeddedCatalogJSON,
	}
	b, _ := json.Marshal(entry)
	if err := os.WriteFile(cacheFile, b, 0o644); err != nil {
		t.Fatalf("write cache: %v", err)
	}

	// Suggest with a 24h TTL — should use cache (no network call needed).
	ctx := context.Background()
	out, err := Suggest(ctx, SuggestOptions{
		CacheDir: dir,
		TTL:      24 * time.Hour,
	})
	if err != nil {
		t.Fatalf("Suggest: %v", err)
	}
	if out == "" {
		t.Fatal("expected non-empty output")
	}
}

func TestSuggest_FallsBackToEmbeddedOnFetchFailure(t *testing.T) {
	// Use a cancelled context so fetch always fails.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	dir := t.TempDir()
	out, err := Suggest(ctx, SuggestOptions{
		CacheDir:     dir,
		TTL:          24 * time.Hour,
		ForceRefresh: true,
	})
	if err != nil {
		t.Fatalf("Suggest should not error on fetch failure: %v", err)
	}
	if out == "" {
		t.Fatal("expected non-empty output from embedded fallback")
	}
}

func TestFormatSuggestOutput_ContainsAllThreeProviders(t *testing.T) {
	cat, err := LoadEmbeddedCatalog()
	if err != nil {
		t.Fatalf("LoadEmbeddedCatalog: %v", err)
	}
	out := FormatSuggestOutput(cat, []string{"anthropic", "google", "openai"}, "test")
	for _, section := range []string{"ANTHROPIC", "GOOGLE", "OPENAI"} {
		if !strings.Contains(out, section) {
			t.Errorf("output missing section %q", section)
		}
	}
}

func TestFormatSuggestOutput_AnthropicDotNotationNote(t *testing.T) {
	cat, err := LoadEmbeddedCatalog()
	if err != nil {
		t.Fatalf("LoadEmbeddedCatalog: %v", err)
	}
	out := FormatSuggestOutput(cat, []string{"anthropic", "google", "openai"}, "test")
	if !strings.Contains(out, "dot notation") || !strings.Contains(out, "claude-opus-4.6") {
		t.Errorf("output missing Anthropic dot-notation note; got:\n%s", out)
	}
}

func TestFormatSuggestOutput_RecommendedDefaultsMarked(t *testing.T) {
	cat, err := LoadEmbeddedCatalog()
	if err != nil {
		t.Fatalf("LoadEmbeddedCatalog: %v", err)
	}
	out := FormatSuggestOutput(cat, []string{"anthropic", "google", "openai"}, "test")
	if !strings.Contains(out, "recommended default") {
		t.Errorf("output missing [recommended default] markers; got:\n%s", out)
	}
}
