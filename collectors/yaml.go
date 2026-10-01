package collectors

import (
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"github.com/tarantool/go-config/v2"
	"github.com/tarantool/go-config/v2/tree"
	"go.yaml.in/yaml/v3"
)

const yamlBoolTag = "!!bool"

// YamlFormat implements Format interface.
type YamlFormat struct {
	name                string
	keepOrder           bool
	emptyAsString       bool
	tarantoolFormatting bool
	data                []byte
	reader              io.Reader
}

// YamlOption configures a YamlFormat created by NewYamlFormat.
type YamlOption func(*YamlFormat)

// EmptyAsString makes the YAML format read a scalar with no content, such as
// the value of `key:`, as the empty string instead of null. YAML resolves such
// a scalar to null, but Tarantool's YAML decoder reads it as "", so a config
// meant for Tarantool has to be read the same way to be validated the way
// Tarantool validates it. An explicit null (`~`, `null`) stays null.
// This behavior is also enabled by [WithTarantoolParserFormatting].
func EmptyAsString() YamlOption {
	return func(y *YamlFormat) {
		y.emptyAsString = true
	}
}

// WithTarantoolParserFormatting makes the YAML format parse scalars using
// Tarantool's rules: leading zeros do not imply octal, and underscores are not
// digit separators. Numbers containing underscores stay strings. Explicit
// 0x, 0o and 0b prefixes retain their bases. Only lowercase plain yes and no
// resolve to booleans; quoted and explicitly string-tagged values stay strings.
// Empty scalars (`key:`) are read as ""; explicit null (`~`, `null`) stays null.
// Parsed strings keep their type during schema validation and typed decoding.
func WithTarantoolParserFormatting() YamlOption {
	return func(y *YamlFormat) {
		y.tarantoolFormatting = true
		y.emptyAsString = true
	}
}

// NewYamlFormat return new YamlFormat object.
func NewYamlFormat(opts ...YamlOption) Format {
	format := YamlFormat{
		name:                "yaml",
		keepOrder:           true,
		emptyAsString:       false,
		tarantoolFormatting: false,
		data:                nil,
		reader:              nil,
	}

	for _, opt := range opts {
		if opt != nil {
			opt(&format)
		}
	}

	return format
}

// Name implements the Format interface.
func (y YamlFormat) Name() string {
	return y.name
}

// KeepOrder implements the Format interface.
func (y YamlFormat) KeepOrder() bool {
	return y.keepOrder
}

// From implements the Format interface.
func (y YamlFormat) From(reader io.Reader) Format {
	y.reader = reader
	return y
}

// Parse implements the Format interface.
func (y YamlFormat) Parse() (*tree.Node, error) {
	var err error

	var node yaml.Node

	if y.reader != nil {
		dataFromReader, err := io.ReadAll(y.reader)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrReader, err)
		}

		y.data = append(y.data, dataFromReader...)
	}

	if y.data == nil {
		return nil, ErrNoData
	}

	err = yaml.Unmarshal(y.data, &node)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnmarshall, err)
	}

	root := tree.New()

	flattener := yamlFlattener{
		ranges:              newYamlRanges(y.data, &node),
		expanding:           make(map[*yaml.Node]bool),
		emptyAsString:       y.emptyAsString,
		tarantoolFormatting: y.tarantoolFormatting,
		visits:              0,
		aliasVisits:         0,
	}

	err = flattener.flatten(root, nil, &node, config.NewKeyPath(""))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnmarshall, err)
	}

	return root, nil
}

// yamlFlattener copies a YAML document into a tree, expanding aliases under
// the limits yaml.v3 applies when it decodes one.
type yamlFlattener struct {
	ranges *yamlRanges
	// expanding holds the aliases being expanded, to catch an anchor that
	// contains itself.
	expanding map[*yaml.Node]bool
	// emptyAsString reads a scalar with no content as "" (see EmptyAsString).
	emptyAsString       bool
	tarantoolFormatting bool
	visits              int
	aliasVisits         int
}

// yaml.v3's limits on alias expansion: once a document has more than
// aliasMinVisits nodes, of which more than aliasMinAliasVisits come from
// aliases, their share may not exceed aliasRatioMax up to aliasRatioLow nodes,
// falling to aliasRatioMin at aliasRatioHigh.
const (
	aliasMinVisits      = 1000
	aliasMinAliasVisits = 100
	aliasRatioLow       = 400_000
	aliasRatioHigh      = 4_000_000
	aliasRatioMax       = 0.99
	aliasRatioMin       = 0.10
)

