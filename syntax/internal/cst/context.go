// Package cst locates YAML nodes and resolves editor contexts in a tolerant CST.
package cst

import (
	"slices"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/tarantool/go-config/v2/syntax/internal/path"
	"github.com/tarantool/go-config/v2/syntax/internal/schema"
	"go.yaml.in/yaml/v3"
)

const (
	nodeBlockMapping      = "block_mapping"
	nodeBlockMappingPair  = "block_mapping_pair"
	nodeBlockSequence     = "block_sequence"
	nodeBlockSequenceItem = "block_sequence_item"
	nodeComment           = "comment"
	nodeError             = "ERROR"
	nodeFlowMapping       = "flow_mapping"
	nodeFlowNode          = "flow_node"
	nodeFlowPair          = "flow_pair"
	nodeFlowSequence      = "flow_sequence"
	nodePlainScalar       = "plain_scalar"
)

// Context identifies the schema path and syntactic role of a CST node.
// For a key, Path addresses the containing object; for a value, the value itself.
type Context struct {
	IsKey     bool // Whether the target is a property name rather than its value.
	Path      []path.Step
	Mapping   *sitter.Node // Enclosing Mapping or ERROR scope for duplicate checks; nil if unknown.
	Current   *sitter.Node // Edited pair or bare Flow key, excluded from duplicate checks.
	Flow      bool         // Inside braces or brackets, where block-style insertion is invalid.
	Separator *sitter.Node // Recovered colon immediately before the cursor, if any.
}

// IsScalar recognizes concrete tree-sitter scalar nodes.
// The plain_scalar wrapper is excluded; its typed child carries the content.
func IsScalar(node *sitter.Node) bool {
	return node != nil && strings.HasSuffix(node.Type(), "_scalar") && node.Type() != nodePlainScalar
}

// ScalarNode unwraps YAML decorations and returns a scalar, or nil for
// containers and wrappers without an unambiguous scalar value.
func ScalarNode(node *sitter.Node) *sitter.Node {
	if node = ContentNode(node); IsScalar(node) {
		return node
	}

	return nil
}

// ContentNode unwraps YAML nodes, ignoring anchors and comments.
// A wrapper containing only these decorations has no value.
func ContentNode(node *sitter.Node) *sitter.Node {
	for node != nil {
		switch node.Type() {
		case "document", "block_node", nodeFlowNode, nodePlainScalar, nodeBlockSequenceItem:
			var content *sitter.Node

			for index := range int(node.NamedChildCount()) {
				child := node.NamedChild(index)
				switch child.Type() {
				case nodeComment, "anchor":
					continue
				}

				if content != nil {
					return nil
				}

				content = child
			}

			node = content
		default:
			return node
		}
	}

	return nil
}

// ChildOfType finds an immediate CST child, including punctuation such as
// colons and dashes that tree-sitter does not expose through named fields.
func ChildOfType(node *sitter.Node, nodeType string) *sitter.Node {
	for index := range int(node.ChildCount()) {
		if child := node.Child(index); child.Type() == nodeType {
			return child
		}
	}

	return nil
}

// sequenceIndex counts preceding sequence items, skipping comment nodes.
func sequenceIndex(parent, child *sitter.Node) int {
	index := 0

	for sibling := range int(parent.NamedChildCount()) {
		candidate := parent.NamedChild(sibling)
		if candidate.Equal(child) {
			break
		}

		if candidate.Type() != nodeComment {
			index++
		}
	}

	return index
}

// ContextAtPath chooses keys for an object schema and scalar values otherwise.
// Object keys take precedence when both object and scalar branches are allowed.
func ContextAtPath(compiled *schema.Schema, steps []path.Step) Context {
	return Context{
		IsKey:     compiled.HasObjectSchema(steps),
		Path:      steps,
		Mapping:   nil,
		Current:   nil,
		Flow:      false,
		Separator: nil,
	}
}

