package syntax

import (
	"bytes"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/tarantool/go-config/v2/syntax/internal/cst"
	"github.com/tarantool/go-config/v2/syntax/internal/schema"
)

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
