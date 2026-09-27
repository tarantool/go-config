package syntax

import (
	"bytes"
	"errors"
	"fmt"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/tarantool/go-config/v2/syntax/internal/cst"
	"github.com/tarantool/go-config/v2/syntax/internal/schema"
)

var errUnavailableTree = errors.New("syntax: tree is unavailable")

// Tree owns a tolerant syntax tree and its immutable YAML source.
// It shares the schema prepared by Builder with other trees. Operations on the
// same tree must be serialized. Close releases the syntax tree; the Tree
// remains usable after its Parser is closed.
type Tree struct {
	source     []byte
	cst        *sitter.Tree
	lineStarts []int
	schema     *schema.Schema
}

// Position is a zero-based location in the original UTF-8 YAML source.
// ByteColumn counts bytes, not Unicode code points.
type Position = cst.Position

// Range is a half-open interval in the original source.
type Range = cst.Range

// CompletionItem contains a plain-text YAML edit in the original source.
// Replace is single-line and uses byte columns.
// Type is empty when the schema type is unknown or ambiguous.
type CompletionItem = cst.CompletionItem

// Source returns a copy of the original UTF-8 YAML bytes. Positions refer to
// the source passed to Parse.
func (t *Tree) Source() []byte {
	if t == nil {
		return nil
	}

	return bytes.Clone(t.source)
}

// Close releases the syntax tree. It is safe to call more than once
// or on a nil tree.
func (t *Tree) Close() {
	if t == nil || t.cst == nil {
		return
	}

	t.cst.Close()

	t.cst = nil
}

// Completion returns property and scalar suggestions for the parsed source.
// It returns an error for an unavailable tree or invalid coordinates, and
// no suggestions for unsupported contexts or values without finite candidates.
func (t *Tree) Completion(pos Position) ([]CompletionItem, error) {
	if t == nil || t.cst == nil {
		return nil, fmt.Errorf("completion: locate cursor: %w", errUnavailableTree)
	}

	point := sitter.Point{Row: pos.Line, Column: pos.ByteColumn}

	items, err := cst.CompletionAt(t.cst.RootNode(), t.source, t.lineStarts, t.schema, point)
	if err != nil {
		return nil, fmt.Errorf("completion: locate cursor: %w", err)
	}

	return items, nil
}
