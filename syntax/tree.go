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

// Metadata describes a field or array item without prescribing a display format.
// Descriptions are copied verbatim from JSON Schema. Child properties and array
// items are not included. Composition groups retain their branch associations;
// AllOf also contains metadata from references and overlapping property patterns.
// Empty AnyOf and OneOf entries retain permitted alternatives without exposed
// metadata; they may still constrain values.
// The returned values are independent of the compiled schema and may be modified.
type Metadata = schema.Metadata

// HoverItem contains metadata for one tooltip and its half-open source range.
// Formatting belongs to the caller; Metadata never contains generated markup.
// A nil Metadata means there is no tooltip.
// Range uses zero-based UTF-8 byte columns. The result remains usable after
// the tree is closed; an LSP adapter converts the range to client coordinates.
type HoverItem struct {
	Metadata *Metadata
	Range    Range
}

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

// Hover describes the key or scalar value at pos in the parsed source.
// It returns an error for an unavailable tree or invalid coordinates, and a zero
// HoverItem when there is no target or no schema documentation.
// Alternatives are described regardless of the current value. Container keys
// describe their own field, without expanding its properties or array items.
func (t *Tree) Hover(pos Position) (HoverItem, error) {
	var empty HoverItem

	if t == nil || t.cst == nil {
		return empty, fmt.Errorf("hover: locate node: %w", errUnavailableTree)
	}

	point := sitter.Point{Row: pos.Line, Column: pos.ByteColumn}

	location, err := cst.Locate(t.cst.RootNode(), t.source, t.lineStarts, point)
	if err != nil {
		return empty, fmt.Errorf("hover: locate node: %w", err)
	}

	node := cst.ScalarNode(location.Node)
	if node == nil || !location.Contains(node) {
		return empty, nil
	}

	context := cst.NodeContext(t.source, t.schema, node)
	if cst.IsScalarValue(node) {
		// Completion may interpret an object-valued position as a place for
		// new keys. Hover describes an existing scalar at that position.
		context.IsKey = false
	}

	metadata := t.schema.MetadataAt(cst.FieldPath(t.source, context))
	if metadata == nil {
		return empty, nil
	}

	start, end := node.StartPoint(), node.EndPoint()

	return HoverItem{
		Metadata: metadata,
		Range: Range{
			Start: Position{Line: start.Row, ByteColumn: start.Column},
			End:   Position{Line: end.Row, ByteColumn: end.Column},
		},
	}, nil
}
