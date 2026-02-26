package modeldb

import (
	_ "embed"

	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/danshapiro/kilroy/internal/modelmeta"
)

//go:embed pinned/openrouter_models.json
var embeddedCatalogJSON []byte

// LoadEmbeddedCatalog returns a Catalog loaded from the pinned OpenRouter model
// snapshot that is compiled into the binary. The catalog may be stale relative
// to the live OpenRouter API, but it is always available and requires no I/O.
func LoadEmbeddedCatalog() (*Catalog, error) {
	return loadCatalogFromBytes(embeddedCatalogJSON, "<embedded>")
}

func loadCatalogFromBytes(b []byte, source string) (*Catalog, error) {
	sum := sha256.Sum256(b)
	sha := hex.EncodeToString(sum[:])

	var payload openRouterPayload
	if err := json.Unmarshal(b, &payload); err != nil {
		return nil, err
	}

	covered := map[string]bool{}
	models := make(map[string]ModelEntry, len(payload.Data))
	for _, m := range payload.Data {
		id := strings.TrimSpace(m.ID)
		if id == "" {
			continue
		}
		provider := modelmeta.ProviderFromModelID(id)
		if provider != "" {
			covered[provider] = true
		}
		ctxWindow := m.ContextLength
		if ctxWindow == 0 {
			ctxWindow = m.TopProvider.ContextLength
		}
		var maxOut *int
		if m.TopProvider.MaxCompletionTokens > 0 {
			v := m.TopProvider.MaxCompletionTokens
			maxOut = &v
		}
		models[id] = ModelEntry{
			Provider:           provider,
			Mode:               "chat",
			ContextWindow:      ctxWindow,
			MaxOutputTokens:    maxOut,
			SupportsTools:      modelmeta.ContainsFold(m.SupportedParameters, "tools"),
			SupportsReasoning:  modelmeta.ContainsFold(m.SupportedParameters, "reasoning") || modelmeta.ContainsFold(m.SupportedParameters, "include_reasoning"),
			SupportsVision:     modelmeta.ContainsFold(m.Architecture.InputModalities, "image") || modelmeta.ContainsFold(m.Architecture.OutputModalities, "image"),
			InputCostPerToken:  modelmeta.ParseFloatStringPtr(m.Pricing.Prompt),
			OutputCostPerToken: modelmeta.ParseFloatStringPtr(m.Pricing.Completion),
		}
	}

	if len(models) == 0 {
		return nil, fmt.Errorf("openrouter model catalog is empty: %s", source)
	}
	return &Catalog{
		Path:             source,
		SHA256:           sha,
		Models:           models,
		CoveredProviders: covered,
	}, nil
}

type openRouterPayload struct {
	Data []openRouterModel `json:"data"`
}

type openRouterModel struct {
	ID                  string   `json:"id"`
	ContextLength       int      `json:"context_length"`
	SupportedParameters []string `json:"supported_parameters"`
	Architecture        struct {
		InputModalities  []string `json:"input_modalities"`
		OutputModalities []string `json:"output_modalities"`
	} `json:"architecture"`
	Pricing struct {
		Prompt     string `json:"prompt"`
		Completion string `json:"completion"`
	} `json:"pricing"`
	TopProvider struct {
		ContextLength       int `json:"context_length"`
		MaxCompletionTokens int `json:"max_completion_tokens"`
	} `json:"top_provider"`
}

// LoadCatalogFromOpenRouterJSON loads model metadata from OpenRouter
// /api/v1/models payload shape: {"data":[...]}.
func LoadCatalogFromOpenRouterJSON(path string) (*Catalog, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cat, err := loadCatalogFromBytes(b, path)
	if err != nil {
		return nil, err
	}
	// Override the Path field to the actual file path (loadCatalogFromBytes
	// receives a generic source string, so we set the real path here).
	cat.Path = path
	return cat, nil
}
