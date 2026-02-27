package modeldb

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const defaultSuggestURL = "https://openrouter.ai/api/v1/models"
const defaultSuggestTTL = 24 * time.Hour
const defaultSuggestTimeout = 10 * time.Second

// SuggestOptions controls the suggest command behavior.
type SuggestOptions struct {
	CacheDir     string        // defaults to os.UserCacheDir()/kilroy
	TTL          time.Duration // defaults to 24h
	ForceRefresh bool          // bypass TTL, always fetch
	Providers    []string      // filter: ["anthropic","google","openai"] or nil = all three
}

type suggestCache struct {
	FetchedAt time.Time       `json:"fetched_at"`
	Data      json.RawMessage `json:"data"`
}

// Suggest fetches (or loads from cache) the model catalog and formats it as
// plain text suitable for injection into an LLM context.
func Suggest(ctx context.Context, opts SuggestOptions) (string, error) {
	if opts.CacheDir == "" {
		cacheBase, err := os.UserCacheDir()
		if err != nil {
			cacheBase = os.TempDir()
		}
		opts.CacheDir = filepath.Join(cacheBase, "kilroy")
	}
	if opts.TTL <= 0 {
		opts.TTL = defaultSuggestTTL
	}
	if len(opts.Providers) == 0 {
		opts.Providers = []string{"anthropic", "google", "openai"}
	}

	cacheFile := filepath.Join(opts.CacheDir, "modeldb_suggest.json")

	var cat *Catalog
	var sourceLabel string

	// Try cache first (unless ForceRefresh).
	if !opts.ForceRefresh {
		if cached, err := loadSuggestCache(cacheFile, opts.TTL); err == nil && cached != nil {
			cat, err = loadCatalogFromBytes(cached, "<cache>")
			if err == nil {
				sourceLabel = "cache"
			}
		}
	}

	// Cache miss or force refresh: fetch live.
	if cat == nil {
		b, fetchErr := fetchBytes(ctx, defaultSuggestURL, defaultSuggestTimeout)
		if fetchErr == nil && len(b) > 0 {
			// Write cache.
			if mkErr := os.MkdirAll(opts.CacheDir, 0o755); mkErr == nil {
				entry := suggestCache{FetchedAt: time.Now().UTC(), Data: json.RawMessage(b)}
				if enc, encErr := json.Marshal(entry); encErr == nil {
					_ = os.WriteFile(cacheFile, enc, 0o644)
				}
			}
			cat, _ = loadCatalogFromBytes(b, defaultSuggestURL)
			sourceLabel = "openrouter.ai"
		} else {
			// Fetch failed: try stale cache.
			if cached, cacheErr := loadSuggestCache(cacheFile, 0); cacheErr == nil && cached != nil {
				cat, _ = loadCatalogFromBytes(cached, "<stale-cache>")
				if cat != nil {
					fmt.Fprintf(os.Stderr, "WARNING: modeldb fetch failed (%v); using stale cache\n", fetchErr)
					sourceLabel = "stale-cache"
				}
			}
			// Still nil: use embedded.
			if cat == nil {
				var embErr error
				cat, embErr = LoadEmbeddedCatalog()
				if embErr != nil {
					return "", fmt.Errorf("modeldb suggest: fetch failed and embedded catalog unavailable: %w", fetchErr)
				}
				fmt.Fprintf(os.Stderr, "WARNING: modeldb fetch failed (%v); using embedded catalog\n", fetchErr)
				sourceLabel = "embedded"
			}
		}
	}

	return FormatSuggestOutput(cat, opts.Providers, sourceLabel), nil
}

// loadSuggestCache reads the cache file. ttl=0 means "load even if stale".
// Returns nil (no error) when the cache file doesn't exist.
func loadSuggestCache(path string, ttl time.Duration) ([]byte, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, nil // cache miss is not an error
	}
	var entry suggestCache
	if err := json.Unmarshal(b, &entry); err != nil {
		return nil, fmt.Errorf("corrupt cache: %w", err)
	}
	if ttl > 0 && time.Since(entry.FetchedAt) >= ttl {
		return nil, nil // stale
	}
	return entry.Data, nil
}

