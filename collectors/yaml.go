package collectors

import (
	"fmt"
	"io"
	"strconv"

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
		for i, item := range yamlNode.Content {
			newPrefix := prefix.Append(strconv.Itoa(i))

			flattenYamlIntoTree(node, *item, newPrefix)
		}
	case yaml.AliasNode:
		// Field `Value` contains name of the anchor.
		// Field `Alias` contains pointer to the anchor.
		flattenYamlIntoTree(node, *yamlNode.Alias, prefix)
	case yaml.ScalarNode:
		node.Set(prefix, yamlNode.Value)
	default:
	}
}
