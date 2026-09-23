package syntax

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/kaptinlin/jsonschema"
	sitter "github.com/smacker/go-tree-sitter"
)

// Parser holds a compiled schema and a configured YAML syntax parser.
type Parser struct {
	schema *jsonschema.Schema
	yaml   *sitter.Parser
	closed bool
}

// ErrClosedParser is returned when Parse is called after Close.
var ErrClosedParser = errors.New("syntax: parser is closed")

// Parse creates a Tree from a UTF-8 YAML source snapshot.
func (p *Parser) Parse(ctx context.Context, source []byte) (*Tree, error) {
	if p == nil || p.closed {
		return nil, ErrClosedParser
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	snapshot := bytes.Clone(source)
	p.yaml.Reset()

	tree, err := p.yaml.ParseCtx(ctx, nil, snapshot)
	if err != nil {
		return nil, fmt.Errorf("syntax: parse YAML: %w", err)
	}

	return &Tree{source: snapshot, cst: tree, schema: p.schema}, nil
}

// Close releases the native parser.
func (p *Parser) Close() {
	if p == nil || p.closed {
		return
	}

	p.yaml.Close()
	p.closed = true
}