// modelAnnotations maps model prefix patterns to use-case annotations.
var modelAnnotations = map[string]string{
	"claude-opus":      "flagship, max intelligence",
	"claude-sonnet":    "best speed/quality balance",
	"claude-haiku":     "fastest, cost-optimized",
	"gemini-3-flash":   "fast, highly capable, 1M context",
	"gemini-2.5-pro":   "high intelligence, large context",
	"gemini-2.5-flash": "fast, cost-effective",
	"gpt-4.1":          "flagship",
	"gpt-4o":           "multimodal, strong reasoning",
	"gpt-4o-mini":      "fast, cost-optimized",
	"o3":               "deep reasoning",
	"o4-mini":          "fast reasoning",
}

var defaultModels = map[string]string{
	"claude-sonnet":  "anthropic",
	"gemini-3-flash": "google",
	"gpt-4.1":        "openai",
}

// FormatSuggestOutput renders the catalog as plain text with use-case annotations.
func FormatSuggestOutput(catalog *Catalog, providers []string, sourceLabel string) string {
	if catalog == nil {
		return "## modeldb suggest: no catalog available\n"
	}

	date := time.Now().UTC().Format("2006-01-02")
	if sourceLabel == "" {
		sourceLabel = "unknown"
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "## Available model IDs (kilroy modeldb, %s, source: %s)\n", date, sourceLabel)
	fmt.Fprintf(&sb, "## Use ONLY these exact IDs in model_stylesheet.\n\n")

	providerOrder := []string{"anthropic", "google", "openai"}
	providerHeadings := map[string]string{
		"anthropic": "ANTHROPIC (use dot notation: claude-opus-4.6 not claude-opus-4-6)",
		"google":    "GOOGLE",
		"openai":    "OPENAI",
	}

	// Build allowed provider set from opts.
	allowed := map[string]bool{}
	for _, p := range providers {
		allowed[strings.ToLower(p)] = true
	}

	for _, prov := range providerOrder {
		if !allowed[prov] {
			continue
		}

		// Collect models for this provider.
		var models []string
		for id, entry := range catalog.Models {
			ep := entry.Provider
			if ep == "" {
				// Infer from ID prefix.
				if parts := strings.SplitN(id, "/", 2); len(parts) == 2 {
					ep = strings.ToLower(parts[0])
				}
			}
			ep = strings.ToLower(ep)
			if ep != prov {
				continue
			}
			// Strip provider prefix for display.
			displayID := id
			if idx := strings.Index(id, "/"); idx >= 0 {
				displayID = id[idx+1:]
			}
			// Skip non-text-generation models (vision-only, embedding, etc.)
			if entry.Mode != "" && entry.Mode != "chat" {
				continue
			}
			models = append(models, displayID)
		}
		sort.Strings(models)

		if len(models) == 0 {
			continue
		}

		fmt.Fprintf(&sb, "%s\n", providerHeadings[prov])
		for _, m := range models {
			annotation := ""
			for prefix, ann := range modelAnnotations {
				if strings.HasPrefix(m, prefix) {
					annotation = ann
					break
				}
			}
			isDefault := false
			for prefix, dp := range defaultModels {
				if strings.HasPrefix(m, prefix) && dp == prov {
					isDefault = true
					break
				}
			}
			line := fmt.Sprintf("  %-40s", m)
			if annotation != "" {
				line += annotation
			}
			if isDefault {
				line += "  [recommended default]"
			}
			fmt.Fprintf(&sb, "%s\n", strings.TrimRight(line, " "))
		}
		fmt.Fprintln(&sb)
	}

	fmt.Fprintf(&sb, "## NOTE: gemini-2.0-flash-exp is RETIRED — use gemini-2.5-flash or gemini-3-flash-preview\n")
	fmt.Fprintf(&sb, "## NOTE: claude-opus-4-6 (with dashes) is WRONG — use claude-opus-4.6 (with dots)\n")

	return sb.String()
}
