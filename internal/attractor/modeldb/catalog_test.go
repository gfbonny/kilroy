package modeldb

import "testing"

func TestCatalogHasProviderModel_AcceptsCanonicalAndProviderRelativeIDs(t *testing.T) {
	c := &Catalog{Models: map[string]ModelEntry{
		"openai/gpt-5":       {Provider: "openai"},
		"anthropic/claude-4": {Provider: "anthropic"},
	}}
	if !CatalogHasProviderModel(c, "openai", "gpt-5") {
		t.Fatalf("expected provider-relative openai model id to resolve")
	}
	if !CatalogHasProviderModel(c, "openai", "openai/gpt-5") {
		t.Fatalf("expected canonical openai model id to resolve")
	}
}

func TestCatalogHasProviderModel_MatchesAnthropicDotsAndDashes(t *testing.T) {
	c := &Catalog{Models: map[string]ModelEntry{
		"anthropic/claude-sonnet-4.5": {Provider: "anthropic"},
		"anthropic/claude-opus-4.6":   {Provider: "anthropic"},
		"anthropic/claude-3.7-sonnet": {Provider: "anthropic"},
	}}
	// Native API format (dashes) should match catalog entries (dots).
	if !CatalogHasProviderModel(c, "anthropic", "claude-sonnet-4-5") {
		t.Fatalf("expected dash-format model to match dot-format catalog entry")
	}
	if !CatalogHasProviderModel(c, "anthropic", "claude-opus-4-6") {
		t.Fatalf("expected dash-format opus to match dot-format catalog entry")
	}
	if !CatalogHasProviderModel(c, "anthropic", "claude-3-7-sonnet") {
		t.Fatalf("expected dash-format 3-7 to match dot-format catalog entry")
	}
	// Dot format should still match directly.
	if !CatalogHasProviderModel(c, "anthropic", "claude-sonnet-4.5") {
		t.Fatalf("expected dot-format model to still match")
	}
}

func TestCatalogHasProviderModel_AcceptsOpenRouterProviderPrefixes(t *testing.T) {
	c := &Catalog{Models: map[string]ModelEntry{
		"moonshotai/kimi-k2.5": {},
		"z-ai/glm-4.7":         {},
	}}
	if !CatalogHasProviderModel(c, "kimi", "kimi-k2.5") {
		t.Fatalf("expected kimi provider-relative model to match moonshotai prefix")
	}
	if !CatalogHasProviderModel(c, "kimi", "moonshotai/kimi-k2.5") {
		t.Fatalf("expected kimi canonical/openrouter id to match")
	}
	if !CatalogHasProviderModel(c, "zai", "glm-4.7") {
		t.Fatalf("expected zai provider-relative model to match z-ai prefix")
	}
	if !CatalogHasProviderModel(c, "zai", "z-ai/glm-4.7") {
		t.Fatalf("expected zai canonical/openrouter id to match")
	}
}
