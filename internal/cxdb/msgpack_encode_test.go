package cxdb

import (
	"testing"

	"github.com/vmihailenco/msgpack/v5"
)

func TestEncodeTurnPayload_UsesRegistryFieldTags(t *testing.T) {
	raw, err := EncodeTurnPayload("com.kilroy.attractor.RunStarted", 1, map[string]any{
		"run_id":       "r1",
		"timestamp_ms": uint64(42),
		"repo_path":    "/tmp/repo",
		"unknown_key":  "ignored",
	})
	if err != nil {
		t.Fatalf("EncodeTurnPayload: %v", err)
	}

	var got map[string]any
	if err := msgpack.Unmarshal(raw, &got); err != nil {
		t.Fatalf("msgpack.Unmarshal: %v", err)
	}

	if got["1"] != "r1" {
		t.Fatalf("field tag 1 (run_id): got=%v", got["1"])
	}
	if got["3"] != "/tmp/repo" {
		t.Fatalf("field tag 3 (repo_path): got=%v", got["3"])
	}
	if got["2"] == nil {
		t.Fatalf("field tag 2 (timestamp_ms) missing")
	}
	if _, ok := got["unknown_key"]; ok {
		t.Fatalf("unknown key should be omitted from msgpack payload")
	}
}
