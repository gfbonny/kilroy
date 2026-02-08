package cxdb

import (
	"strconv"
	"strings"

	"github.com/vmihailenco/msgpack/v5"
)

// EncodeTurnPayload converts named JSON-like turn data into msgpack keyed by
// numeric field tags (as strings) using Kilroy's published registry bundle.
func EncodeTurnPayload(typeID string, typeVersion int, data map[string]any) ([]byte, error) {
	if data == nil {
		data = map[string]any{}
	}

	fieldTags := registryFieldTags(typeID, typeVersion)
	out := make(map[string]any, len(data))
	for k, v := range data {
		if tag, ok := fieldTags[k]; ok {
			out[tag] = normalizeMsgpackValue(v)
			continue
		}
		// Allow callers to pass explicit numeric-tag keys.
		if _, err := strconv.ParseUint(strings.TrimSpace(k), 10, 64); err == nil {
			out[k] = normalizeMsgpackValue(v)
		}
	}
	return msgpack.Marshal(out)
}

func registryFieldTags(typeID string, typeVersion int) map[string]string {
	_, bundle, _, err := KilroyAttractorRegistryBundle()
	if err != nil {
		return map[string]string{}
	}
	typeSpec, ok := bundle.Types[typeID].(map[string]any)
	if !ok {
		return map[string]string{}
	}
	versions, ok := typeSpec["versions"].(map[string]any)
	if !ok {
		return map[string]string{}
	}
	versionSpec, ok := versions[strconv.Itoa(typeVersion)].(map[string]any)
	if !ok {
		return map[string]string{}
	}
	fields, ok := versionSpec["fields"].(map[string]any)
	if !ok {
		return map[string]string{}
	}
	out := make(map[string]string, len(fields))
	for tag, raw := range fields {
		fieldSpec, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		name, _ := fieldSpec["name"].(string)
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		out[name] = strings.TrimSpace(tag)
	}
	return out
}

func normalizeMsgpackValue(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, v := range x {
			out[k] = normalizeMsgpackValue(v)
		}
		return out
	case []any:
		out := make([]any, 0, len(x))
		for _, v := range x {
			out = append(out, normalizeMsgpackValue(v))
		}
		return out
	default:
		return v
	}
}
