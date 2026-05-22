package config

import (
	"bytes"
	"fmt"
	"math"
	"strconv"

	"github.com/tarantool/go-config/meta"
	"github.com/tarantool/go-config/tree"
	"go.yaml.in/yaml/v3"
)

const (
	yamlIndent    = 2
	yamlFloatTag  = "!!float"
	yamlNullTag   = "!!null"
	yamlStringTag = "!!str"
)

// YAMLAnnotation captures the YAML-specific information needed to faithfully
// re-emit a tree node: scalar style, comments, and tag. It is attached to
// tree.Node by the YAML collector and consumed by Config.MarshalYAML.
//
// Key holds the *yaml.Node corresponding to this entry's key in its parent
// mapping (nil for sequence items, document roots, and programmatically-added
// nodes). Val holds the *yaml.Node corresponding to this node's value.
type YAMLAnnotation struct {
	Key *yaml.Node
	Val *yaml.Node
}

// String returns the configuration serialized as YAML.
// Returns an empty string if marshaling fails.
func (c *Config) String() string {
	out, err := c.MarshalYAML()
	if err != nil {
		return ""
	}

	return string(out)
}

// MarshalYAML serializes the Config as YAML.
//
// Key insertion order is preserved across maps. For nodes that came from a
// YAML source and have not been mutated, the original scalar style and the
// surrounding head/line/foot comments are preserved. Programmatically-added
// keys are emitted with default style and no comments.
func (c *Config) MarshalYAML() ([]byte, error) {
	if c.root == nil || c.root.IsLeaf() && c.root.Value == nil && len(c.root.ChildrenKeys()) == 0 {
		return []byte{}, nil
	}

	yamlNode, err := nodeToYAML(c.root)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer

	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(yamlIndent)

	encErr := enc.Encode(yamlNode)
	if encErr != nil {
		return nil, fmt.Errorf("yaml encode: %w", encErr)
	}

	closeErr := enc.Close()
	if closeErr != nil {
		return nil, fmt.Errorf("yaml encode close: %w", closeErr)
	}

	return buf.Bytes(), nil
}

// yamlAnnotation extracts a YAMLAnnotation from a tree.Node, or returns
// the zero value when no YAML annotation is attached.
func yamlAnnotation(node *tree.Node) YAMLAnnotation {
	if node == nil {
		return YAMLAnnotation{Key: nil, Val: nil}
	}

	annotation, ok := node.Annotation().(YAMLAnnotation)
	if !ok {
		return YAMLAnnotation{Key: nil, Val: nil}
	}

	return annotation
}

// nodeToYAML converts a tree.Node into a *yaml.Node.
// When the tree node was unchanged since parsing from YAML, the original
// scalar style and surrounding comments are reused.
func nodeToYAML(node *tree.Node) (*yaml.Node, error) {
	switch {
	case node == nil:
		return newScalarNode(yamlNullTag, ""), nil
	case node.IsArray():
		return arrayNodeToYAML(node)
	case node.IsLeaf():
		return scalarNodeToYAML(node)
	default:
		return mappingNodeToYAML(node)
	}
}

// scalarNodeToYAML emits a leaf as a scalar yaml.Node, reusing style/comments
// from the original annotation when the value has not been modified.
func scalarNodeToYAML(node *tree.Node) (*yaml.Node, error) {
	annotation := yamlAnnotation(node)
	mutated := node.Source == meta.ModifiedSourceName

	if annotation.Val != nil && !mutated {
		clone := cloneScalarYAMLNode(annotation.Val)
		forcePlainStringQuoting(clone)

		return clone, nil
	}

	out := newScalarNode("", "")

	encErr := out.Encode(node.Value)
	if encErr != nil {
		return nil, fmt.Errorf("encode scalar: %w", encErr)
	}

	// Encode emits canonical YAML for ordinary scalars but produces an empty
	// Value for Inf and NaN — the fallback below ensures we always emit
	// the canonical YAML form for those special floats.
	if out.Kind == yaml.ScalarNode && out.Value == "" {
		fillSpecialFloat(out, node.Value)
	}

	// Carry over comments from the annotation even when the value is mutated.
	if annotation.Val != nil {
		out.HeadComment = annotation.Val.HeadComment
		out.LineComment = annotation.Val.LineComment
		out.FootComment = annotation.Val.FootComment
	}

	return out, nil
}

// forcePlainStringQuoting upgrades a plain (unquoted) string scalar to the
// quoted style that go.yaml.in/yaml/v3 would itself choose when encoding the
// same value from scratch.
//
// We need this only on the style-preserving path: when a source document
// contained an unquoted token that is a string under the YAML 1.2 core schema
// but a boolean or null under YAML 1.1 (off, on, yes, no, y, n, ~, ...), the
// parser hands us a plain !!str scalar. Re-emitting it plain would let a YAML
// 1.1 reader — notably Tarantool's libyaml-based config loader — interpret the
// value as a bool/null instead of the string the rest of the pipeline already
// treats it as. Quoting it makes the string interpretation explicit without
// changing its value. yaml/v3's own encoder already quotes such values, so the
// freshly-encoded (mutated) path is unaffected.
func forcePlainStringQuoting(node *yaml.Node) {
	if node.Kind != yaml.ScalarNode || node.Style != 0 {
		return
	}

	if node.Tag != "" && node.Tag != yamlStringTag {
		return
	}

	var probe yaml.Node
	if probe.Encode(node.Value) != nil {
		return
	}

	if probe.Style != 0 {
		node.Style = probe.Style
	}
}