// FieldPath returns the path to the field represented by context. For a key,
// Context.Path addresses its containing object; this adds the decoded key.
// Values and containers already carry their own path. No CST ancestor walk is
// needed, so completion and inspection can share the same resolved context.
func FieldPath(source []byte, context Context) []path.Step {
	if !context.IsKey || context.Current == nil {
		return context.Path
	}

	key := context.Current.ChildByFieldName("key")
	if key == nil {
		key = ScalarNode(context.Current)
	}

	if key == nil {
		return context.Path
	}

	return append(slices.Clone(context.Path), path.Property(KeyText(source, key)))
}

// KeyText decodes a YAML property name, including quoted and escaped keys.
// An absent key is empty; malformed keys fall back to their source text.
func KeyText(source []byte, node *sitter.Node) string {
	if node == nil {
		return ""
	}

	raw := node.Content(source)

	var decoded string

	err := yaml.Unmarshal([]byte(raw), &decoded)
	if err == nil {
		return decoded
	}

	return raw
}

// recoveryScope is one open mapping or sequence inside a flattened ERROR node.
// path addresses the container; index tracks its current sequence item, and
// column/delimiter determine when indentation or punctuation closes the scope.
type recoveryScope struct {
	path      []path.Step
	column    uint32
	delimiter string
	index     int
}

// NodeContext walks CST ancestors to determine the schema path, key/value role,
// and containing mapping. For incomplete parents it delegates to ErrorContext.
func NodeContext(source []byte, compiled *schema.Schema, node *sitter.Node) Context {
	var ancestors []*sitter.Node

	for current := node; current != nil; current = current.Parent() {
		ancestors = append(ancestors, current)
	}

	context := ContextAtPath(compiled, nil)

	for index := len(ancestors) - 1; index > 0; index-- {
		parent, child := ancestors[index], ancestors[index-1]

		switch parent.Type() {
		case nodeBlockMapping, nodeFlowMapping:
			context.IsKey = true
			context.Mapping = parent
			context.Current = child
			context.Flow = context.Flow || parent.Type() == nodeFlowMapping
		case nodeBlockMappingPair, nodeFlowPair:
			key := parent.ChildByFieldName("key")

			if key != nil && child.Equal(key) {
				context.IsKey = true
				context.Current = parent
			} else {
				context.Path = append(slices.Clone(context.Path), path.Property(KeyText(source, key)))
				context.IsKey = false
				context.Mapping = nil
				context.Current = nil

				if key != nil && node.StartPoint().Row > key.EndPoint().Row &&
					compiled.HasObjectSchema(context.Path) {
					context.IsKey = true
				}
			}
		case nodeFlowSequence, nodeBlockSequence:
			context.Path = append(slices.Clone(context.Path), path.Index(sequenceIndex(parent, child)))
			context.IsKey = false
			context.Mapping = nil
			context.Current = nil
			context.Flow = context.Flow || parent.Type() == nodeFlowSequence

			if compiled.HasObjectSchema(context.Path) {
				context.IsKey = true
			}
		case nodeError:
			point := child.StartPoint()

			context, _ = ErrorContext(
				source, compiled, parent, context, point, int(child.StartByte()),
			)
		}
	}

	// A pair names a field even if recovery returned only its enclosing scope.
	if node != nil && node.ChildByFieldName("key") != nil {
		context.IsKey = true
		context.Current = node
	}

	if context.IsKey && context.Current == nil && IsScalar(node) {
		context.Current = node
	}

	return context
}

