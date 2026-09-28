// Package syntax provides schema-aware YAML parsing for editor operations.
// Parsing tolerates incomplete YAML so a tree remains available for
// diagnostics and completion while it is being edited.
package syntax

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"

	sitter "github.com/odvcencio/gotreesitter"
	yamlgrammar "github.com/odvcencio/gotreesitter/grammars/yaml"
	"github.com/tarantool/go-config/v2/syntax/internal/schema"
)

// ErrNoJSONSchema is returned by Build when no JSON Schema was provided.
var ErrNoJSONSchema = errors.New("syntax: JSON Schema is required")

// ErrUnresolvedReference is returned when a schema reference cannot be resolved.
var ErrUnresolvedReference = schema.ErrUnresolvedReference

// Builder configures a reusable YAML parser backed by one JSON Schema.
type Builder struct {
	schema []byte
}

// NewBuilder creates an empty parser builder.
func NewBuilder() *Builder {
	return new(Builder)
}

// WithJSONSchema sets the JSON Schema used by the parser.
func (b *Builder) WithJSONSchema(schema []byte) *Builder {
	b.schema = bytes.Clone(schema)
	return b
}

// Build compiles the schema, resolves its references, and configures a
// tree-sitter YAML parser. Close the returned Parser when it is no longer used.
func (b *Builder) Build(ctx context.Context) (*Parser, error) {
	err := ctx.Err()
	if err != nil {
		return nil, fmt.Errorf("syntax: build parser: %w", err)
	}

	if b == nil || len(b.schema) == 0 {
		return nil, ErrNoJSONSchema
	}

	compiled, err := schema.Compile(b.schema)
	if err != nil {
		return nil, fmt.Errorf("syntax: build parser: %w", err)
	}

	err = ctx.Err()
	if err != nil {
		return nil, fmt.Errorf("syntax: build parser: %w", err)
	}

	yamlParser := sitter.NewParser(yamlgrammar.Language())

	return &Parser{mu: sync.Mutex{}, schema: compiled, yaml: yamlParser, closed: false}, nil
}
