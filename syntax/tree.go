package syntax

import (
	"bytes"

	"github.com/kaptinlin/jsonschema"
	sitter "github.com/smacker/go-tree-sitter"
)

// Tree is a parsed YAML source snapshot with a tolerant syntax tree and the
// schema selected when the Parser was built.
type Tree struct {
	source []byte
	cst    *sitter.Tree
	schema *jsonschema.Schema
}

// Source returns a copy of the original YAML source.
func (t *Tree) Source() []byte {
	if t == nil {
		return nil
	}

	return bytes.Clone(t.source)
}

// Close releases resources held by the syntax tree.
func (t *Tree) Close() {
	if t == nil || t.cst == nil {
		return
	}

	t.cst.Close()
}
