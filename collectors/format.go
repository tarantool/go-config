package collectors

import (
	"fmt"
	"io"
	"strconv"

	"github.com/tarantool/go-config"
	"github.com/tarantool/go-config/tree"
	"go.yaml.in/yaml/v3"
)

// Format represents way to convert yaml into the tree.Node.
type Format interface {
	Name() string
	KeepOrder() bool
	With(data []byte) *FormatCollector // ?
	From(r io.Reader) *FormatCollector // ?
	Parse(data []byte) (*tree.Node, error)
}

// FormatCollector implements Format interface.
type FormatCollector struct {
	name      string
	keepOrder bool
	data      []byte
	reader    io.Reader
}

// Name implements the Format interface.
func (f *FormatCollector) Name() string {
	return f.name
}

// KeepOrder implements the Format interface.
func (f *FormatCollector) KeepOrder() bool {
	return f.keepOrder
}

// With implements the Format interface.
func (f *FormatCollector) With(data []byte) *FormatCollector {
	f.data = data
	return f
}

// From implements the Format interface.
func (f *FormatCollector) From(r io.Reader) *FormatCollector {
	f.reader = r
	return f
}

// Parse implements the Format interface.
func (f *FormatCollector) Parse(data []byte) (*tree.Node, error) {
	var err error

	var node yaml.Node

	err = yaml.Unmarshal(data, &node)
	if err != nil {
		return nil, fmt.Errorf("%w", errUnmarshallError)
	}

	root := tree.New()

	convertYamlIntoTree(root, node, config.NewKeyPath(""))

	return root, nil
}

func convertYamlIntoTree(node *tree.Node, yamlNode yaml.Node,
	prefix config.KeyPath,
) {
	switch yamlNode.Kind {
	case yaml.DocumentNode:
		for _, child := range yamlNode.Content {
			convertYamlIntoTree(node, *child, prefix)
		}
	case yaml.MappingNode:
		for i := 0; i < len(yamlNode.Content); i += 2 {
			key := yamlNode.Content[i]
			value := yamlNode.Content[i+1]

			// key.HeadComment - over the line.
			// key.LineComment - in the line.

			newPrefix := prefix.Append(key.Value)

			convertYamlIntoTree(node, *value, newPrefix)
		}
	case yaml.SequenceNode:
		for i, item := range yamlNode.Content {
			newPrefix := prefix.Append(strconv.Itoa(i))

			convertYamlIntoTree(node, *item, newPrefix)
		}
	case yaml.AliasNode:
		// Field `Value` contains name of the anchor.
		// Field `Alias` contains pointer to the anchor.
		convertYamlIntoTree(node, *yamlNode.Alias, prefix)
	case yaml.ScalarNode:
		node.Set(prefix, yamlNode.Value)
	default:
	}
}
