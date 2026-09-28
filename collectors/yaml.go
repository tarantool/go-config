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

// YamlFormat implements Format interface.
type YamlFormat struct {
	name          string
	keepOrder     bool
	emptyAsString bool
	data          []byte
	reader        io.Reader
}

// YamlOption configures a YamlFormat created by NewYamlFormat.
type YamlOption func(*YamlFormat)

// EmptyAsString makes the YAML format read a scalar with no content, such as
// the value of `key:`, as the empty string instead of null. YAML resolves such
// a scalar to null, but Tarantool's YAML decoder reads it as "", so a config
// meant for Tarantool has to be read the same way to be validated the way
// Tarantool validates it. An explicit null (`~`, `null`) stays null.
func EmptyAsString() YamlOption {
	return func(y *YamlFormat) {
		y.emptyAsString = true
	}
}

// NewYamlFormat return new YamlFormat object.
func NewYamlFormat(opts ...YamlOption) Format {
	format := YamlFormat{
		name:          "yaml",
		keepOrder:     true,
		emptyAsString: false,
		data:          nil,
		reader:        nil,
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
		ranges:        newYamlRanges(y.data, &node),
		expanding:     make(map[*yaml.Node]bool),
		emptyAsString: y.emptyAsString,
		visits:        0,
		aliasVisits:   0,
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
	emptyAsString bool
	visits        int
	aliasVisits   int
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
		value := resolveYamlScalar(*yamlNode)
		if value == nil && yamlNode.Value == "" && f.emptyAsString {
			value = ""
		}

		node.Set(prefix, value)

		target := node.Get(prefix)
		if target != nil {
			target.Range = f.ranges.get(yamlNode)
			target.SetTypeFixed(yamlScalarTypeFixed(yamlNode))

			yamlNodeCopy := *yamlNode
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
func resolveYamlScalar(yamlNode yaml.Node) any {
	tag := yamlNode.ShortTag()

	switch tag {
	case "!!null":
		return nil
	case "!!bool":
		return resolveYamlBool(yamlNode.Value)
	case "!!int":
		return resolveYamlInt(yamlNode.Value)
	case "!!float":
		return resolveYamlFloat(yamlNode.Value)
	default:
		return yamlNode.Value
	}
}

// resolveYamlBool parses YAML boolean values.
func resolveYamlBool(value string) any {
	lower := strings.ToLower(value)

	switch lower {
	case "true":
		return true
	case "false":
		return false
	default:
		return value
	}
}

// resolveYamlInt parses YAML integer values (decimal, hex, octal, binary).
func resolveYamlInt(value string) any {
	plain := strings.ReplaceAll(value, "_", "")

	i, err := strconv.ParseInt(plain, 0, 64)
	if err == nil {
		return i
	}

	// Try as unsigned for very large values.
	u, err := strconv.ParseUint(plain, 0, 64)
	if err == nil {
		return u
	}

	// Handle 0o and 0b prefixes with signs.
	switch {
	case strings.HasPrefix(plain, "0o"):
		i, err = strconv.ParseInt(plain[2:], 8, 64)
		if err == nil {
			return i
		}
	case strings.HasPrefix(plain, "-0o"):
		i, err = strconv.ParseInt("-"+plain[3:], 8, 64)
		if err == nil {
			return i
		}
	case strings.HasPrefix(plain, "0b"):
		i, err = strconv.ParseInt(plain[2:], 2, 64)
		if err == nil {
			return i
		}
	case strings.HasPrefix(plain, "-0b"):
		i, err = strconv.ParseInt("-"+plain[3:], 2, 64)
		if err == nil {
			return i
		}
	}

	return value
}

// resolveYamlFloat parses YAML float values including special values (.inf, .nan).
func resolveYamlFloat(value string) any {
	lower := strings.ToLower(value)

	switch lower {
	case ".inf", "+.inf":
		return math.Inf(1)
	case "-.inf":
		return math.Inf(-1)
	case ".nan":
		return math.NaN()
	}

	plain := strings.ReplaceAll(value, "_", "")

	f, err := strconv.ParseFloat(plain, 64)
	if err == nil {
		return f
	}

	return value
}
