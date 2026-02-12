package engine

import "testing"

func TestParseEscalationModels_Empty(t *testing.T) {
	chain := parseEscalationModels("")
	if len(chain) != 0 {
		t.Fatalf("expected empty chain, got %d entries", len(chain))
	}
}

func TestParseEscalationModels_SingleEntry(t *testing.T) {
	chain := parseEscalationModels("kimi:kimi-k2.5")
	if len(chain) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(chain))
	}
	if chain[0].Provider != "kimi" || chain[0].Model != "kimi-k2.5" {
		t.Fatalf("got %+v", chain[0])
	}
}

func TestParseEscalationModels_MultipleEntries(t *testing.T) {
	chain := parseEscalationModels("kimi:kimi-k2.5, google:gemini-pro, anthropic:claude-opus-4-6")
	if len(chain) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(chain))
	}
	expected := []providerModel{
		{Provider: "kimi", Model: "kimi-k2.5"},
		{Provider: "google", Model: "gemini-pro"},
		{Provider: "anthropic", Model: "claude-opus-4-6"},
	}
	for i, e := range expected {
		if chain[i].Provider != e.Provider || chain[i].Model != e.Model {
			t.Errorf("entry %d: got %+v, want %+v", i, chain[i], e)
		}
	}
}

func TestParseEscalationModels_WhitespaceHandling(t *testing.T) {
	chain := parseEscalationModels("  kimi : kimi-k2.5 ,  google : gemini-pro  ")
	if len(chain) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(chain))
	}
	if chain[0].Provider != "kimi" || chain[1].Provider != "google" {
		t.Fatalf("providers: %q, %q", chain[0].Provider, chain[1].Provider)
	}
}

func TestParseEscalationModels_InvalidEntrySkipped(t *testing.T) {
	chain := parseEscalationModels("kimi:kimi-k2.5, badentry, google:gemini-pro")
	if len(chain) != 2 {
		t.Fatalf("expected 2 entries (invalid skipped), got %d", len(chain))
	}
}

func TestRetriesBeforeEscalation_Default(t *testing.T) {
	got := retriesBeforeEscalation(nil)
	if got != defaultRetriesBeforeEscalation {
		t.Fatalf("got %d, want %d", got, defaultRetriesBeforeEscalation)
	}
}
