package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/danshapiro/kilroy/internal/attractor/runtime"
)

type ResolvedArtifactPolicy struct {
	Profiles     []string                   `json:"profiles,omitempty"`
	ManagedRoots map[string]string          `json:"managed_roots,omitempty"`
	Env          ResolvedArtifactEnv        `json:"env,omitempty"`
	Checkpoint   ResolvedArtifactCheckpoint `json:"checkpoint,omitempty"`
}

type ResolvedArtifactEnv struct {
	Vars map[string]string `json:"vars,omitempty"`
}

type ResolvedArtifactCheckpoint struct {
	ExcludeGlobs []string `json:"exclude_globs,omitempty"`
}

type ResolveArtifactPolicyInput struct {
	LogsRoot string `json:"logs_root,omitempty"`
}

type artifactPolicyResolvedEnvelope struct {
	Version int                    `json:"version"`
	Policy  ResolvedArtifactPolicy `json:"policy"`
}

const (
	artifactPolicyResolvedExtraKey = "artifact_policy_resolved"
	artifactPolicyResolvedVersion  = 1
)

var (
	managedRootTemplateRE = regexp.MustCompile(`\{managed_roots\.([A-Za-z0-9_-]+)\}`)
	profileDefaultEnv     = map[string]map[string]string{
		"generic": {},
		"rust": {
			"CARGO_HOME":       "{managed_roots.tool_cache_root}/cargo-home",
			"RUSTUP_HOME":      "{managed_roots.tool_cache_root}/rustup-home",
			"CARGO_TARGET_DIR": "{managed_roots.tool_cache_root}/cargo-target",
		},
		"go": {
			"GOPATH":     "{managed_roots.tool_cache_root}/go-path",
			"GOMODCACHE": "{managed_roots.tool_cache_root}/go-path/pkg/mod",
		},
		"node":   {},
		"python": {},
		"java":   {},
	}
)

func ResolveArtifactPolicy(cfg *RunConfigFile, in ResolveArtifactPolicyInput) (ResolvedArtifactPolicy, error) {
	out := ResolvedArtifactPolicy{
		ManagedRoots: map[string]string{},
		Env:          ResolvedArtifactEnv{Vars: map[string]string{}},
		Checkpoint:   ResolvedArtifactCheckpoint{ExcludeGlobs: []string{}},
	}
	if cfg == nil {
		return out, nil
	}

	profiles := normalizeArtifactPolicyProfiles(cfg.ArtifactPolicy.Profiles)
	if len(profiles) == 0 {
		profiles = []string{"generic"}
	}
	out.Profiles = append([]string{}, profiles...)

	managedRoots := map[string]string{}
	for k, v := range cfg.ArtifactPolicy.Env.ManagedRoots {
		key := strings.TrimSpace(k)
		value := strings.TrimSpace(v)
		if key == "" || value == "" {
			continue
		}
		managedRoots[key] = materializeManagedRoot(value, in.LogsRoot)
	}
	if _, ok := managedRoots["tool_cache_root"]; !ok {
		managedRoots["tool_cache_root"] = materializeManagedRoot("tool-cache", in.LogsRoot)
	}
	out.ManagedRoots = managedRoots

	envVars := map[string]string{}
	for _, profile := range profiles {
		defaults, ok := profileDefaultEnv[profile]
		if !ok {
			return ResolvedArtifactPolicy{}, fmt.Errorf("artifact_policy: unsupported profile %q", profile)
		}
		for k, v := range defaults {
			key := strings.TrimSpace(k)
			if key == "" {
				continue
			}
			envVars[key] = expandManagedRootTemplates(v, managedRoots)
		}
	}
	for _, profile := range profiles {
		overrides, ok := cfg.ArtifactPolicy.Env.Overrides[profile]
		if !ok {
			continue
		}
		for k, v := range overrides {
			key := strings.TrimSpace(k)
			if key == "" {
				continue
			}
			envVars[key] = expandManagedRootTemplates(v, managedRoots)
		}
	}
	for k := range envVars {
		if v, ok := os.LookupEnv(k); ok {
			envVars[k] = v
		}
	}
	out.Env.Vars = envVars

	out.Checkpoint.ExcludeGlobs = append([]string{}, trimNonEmpty(cfg.ArtifactPolicy.Checkpoint.ExcludeGlobs)...)
	return normalizeResolvedArtifactPolicy(out), nil
}