// ErrorContext reconstructs paths flattened into a tree-sitter ERROR node.
// It tracks indentation and flow delimiters up to offset and returns the token
// at the cursor when available. Complete subtrees use normal ancestor traversal;
// this does not repair the YAML or reparse it with synthetic text.
func ErrorContext(
	source []byte, compiled *schema.Schema, node *sitter.Node, context Context, pos sitter.Point, offset int,
) (Context, *sitter.Node) {
	base := context.Path

	context.Mapping = node

	var (
		blocks, flows []recoveryScope
		pending       *sitter.Node
		previousRow   uint32
	)

	for index := range int(node.ChildCount()) {
		child := node.Child(index)
		if int(child.StartByte()) > offset {
			break
		}

		if child.Type() == nodeComment {
			if offset == int(child.StartByte()) {
				return context, nil
			}

			if offset <= int(child.EndByte()) {
				return context, child
			}

			continue
		}

		point := child.StartPoint()
		if len(flows) == 0 && (index == 0 || point.Row > previousRow) {
			for len(blocks) > 0 && point.Column <= blocks[len(blocks)-1].column {
				last := blocks[len(blocks)-1]
				// A sequence may start at the same indentation as its mapping key.
				if point.Column == last.column &&
					(child.Type() == "-" || child.Type() == nodeBlockSequenceItem) {
					break
				}

				blocks = blocks[:len(blocks)-1]
			}

			steps := base

			if len(blocks) > 0 {
				last := blocks[len(blocks)-1]

				steps = last.path

				if last.delimiter == "-" {
					steps = append(slices.Clone(steps), path.Index(last.index))
				}
			}

			context = ContextAtPath(compiled, slices.Clone(steps))
			context.Mapping = node
		}

		previousRow = point.Row

		switch child.Type() {
		case nodeFlowPair, nodeBlockMappingPair:
			if offset <= int(child.EndByte()) {
				return context, nil
			}

			if len(flows) > 0 {
				key := path.Property(KeyText(source, child.ChildByFieldName("key")))

				context.Path = append(slices.Clone(flows[len(flows)-1].path), key)
				context.IsKey = false
			}
		case nodeFlowNode, "block_node", nodePlainScalar:
			if offset <= int(child.EndByte()) || point.Row == pos.Row &&
				strings.TrimSpace(string(source[child.EndByte():offset])) == "" {
				if context.IsKey {
					context.Current = child
				}

				return context, ScalarNode(child)
			}

			pending = child
		case ":":
			if len(flows) > 0 {
				last := flows[len(flows)-1]

				context.Path = append(slices.Clone(last.path), path.Property(KeyText(source, pending)))
			} else {
				column := point.Column
				if pending != nil {
					column = pending.StartPoint().Column
				}

				context.Path = append(slices.Clone(context.Path), path.Property(KeyText(source, pending)))
				blocks = append(blocks, recoveryScope{path: context.Path, column: column, delimiter: ":", index: 0})
			}

			context.IsKey = false
			if offset == int(child.EndByte()) {
				context.Separator = child
			}
		case "[", "{":
			flows = append(flows, recoveryScope{
				path:      slices.Clone(context.Path),
				delimiter: child.Type(),
				column:    0,
				index:     0,
			})
			context.Flow = true
			context.Separator = nil

			if child.Type() == "[" {
				context.IsKey = false
				context.Path = append(context.Path, path.Index(0))
			} else {
				context.IsKey = true
			}
		case "-", nodeBlockSequenceItem:
			if len(blocks) > 0 && blocks[len(blocks)-1].delimiter == "-" &&
				blocks[len(blocks)-1].column == point.Column {
				blocks[len(blocks)-1].index++
			} else {
				blocks = append(blocks, recoveryScope{
					path:      slices.Clone(context.Path),
					delimiter: "-",
					column:    point.Column,
					index:     0,
				})
			}

			last := blocks[len(blocks)-1]

			context = ContextAtPath(compiled, append(slices.Clone(last.path), path.Index(last.index)))

			if child.Type() == nodeBlockSequenceItem && offset <= int(child.EndByte()) {
				return context, nil
			}
		case ",":
			if len(flows) > 0 {
				last := &flows[len(flows)-1]

				context.Path = slices.Clone(last.path)
				context.IsKey = true

				if last.delimiter == "[" {
					last.index++

					context.Path = append(context.Path, path.Index(last.index))
					context.IsKey = false
				}
			}
		case "]", "}":
			if len(flows) > 0 {
				context.Path = slices.Clone(flows[len(flows)-1].path)
				flows = flows[:len(flows)-1]
				context.IsKey = false
				context.Flow = len(flows) > 0
			}
		case "\"", "'":
			return context, child
		}
	}

	return context, nil
}
