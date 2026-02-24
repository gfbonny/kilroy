package engine

import "testing"

func TestResolveProviderRuntimes_MergesBuiltinAndConfigOverrides(t *testing.T) {
	cfg := &RunConfigFile{}
	cfg.LLM.Providers = map[string]ProviderConfig{
		"kimi": {
			Backend: BackendAPI,
			API: ProviderAPIConfig{
				APIKeyEnv: "KIMI_API_KEY",
				Headers:   map[string]string{"X-Trace": "1"},
			},
		},
	}

	rt, err := resolveProviderRuntimes(cfg)
	if err != nil {
		t.Fatalf("resolveProviderRuntimes: %v", err)
	}
	if rt["kimi"].API.Protocol != "anthropic_messages" {
		t.Fatalf("kimi protocol mismatch")
	}
	if _, ok := rt["zai"]; !ok {
		t.Fatalf("expected failover target runtime for zai")
	}
	if rt["zai"].API.DefaultPath != "/api/coding/paas/v4/chat/completions" {
		t.Fatalf("expected synthesized zai default path")
	}
	if _, ok := rt["cerebras"]; !ok {
		t.Fatalf("expected recursive failover target runtime for cerebras")
	}
	if got := rt["kimi"].APIHeaders(); got["X-Trace"] != "1" {
		t.Fatalf("expected runtime headers copy, got %v", got)
	}
}

func TestResolveProviderRuntimes_ExplicitEmptyFailoverDisablesBuiltinFallback(t *testing.T) {
	cfg := &RunConfigFile{}
	cfg.LLM.Providers = map[string]ProviderConfig{
		"zai": {
			Backend:  BackendAPI,
			Failover: []string{},
			API: ProviderAPIConfig{
				Protocol:  "openai_chat_completions",
				APIKeyEnv: "ZAI_API_KEY",
			},
		},
	}

	rt, err := resolveProviderRuntimes(cfg)
	if err != nil {
		t.Fatalf("resolveProviderRuntimes: %v", err)
	}
	if got := len(rt["zai"].Failover); got != 0 {
		t.Fatalf("zai failover len=%d want 0 for explicit empty override", got)
	}
	if !rt["zai"].FailoverExplicit {
		t.Fatalf("zai failover should be marked explicit")
	}
}

func TestResolveProviderRuntimes_OmittedFailoverUsesBuiltinFallback(t *testing.T) {
	cfg := &RunConfigFile{}
	cfg.LLM.Providers = map[string]ProviderConfig{
		"zai": {
			Backend: BackendAPI,
			API: ProviderAPIConfig{
				Protocol:  "openai_chat_completions",
				APIKeyEnv: "ZAI_API_KEY",
			},
		},
	}

	rt, err := resolveProviderRuntimes(cfg)
	if err != nil {
		t.Fatalf("resolveProviderRuntimes: %v", err)
	}
	if got := rt["zai"].Failover; len(got) != 1 || got[0] != "cerebras" {
		t.Fatalf("zai failover=%v want [cerebras]", got)
	}
	if rt["zai"].FailoverExplicit {
		t.Fatalf("zai failover should not be marked explicit when omitted")
	}
}

func TestResolveProviderRuntimes_RejectsCanonicalAliasCollisions(t *testing.T) {
	cfg := &RunConfigFile{}
	cfg.LLM.Providers = map[string]ProviderConfig{
		"zai": {
			Backend: BackendAPI,
			API: ProviderAPIConfig{
				Protocol:  "openai_chat_completions",
				APIKeyEnv: "ZAI_API_KEY",
			},
		},
		"z-ai": {
			Backend: BackendAPI,
			API: ProviderAPIConfig{
				Protocol:  "openai_chat_completions",
				APIKeyEnv: "ZAI_API_KEY",
			},
		},
	}

	_, err := resolveProviderRuntimes(cfg)
	if err == nil {
		t.Fatalf("expected canonical collision error, got nil")
	}
	const want = `duplicate provider config after canonicalization: "z-ai" and "zai" both map to "zai"`
	if err.Error() != want {
		t.Fatalf("expected canonical collision error, got %v", err)
	}
}
