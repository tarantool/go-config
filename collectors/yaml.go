package collectors

import (
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"github.com/tarantool/go-config"
	"github.com/tarantool/go-config/tree"
	"go.yaml.in/yaml/v3"
)

// YamlFormat implements Format interface.
type YamlFormat struct {
	name      string
	keepOrder bool
	data      []byte
	reader    io.Reader
}

// NewYamlFormat return new YamlFormat object.
func NewYamlFormat() Format {
	return YamlFormat{
		name:      "yaml",
		keepOrder: true,
		data:      nil,
		reader:    nil,
	}
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

	flattenYamlIntoTree(root, node, config.NewKeyPath(""))

	return root, nil
}

func flattenYamlIntoTree(node *tree.Node, yamlNode yaml.Node,
	prefix config.KeyPath,
) {
	switch yamlNode.Kind {
	case yaml.DocumentNode:
		for _, child := range yamlNode.Content {
			flattenYamlIntoTree(node, *child, prefix)
		}
	case yaml.MappingNode:
		for i := 0; i < len(yamlNode.Content); i += 2 {
			key := yamlNode.Content[i]
			value := yamlNode.Content[i+1]

			// key.HeadComment - over the line.
			// key.LineComment - in the line.

			newPrefix := prefix.Append(key.Value)

			flattenYamlIntoTree(node, *value, newPrefix)
		}
	case yaml.SequenceNode:
		// Ensure the array node exists even for empty sequences.
		if node.Get(prefix) == nil {
			node.Set(prefix, nil)
		}

		arrNode := node.Get(prefix)
		arrNode.MarkArray()

		for i, item := range yamlNode.Content {
			newPrefix := prefix.Append(strconv.Itoa(i))

			flattenYamlIntoTree(node, *item, newPrefix)
		}
	case yaml.AliasNode:
		// Field `Value` contains name of the anchor.
		// Field `Alias` contains pointer to the anchor.
		flattenYamlIntoTree(node, *yamlNode.Alias, prefix)
	case yaml.ScalarNode:
		node.Set(prefix, resolveYamlScalar(yamlNode))

		if n := node.Get(prefix); n != nil {
			n.Range = tree.Range{
				Start: tree.Position{Line: yamlNode.Line, Column: yamlNode.Column},
				End:   tree.Position{Line: yamlNode.Line, Column: yamlNode.Column},
			}
		}
	default:
	}
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
