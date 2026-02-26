package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/danshapiro/kilroy/internal/llm"
)

type InputDocForInference struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type InputInferenceOptions struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

type InferredReference struct {
	Pattern    string `json:"pattern"`
	Rationale  string `json:"rationale"`
	Confidence string `json:"confidence"`
}

type InputReferenceInferer interface {
	Infer(ctx context.Context, docs []InputDocForInference, opts InputInferenceOptions) ([]InferredReference, error)
}

type llmInputReferenceInferer struct {
	client *llm.Client
}

func newInputReferenceInfererFromRuntimes(runtimes map[string]ProviderRuntime) (InputReferenceInferer, error) {
	client, err := newAPIClientFromProviderRuntimes(runtimes)
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, fmt.Errorf("input reference inferer requires an API client")
	}
	return &llmInputReferenceInferer{client: client}, nil
}

func (i *llmInputReferenceInferer) Infer(ctx context.Context, docs []InputDocForInference, opts InputInferenceOptions) ([]InferredReference, error) {
	if i == nil || i.client == nil {
		return nil, fmt.Errorf("input reference inferer is not configured")
	}
	if strings.TrimSpace(opts.Provider) == "" || strings.TrimSpace(opts.Model) == "" {
		return nil, fmt.Errorf("input inference requires provider and model")
	}
	if len(docs) == 0 {
		return nil, nil
	}

	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"references": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"pattern":    map[string]any{"type": "string"},
						"rationale":  map[string]any{"type": "string"},
						"confidence": map[string]any{"type": "string"},
					},
					"required":             []string{"pattern", "rationale", "confidence"},
					"additionalProperties": false,
				},
			},
		},
		"required":             []string{"references"},
		"additionalProperties": false,
	}

	req := llm.Request{
		Provider: opts.Provider,
		Model:    opts.Model,
		Messages: []llm.Message{
			llm.System("Return strictly valid JSON matching the schema. Infer input file path/glob references required by the docs. Prefer broad inclusion when uncertain."),
			llm.User(buildInputInferencePrompt(docs)),
		},
		ResponseFormat: &llm.ResponseFormat{
			Type:       "json_schema",
			Strict:     true,
			JSONSchema: schema,
		},
	}
	resp, err := i.client.Complete(ctx, req)
	if err != nil {
		return nil, err
	}
	refs, err := parseInferredReferencePayload(resp.Text())
	if err != nil {
		return nil, err
	}
	return refs, nil
}

func buildInputInferencePrompt(docs []InputDocForInference) string {
	var b strings.Builder
	b.WriteString("Given these documents, infer additional file path or glob references needed to satisfy requirements.\n")
	b.WriteString("Return conservative over-inclusive patterns when uncertain.\n\n")
	for idx, doc := range docs {
		path := strings.TrimSpace(doc.Path)
		if path == "" {
			path = fmt.Sprintf("doc_%d", idx+1)
		}
		b.WriteString("### Document: ")
		b.WriteString(path)
		b.WriteString("\n")
		b.WriteString(doc.Content)
		b.WriteString("\n\n")
	}
	return b.String()
}

func parseInferredReferencePayload(raw string) ([]InferredReference, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var wrapped struct {
		References []InferredReference `json:"references"`
	}
	if err := json.Unmarshal([]byte(raw), &wrapped); err == nil {
		return normalizeInferredReferences(wrapped.References), nil
	}
	var direct []InferredReference
	if err := json.Unmarshal([]byte(raw), &direct); err == nil {
		return normalizeInferredReferences(direct), nil
	}
	return nil, fmt.Errorf("invalid inferred reference payload")
}

func normalizeInferredReferences(in []InferredReference) []InferredReference {
	if len(in) == 0 {
		return nil
	}
	out := make([]InferredReference, 0, len(in))
	seen := map[string]bool{}
	for _, ref := range in {
		pattern := normalizeReferenceToken(ref.Pattern)
		if pattern == "" {
			continue
		}
		if !looksLikeStructuredReferenceToken(pattern) {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(pattern))
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, InferredReference{
			Pattern:    pattern,
			Rationale:  strings.TrimSpace(ref.Rationale),
			Confidence: strings.TrimSpace(ref.Confidence),
		})
	}
	sortInferredReferences(out)
	return out
}

func sortInferredReferences(in []InferredReference) {
	if len(in) == 0 {
		return
	}
	sort.SliceStable(in, func(i, j int) bool {
		return in[i].Pattern < in[j].Pattern
	})
}
