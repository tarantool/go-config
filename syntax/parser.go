package syntax

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/tarantool/go-config/v2/syntax/internal/cst"
	"github.com/tarantool/go-config/v2/syntax/internal/schema"
)

const inputChunkSize = 4096

// Parser holds a compiled schema and a configured YAML syntax parser.
// Parse and Close may be called concurrently; operations are serialized internally.
// Returned trees have independent lifetimes and remain usable after the parser
// is closed. A Parser must not be copied after first use.
type Parser struct {
	mu     sync.Mutex
	schema *schema.Schema
	yaml   *sitter.Parser
	closed bool
}

// ErrClosedParser is returned when Parse is called after Close.
var ErrClosedParser = errors.New("syntax: parser is closed")

// Parse copies the UTF-8 source into a Tree, tolerating incomplete YAML.
// Cancellation is checked between input chunks and discards the partial tree;
// a later call can reuse the parser. The caller owns and must close the tree.
func (p *Parser) Parse(ctx context.Context, source []byte) (*Tree, error) {
	if p == nil {
		return nil, ErrClosedParser
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil, ErrClosedParser
	}

	err := ctx.Err()
	if err != nil {
		return nil, fmt.Errorf("syntax: parse YAML: %w", err)
	}

	content := bytes.Clone(source)

	p.yaml.Reset()

	// ParseCtx leaves a cancellation goroutine racing with parser reuse.
	// Check cancellation synchronously between bounded input chunks instead.
	tree, err := p.yaml.ParseInputCtx(ctx, nil, sitter.Input{
		Encoding: sitter.InputEncodingUTF8,
		Read: func(offset uint32, _ sitter.Point) []byte {
			if ctx.Err() != nil || int(offset) >= len(content) {
				return nil
			}

			return content[offset:min(int(offset)+inputChunkSize, len(content))]
		},
	})

	canceled := ctx.Err()
	if canceled != nil {
		if tree != nil {
			tree.Close()
		}

		return nil, fmt.Errorf("syntax: parse YAML: %w", canceled)
	}

	if err != nil {
		return nil, fmt.Errorf("syntax: parse YAML: %w", err)
	}

	return &Tree{source: content, cst: tree, lineStarts: cst.LineStarts(content), schema: p.schema}, nil
}

// Close waits for an ongoing parse and releases the native parser.
// It is safe to call more than once or on a nil parser.
func (p *Parser) Close() {
	if p == nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return
	}

	p.yaml.Close()

	p.closed = true
}