func allowedAliasRatio(visits int) float64 {
	switch {
	case visits <= aliasRatioLow:
		return aliasRatioMax
	case visits >= aliasRatioHigh:
		return aliasRatioMin
	default:
		share := float64(visits-aliasRatioLow) / float64(aliasRatioHigh-aliasRatioLow)

		return aliasRatioMax - (aliasRatioMax-aliasRatioMin)*share
	}
}

func (f *yamlFlattener) flatten(node *tree.Node, key *yaml.Node, yamlNode *yaml.Node, prefix config.KeyPath) error {
	f.visits++
	if len(f.expanding) > 0 {
		f.aliasVisits++
	}

	if f.aliasVisits > aliasMinAliasVisits && f.visits > aliasMinVisits &&
		float64(f.aliasVisits)/float64(f.visits) > allowedAliasRatio(f.visits) {
		return ErrYamlExcessiveAliasing
	}

	switch yamlNode.Kind {
	case yaml.DocumentNode:
		for _, child := range yamlNode.Content {
			err := f.flatten(node, nil, child, prefix)
			if err != nil {
				return err
			}
		}
	case yaml.MappingNode:
		if len(yamlNode.Content) == 0 {
			node.Set(prefix, map[string]any{})

			if target := node.Get(prefix); target != nil {
				target.Range = f.ranges.get(yamlNode)

				yamlNodeCopy := *yamlNode
				target.SetAnnotation(config.YAMLAnnotation{Key: key, Val: &yamlNodeCopy})
			}

			return nil
		}

		// Ensure the mapping node exists so we can attach the annotation.
		if len(prefix) > 0 && node.Get(prefix) == nil {
			node.Set(prefix, nil)

			node.Get(prefix).Value = nil
		}

		if target := node.Get(prefix); target != nil {
			target.Range = f.ranges.get(yamlNode)

			yamlNodeCopy := *yamlNode
			target.SetAnnotation(config.YAMLAnnotation{Key: key, Val: &yamlNodeCopy})
		}

		for i := 0; i < len(yamlNode.Content); i += 2 {
			k := yamlNode.Content[i]
			value := yamlNode.Content[i+1]

			err := f.flatten(node, k, value, prefix.Append(k.Value))
			if err != nil {
				return err
			}
		}
	case yaml.SequenceNode:
		// Ensure the array node exists even for empty sequences.
		if node.Get(prefix) == nil {
			node.Set(prefix, nil)
		}

		arrNode := node.Get(prefix)
		arrNode.MarkArray()

		arrNode.Range = f.ranges.get(yamlNode)

		yamlNodeCopy := *yamlNode
		arrNode.SetAnnotation(config.YAMLAnnotation{Key: key, Val: &yamlNodeCopy})

		for i, item := range yamlNode.Content {
			err := f.flatten(node, nil, item, prefix.Append(strconv.Itoa(i)))
			if err != nil {
				return err
			}
		}
	case yaml.AliasNode:
		// Field `Value` contains name of the anchor.
		// Field `Alias` contains pointer to the anchor.
		if f.expanding[yamlNode] {
			return fmt.Errorf("%w: %q", ErrYamlAliasCycle, yamlNode.Value)
		}

		f.expanding[yamlNode] = true

		err := f.flatten(node, key, yamlNode.Alias, prefix)

		delete(f.expanding, yamlNode)

		if err != nil {
			return err
		}

		if target := node.Get(prefix); target != nil {
			target.Range = f.ranges.get(yamlNode)
		}
	case yaml.ScalarNode:
		value := resolveYamlScalar(*yamlNode, f.tarantoolFormatting)
		if value == nil && yamlNode.Value == "" && f.emptyAsString {
			value = ""
		}

		node.Set(prefix, value)

		target := node.Get(prefix)
		if target != nil {
			target.Range = f.ranges.get(yamlNode)

			// Tarantool-resolved strings must not be coerced back into numbers or booleans.
			_, isString := value.(string)
			target.SetTypeFixed(yamlScalarTypeFixed(yamlNode) || f.tarantoolFormatting && isString)

			yamlNodeCopy := *yamlNode
			if boolean, ok := value.(bool); ok && yamlNodeCopy.ShortTag() == "!!str" {
				// Plain yes/no were resolved to bool, but yaml.v3 tagged them as strings.
				// Use a boolean annotation so marshaling preserves their resolved type.
				yamlNodeCopy.Tag = yamlBoolTag
				yamlNodeCopy.Value = strconv.FormatBool(boolean)
			}

			target.SetAnnotation(config.YAMLAnnotation{Key: key, Val: &yamlNodeCopy})
		}
	default:
	}

	return nil
}

