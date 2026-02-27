package modelmeta

import "testing"

func TestProviderRelativeModelID(t *testing.T) {
	cases := []struct {
		provider string
		input    string
		want     string
	}{
		{"anthropic", "anthropic/claude-opus-4.6", "claude-opus-4.6"},
		{"anthropic", "claude-opus-4.6", "claude-opus-4.6"},
		{"google", "google/gemini-2.5-flash", "gemini-2.5-flash"},
		{"google", "gemini-2.5-flash", "gemini-2.5-flash"},
		{"openai", "openai/gpt-4.1", "gpt-4.1"},
		{"openai", "gpt-4.1", "gpt-4.1"},
		{"kimi", "kimi/moonshot-v1-8k", "moonshot-v1-8k"},
		{"kimi", "moonshot-v1-8k", "moonshot-v1-8k"},
		{"", "anthropic/claude-opus-4.6", "anthropic/claude-opus-4.6"},
	}
	for _, tc := range cases {
		got := ProviderRelativeModelID(tc.provider, tc.input)
		if got != tc.want {
			t.Errorf("ProviderRelativeModelID(%q, %q) = %q; want %q", tc.provider, tc.input, got, tc.want)
		}
	}
}

func TestNativeModelID(t *testing.T) {
	cases := []struct {
		provider string
		input    string
		want     string
	}{
		// Anthropic: dots → dashes in version numbers
		{"anthropic", "claude-opus-4.6", "claude-opus-4-6"},
		{"anthropic", "claude-opus-4-6", "claude-opus-4-6"}, // already native — passthrough
		{"anthropic", "anthropic/claude-opus-4.6", "claude-opus-4-6"},
		{"anthropic", "claude-sonnet-4.6", "claude-sonnet-4-6"},
		{"anthropic", "claude-haiku-4.5", "claude-haiku-4-5"},
		{"anthropic", "claude-3.7-sonnet", "claude-3-7-sonnet"},
		// Google: prefix stripped, no version normalization
		{"google", "gemini-2.5-flash", "gemini-2.5-flash"},
		{"google", "google/gemini-2.5-flash", "gemini-2.5-flash"},
		// OpenAI: prefix stripped, dots in version numbers preserved
		{"openai", "gpt-4.1", "gpt-4.1"},
		{"openai", "openai/gpt-4.1", "gpt-4.1"},
		{"openai", "gpt-5.3-codex-spark", "gpt-5.3-codex-spark"},
		// Kimi (openai-compat): prefix stripped, native ID unchanged
		{"kimi", "moonshot-v1-8k", "moonshot-v1-8k"},
		{"kimi", "kimi/moonshot-v1-8k", "moonshot-v1-8k"},
	}
	for _, tc := range cases {
		got := NativeModelID(tc.provider, tc.input)
		if got != tc.want {
			t.Errorf("NativeModelID(%q, %q) = %q; want %q", tc.provider, tc.input, got, tc.want)
		}
	}
}