func materializeManagedRoot(value string, logsRoot string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if filepath.IsAbs(trimmed) {
		return filepath.Clean(trimmed)
	}
	base := strings.TrimSpace(logsRoot)
	if base == "" {
		return filepath.Clean(trimmed)
	}
	return filepath.Join(base, "policy-managed-roots", trimmed)
}

func expandManagedRootTemplates(value string, managedRoots map[string]string) string {
	return managedRootTemplateRE.ReplaceAllStringFunc(value, func(match string) string {
		parts := managedRootTemplateRE.FindStringSubmatch(match)
		if len(parts) != 2 {
			return match
		}
		key := strings.TrimSpace(parts[1])
		if key == "" {
			return match
		}
		if v, ok := managedRoots[key]; ok && strings.TrimSpace(v) != "" {
			return v
		}
		return match
	})
}

func normalizeResolvedArtifactPolicy(in ResolvedArtifactPolicy) ResolvedArtifactPolicy {
	out := ResolvedArtifactPolicy{
		Profiles:     append([]string{}, normalizeArtifactPolicyProfiles(in.Profiles)...),
		ManagedRoots: map[string]string{},
		Env:          ResolvedArtifactEnv{Vars: map[string]string{}},
		Checkpoint:   ResolvedArtifactCheckpoint{ExcludeGlobs: append([]string{}, trimNonEmpty(in.Checkpoint.ExcludeGlobs)...)},
	}
	for k, v := range in.ManagedRoots {
		key := strings.TrimSpace(k)
		value := strings.TrimSpace(v)
		if key == "" || value == "" {
			continue
		}
		out.ManagedRoots[key] = value
	}
	for k, v := range in.Env.Vars {
		key := strings.TrimSpace(k)
		if key == "" {
			continue
		}
		out.Env.Vars[key] = v
	}
	return out
}

func restoreArtifactPolicyForResume(cp *runtime.Checkpoint, cfg *RunConfigFile, in ResolveArtifactPolicyInput) (ResolvedArtifactPolicy, error) {
	if cp != nil && cp.Extra != nil {
		if raw, ok := cp.Extra[artifactPolicyResolvedExtraKey]; ok && raw != nil {
			rp, err := decodeResolvedArtifactPolicy(raw)
			if err != nil {
				return ResolvedArtifactPolicy{}, fmt.Errorf("resume: invalid %s snapshot: %w", artifactPolicyResolvedExtraKey, err)
			}
			return rp, nil
		}
	}
	return ResolveArtifactPolicy(cfg, in)
}

func decodeResolvedArtifactPolicy(raw any) (ResolvedArtifactPolicy, error) {
	if rp, ok := raw.(ResolvedArtifactPolicy); ok {
		return normalizeResolvedArtifactPolicy(rp), nil
	}
	if env, ok := raw.(artifactPolicyResolvedEnvelope); ok {
		if env.Version != artifactPolicyResolvedVersion {
			return ResolvedArtifactPolicy{}, fmt.Errorf("unsupported snapshot version %d", env.Version)
		}
		return normalizeResolvedArtifactPolicy(env.Policy), nil
	}

	b, err := json.Marshal(raw)
	if err != nil {
		return ResolvedArtifactPolicy{}, err
	}

	var env artifactPolicyResolvedEnvelope
	if err := json.Unmarshal(b, &env); err == nil && env.Version != 0 {
		if env.Version != artifactPolicyResolvedVersion {
			return ResolvedArtifactPolicy{}, fmt.Errorf("unsupported snapshot version %d", env.Version)
		}
		return normalizeResolvedArtifactPolicy(env.Policy), nil
	}

	var rp ResolvedArtifactPolicy
	if err := json.Unmarshal(b, &rp); err == nil {
		return normalizeResolvedArtifactPolicy(rp), nil
	}

	return ResolvedArtifactPolicy{}, fmt.Errorf("unable to decode resolved artifact policy snapshot")
}