// fillSpecialFloat fills out.Value/Tag for Inf and NaN values, which yaml.Node.Encode
// leaves with an empty Value.
func fillSpecialFloat(out *yaml.Node, value any) {
	floatVal, ok := value.(float64)
	if !ok {
		return
	}

	switch {
	case math.IsNaN(floatVal):
		out.Value = ".nan"
		out.Tag = yamlFloatTag
	case math.IsInf(floatVal, 1):
		out.Value = ".inf"
		out.Tag = yamlFloatTag
	case math.IsInf(floatVal, -1):
		out.Value = "-.inf"
		out.Tag = yamlFloatTag
	}
}

// mappingNodeToYAML emits a mapping yaml.Node, walking children in tree order.
func mappingNodeToYAML(node *tree.Node) (*yaml.Node, error) {
	out := newCollectionNode(yaml.MappingNode)

	if annotation := yamlAnnotation(node); annotation.Val != nil {
		out.Style = annotation.Val.Style
		out.HeadComment = annotation.Val.HeadComment
		out.LineComment = annotation.Val.LineComment
		out.FootComment = annotation.Val.FootComment
	}

	for _, key := range node.ChildrenKeys() {
		child := node.Child(key)
		if child == nil {
			continue
		}

		keyNode := keyYAMLNode(key, child)

		valNode, err := nodeToYAML(child)
		if err != nil {
			return nil, err
		}

		out.Content = append(out.Content, keyNode, valNode)
	}

	return out, nil
}

// arrayNodeToYAML emits a sequence yaml.Node, walking children in index order.
func arrayNodeToYAML(node *tree.Node) (*yaml.Node, error) {
	out := newCollectionNode(yaml.SequenceNode)

	if annotation := yamlAnnotation(node); annotation.Val != nil {
		out.Style = annotation.Val.Style
		out.HeadComment = annotation.Val.HeadComment
		out.LineComment = annotation.Val.LineComment
		out.FootComment = annotation.Val.FootComment
	}

	for _, key := range orderedArrayKeys(node) {
		child := node.Child(key)
		if child == nil {
			continue
		}

		valNode, err := nodeToYAML(child)
		if err != nil {
			return nil, err
		}

		out.Content = append(out.Content, valNode)
	}

	return out, nil
}

// orderedArrayKeys returns the keys of an array node in numeric order, then
// any non-numeric keys in their existing order. Non-numeric keys are unusual
// for arrays but we accept them to be defensive.
func orderedArrayKeys(node *tree.Node) []string {
	keys := node.ChildrenKeys()

	indexed := make([]string, 0, len(keys))
	other := make([]string, 0)

	for _, key := range keys {
		_, atoiErr := strconv.Atoi(key)
		if atoiErr == nil {
			indexed = append(indexed, key)
		} else {
			other = append(other, key)
		}
	}

	return append(indexed, other...)
}

// keyYAMLNode returns a yaml.Node to use as the key in a mapping.
// When the child carries a YAML annotation with the original key node, that
// node's style and comments are reused.
func keyYAMLNode(key string, child *tree.Node) *yaml.Node {
	if annotation := yamlAnnotation(child); annotation.Key != nil {
		return cloneScalarYAMLNode(annotation.Key)
	}

	return newScalarNode(yamlStringTag, key)
}

// cloneScalarYAMLNode returns a copy of the given scalar yaml.Node with
// Content/Anchor/Alias stripped — those are not meaningful on the cloned
// scalar value or key.
func cloneScalarYAMLNode(src *yaml.Node) *yaml.Node {
	clone := *src

	clone.Content = nil
	clone.Anchor = ""
	clone.Alias = nil

	return &clone
}

// newScalarNode constructs a fully-zeroed scalar yaml.Node with the given
// tag and value. Centralising construction satisfies exhaustruct without
// scattering nolint directives.
func newScalarNode(tag, value string) *yaml.Node {
	return &yaml.Node{
		Kind:        yaml.ScalarNode,
		Style:       0,
		Tag:         tag,
		Value:       value,
		Anchor:      "",
		Alias:       nil,
		Content:     nil,
		HeadComment: "",
		LineComment: "",
		FootComment: "",
		Line:        0,
		Column:      0,
	}
}

// newCollectionNode constructs a fully-zeroed yaml.Node of MappingNode or
// SequenceNode kind.
func newCollectionNode(kind yaml.Kind) *yaml.Node {
	return &yaml.Node{
		Kind:        kind,
		Style:       0,
		Tag:         "",
		Value:       "",
		Anchor:      "",
		Alias:       nil,
		Content:     nil,
		HeadComment: "",
		LineComment: "",
		FootComment: "",
		Line:        0,
		Column:      0,
	}
}