// yamlScalarTypeFixed reports whether the author of the document fixed the
// type of a scalar. YAML 1.2 resolves a quoted or block scalar to a string
// whatever its content, and an explicit tag names the type outright; only a
// plain scalar without a tag has its type inferred, and only such a scalar
// may be read as another type later on.
func yamlScalarTypeFixed(yamlNode *yaml.Node) bool {
	const fixed = yaml.TaggedStyle | yaml.DoubleQuotedStyle | yaml.SingleQuotedStyle |
		yaml.LiteralStyle | yaml.FoldedStyle

	return yamlNode.Style&fixed != 0
}

// resolveYamlScalar converts a YAML scalar node's string value into a typed Go value
// based on the YAML tag. Only core YAML tags (!!null, !!bool, !!int, !!float, !!str)
// are converted; unknown tags default to string.
func resolveYamlScalar(yamlNode yaml.Node, tarantoolFormatting bool) any {
	tag := yamlNode.ShortTag()

	if tarantoolFormatting && tag == "!!str" && !yamlScalarTypeFixed(&yamlNode) {
		// yaml.v3 tags Tarantool's lowercase yes/no booleans as strings.
		switch yamlNode.Value {
		case "yes", "no":
			tag = yamlBoolTag
		}
	}

	// strconv also accepts underscores in prefixed integers and floats, so
	// reject separators before parsing either numeric type in Tarantool mode.
	if tarantoolFormatting && (tag == "!!int" || tag == "!!float") && strings.ContainsRune(yamlNode.Value, '_') {
		return yamlNode.Value
	}

	switch tag {
	case "!!null":
		return nil
	case yamlBoolTag:
		return resolveYamlBool(yamlNode.Value, tarantoolFormatting)
	case "!!int":
		return resolveYamlInt(yamlNode.Value, tarantoolFormatting)
	case "!!float":
		// yaml.v3 infers a float for integers such as 08 or 09. Retry them
		// with decimal rules, preserving an explicit !!float tag.
		if tarantoolFormatting && yamlNode.Style&yaml.TaggedStyle == 0 {
			switch value := resolveYamlInt(yamlNode.Value, tarantoolFormatting); value.(type) {
			case int64, uint64:
				return value
			}
		}

		return resolveYamlFloat(yamlNode.Value, tarantoolFormatting)
	default:
		return yamlNode.Value
	}
}

// resolveYamlBool parses YAML boolean values.
func resolveYamlBool(value string, tarantoolFormatting bool) any {
	if tarantoolFormatting {
		switch value {
		case "true", "yes":
			return true
		case "false", "no":
			return false
		default:
			return value
		}
	}

	lower := strings.ToLower(value)

	switch lower {
	case "true":
		return true
	case "false":
		return false
	default:
	}

	return value
}

// resolveYamlInt parses YAML integers, using decimal for leading zeros in
// Tarantool mode and preserving explicit hex, octal and binary prefixes.
func resolveYamlInt(value string, tarantoolFormatting bool) any {
	plain := value
	base := 0

	if tarantoolFormatting {
		base = 10

		digits := strings.ToLower(strings.TrimLeft(plain, "+-"))

		if strings.HasPrefix(digits, "0x") || strings.HasPrefix(digits, "0o") || strings.HasPrefix(digits, "0b") {
			base = 0
		}
	} else {
		plain = strings.ReplaceAll(value, "_", "")
	}

	i, err := strconv.ParseInt(plain, base, 64)
	if err == nil {
		return i
	}

	// Try as unsigned for very large values.
	u, err := strconv.ParseUint(plain, base, 64)
	if err == nil {
		return u
	}

	return value
}

// resolveYamlFloat parses YAML float values including special values (.inf, .nan).
func resolveYamlFloat(value string, tarantoolFormatting bool) any {
	lower := strings.ToLower(value)

	switch lower {
	case ".inf", "+.inf":
		return math.Inf(1)
	case "-.inf":
		return math.Inf(-1)
	case ".nan":
		return math.NaN()
	}

	plain := value
	if !tarantoolFormatting {
		plain = strings.ReplaceAll(value, "_", "")
	}

	f, err := strconv.ParseFloat(plain, 64)
	if err == nil {
		return f
	}

	return value
}
