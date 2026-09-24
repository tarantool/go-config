// Package syntax provides schema-aware YAML parsing for editor operations.
// Parsing tolerates incomplete YAML so a tree remains available for
// diagnostics and completion while it is being edited.
package syntax

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/kaptinlin/jsonschema"
	sitter "github.com/smacker/go-tree-sitter"
	yamlgrammar "github.com/smacker/go-tree-sitter/yaml"
)

// ErrNoJSONSchema is returned by Build when no JSON Schema was provided.
var ErrNoJSONSchema = errors.New("syntax: JSON Schema is required")

// Builder configures a reusable YAML parser backed by one JSON Schema.
type Builder struct {
	schema []byte
}

// NewBuilder creates an empty parser builder.
func NewBuilder() *Builder {
	return &Builder{}
}

// WithJSONSchema sets the JSON Schema used by the parser.
func (b *Builder) WithJSONSchema(schema []byte) *Builder {
	b.schema = bytes.Clone(schema)
	return b
}

// Build compiles the schema, resolves its references, and configures a
// tree-sitter YAML parser. Close the returned Parser when it is no longer used.
func (b *Builder) Build(ctx context.Context) (*Parser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if b == nil || len(b.schema) == 0 {
		return nil, ErrNoJSONSchema
	}

	schema, err := jsonschema.NewCompiler().Compile(b.schema)
	if err != nil {
		return nil, fmt.Errorf("syntax: compile JSON Schema: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	yamlParser := sitter.NewParser()
	yamlParser.SetLanguage(yamlgrammar.GetLanguage())
	return &Parser{schema: schema, yaml: yamlParser}, nil
}
