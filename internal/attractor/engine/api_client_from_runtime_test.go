package engine

import (
	"testing"

	"github.com/strongdm/kilroy/internal/providerspec"
)

func TestNewAPIClientFromProviderRuntimes_RegistersAdaptersByProtocol(t *testing.T) {
	runtimes := map[string]ProviderRuntime{
		"openai": {
			Key:     "openai",
			Backend: BackendAPI,
			API: providerspec.APISpec{
				Protocol:           providerspec.ProtocolOpenAIResponses,
				DefaultBaseURL:     "http://127.0.0.1:0",
				DefaultAPIKeyEnv:   "OPENAI_API_KEY",
				ProviderOptionsKey: "openai",
			},
		},
	}
	t.Setenv("OPENAI_API_KEY", "test-key")
	c, err := newAPIClientFromProviderRuntimes(runtimes)
	if err != nil {
		t.Fatalf("newAPIClientFromProviderRuntimes: %v", err)
	}
	if len(c.ProviderNames()) != 1 {
		t.Fatalf("expected one adapter")
	}
}

func TestNewAPIClientFromProviderRuntimes_CLIOnlyIsNotAnError(t *testing.T) {
	runtimes := map[string]ProviderRuntime{
		"openai": {Key: "openai", Backend: BackendCLI},
	}
	c, err := newAPIClientFromProviderRuntimes(runtimes)
	if err != nil {
		t.Fatalf("expected nil error for cli-only runtimes, got %v", err)
	}
	if len(c.ProviderNames()) != 0 {
		t.Fatalf("expected zero adapters, got %v", c.ProviderNames())
	}
}

func TestNewAPIClientFromProviderRuntimes_RegistersOpenAICompatByProtocol(t *testing.T) {
	runtimes := map[string]ProviderRuntime{
		"kimi": {
			Key:     "kimi",
			Backend: BackendAPI,
			API: providerspec.APISpec{
				Protocol:           providerspec.ProtocolOpenAIChatCompletions,
				DefaultBaseURL:     "http://127.0.0.1:0",
				DefaultPath:        "/v1/chat/completions",
				DefaultAPIKeyEnv:   "KIMI_API_KEY",
				ProviderOptionsKey: "kimi",
			},
		},
	}
	t.Setenv("KIMI_API_KEY", "test-key")
	c, err := newAPIClientFromProviderRuntimes(runtimes)
	if err != nil {
		t.Fatalf("newAPIClientFromProviderRuntimes: %v", err)
	}
	if len(c.ProviderNames()) != 1 || c.ProviderNames()[0] != "kimi" {
		t.Fatalf("expected kimi adapter, got %v", c.ProviderNames())
	}
}
