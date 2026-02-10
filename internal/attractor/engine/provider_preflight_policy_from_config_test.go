package engine

import "testing"

func TestConfiguredAPIPromptProbeTransports_FromConfig(t *testing.T) {
	cfg := &RunConfigFile{}
	applyConfigDefaults(cfg)
	enabled := true
	cfg.Preflight.PromptProbes.Enabled = &enabled
	cfg.Preflight.PromptProbes.Transports = []string{"complete", "stream"}

	got := configuredAPIPromptProbeTransports(cfg, nil)
	if len(got) != 2 || got[0] != "complete" || got[1] != "stream" {
		t.Fatalf("unexpected transports: %v", got)
	}
}

func TestPromptProbeMode_ConfigOverridesEnv(t *testing.T) {
	t.Setenv("KILROY_PREFLIGHT_PROMPT_PROBES", "off")
	cfg := &RunConfigFile{}
	applyConfigDefaults(cfg)
	on := true
	cfg.Preflight.PromptProbes.Enabled = &on
	if got := promptProbeMode(cfg); got != "on" {
		t.Fatalf("mode=%q want on", got)
	}
}
