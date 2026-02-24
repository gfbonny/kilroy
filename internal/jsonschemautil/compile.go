package jsonschemautil

import (
	"bytes"
	"encoding/json"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

// CompileMapSchema compiles an in-memory JSON schema map without relying on
// process working directory resolution.
func CompileMapSchema(schema map[string]any, draft *jsonschema.Draft) (*jsonschema.Schema, error) {
	c := jsonschema.NewCompiler()
	if draft != nil {
		c.Draft = draft
	}
	b, err := json.Marshal(schema)
	if err != nil {
		return nil, err
	}
	const uri = "mem://schema/inline.json"
	if err := c.AddResource(uri, bytes.NewReader(b)); err != nil {
		return nil, err
	}
	return c.Compile(uri)
}
