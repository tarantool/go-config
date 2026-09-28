package syntax

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	sitter "github.com/odvcencio/gotreesitter"
	"github.com/tarantool/go-config/v2/syntax/internal/cst"
	"github.com/tarantool/go-config/v2/syntax/internal/schema"
)

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
// Cancellation stops parsing and discards the partial tree; a later call can
// reuse the parser. The caller owns and must close the tree.
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

	// Each call owns its flag: a late callback cannot cancel a later parse.
	var canceledFlag uint32

	p.yaml.SetCancellationFlag(&canceledFlag)

	stop := context.AfterFunc(ctx, func() { atomic.StoreUint32(&canceledFlag, 1) })

	defer func() {
		stop()
		p.yaml.SetCancellationFlag(nil)
	}()

	tree, err := p.yaml.ParseStrict(content)

	canceled := ctx.Err()
	if canceled != nil {
		if tree != nil {
			tree.Release()
		}

		return nil, fmt.Errorf("syntax: parse YAML: %w", canceled)
	}

	if err != nil {
		if tree != nil {
			tree.Release()
		}

		return nil, fmt.Errorf("syntax: parse YAML: %w", err)
	}

	return &Tree{source: content, cst: tree, lineStarts: cst.LineStarts(content), schema: p.schema}, nil
}

// Close waits for an ongoing parse and releases the parser resources.
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

	p.yaml = nil

	p.closed = true
}
