package collectors

import (
	"context"
	"fmt"
	"io"
	"strconv"

	"github.com/tarantool/go-config"
	"github.com/tarantool/go-config/tree"
	"go.yaml.in/yaml/v3"
)

// YamlCollector represents common collector from io.Reader.
type YamlCollector struct {
	name       string
	sourceType config.SourceType
	revision   config.RevisionType
	keepOrder  bool
	data       []byte
}

// Name implements the Collector interface.
func (y *YamlCollector) Name() string {
	return y.name
}

// Source implements the Collector interface.
func (y *YamlCollector) Source() config.SourceType {
	return y.sourceType
}

// Revision implements the Collector interface.
func (y *YamlCollector) Revision() config.RevisionType {
	return y.revision
}

// KeepOrder implements the Collector interface.
func (y *YamlCollector) KeepOrder() bool {
	return y.keepOrder
}

// Read implements the Collector interface.
func (y *YamlCollector) Read(ctx context.Context) <-chan config.Value {
	channel := make(chan config.Value)

	go func() {
		defer close(channel)

		var err error

		var node yaml.Node

		err = yaml.Unmarshal(y.data, &node)
		if err != nil {
			return
		}

		// Build a tree.
		root := tree.New()

		flattenYamlIntoTree(root, node, config.NewKeyPath(""))

		// Walk the tree and send leaf values.
		// For simplicity, we traverse recursively.
		walkTree(ctx, root, config.NewKeyPath(""), channel)
	}()

	return channel
}

// YamlCollectorBuilder represent Builder object.
type YamlCollectorBuilder struct {
	reader     io.Reader
	name       string
	sourceType config.SourceType
	revision   config.RevisionType
	keepOrder  bool
	data       []byte
}

// NewYamlCollectorBuilder returns new YamlCollectorBuilder object.
func NewYamlCollectorBuilder(reader io.Reader) YamlCollectorBuilder {
	return YamlCollectorBuilder{
		reader:     reader,
		name:       "yaml",
		sourceType: config.UnknownSource,
		revision:   "",
		keepOrder:  false,
		data:       nil,
	}
}

// SetName sets a custom name for the collector.
func (y YamlCollectorBuilder) SetName(name string) YamlCollectorBuilder {
	y.name = name
	return y
}

// SetSourceType sets the source type for the collector.
func (y YamlCollectorBuilder) SetSourceType(source config.SourceType) YamlCollectorBuilder {
	y.sourceType = source
	return y
}

// SetRevision sets the revision for the collector.
func (y YamlCollectorBuilder) SetRevision(rev config.RevisionType) YamlCollectorBuilder {
	y.revision = rev
	return y
}

// SetKeepOrder sets whether the collector preserves key order.
func (y YamlCollectorBuilder) SetKeepOrder(keep bool) YamlCollectorBuilder {
	y.keepOrder = keep
	return y
}

// Build create YamlCollector.
func (y YamlCollectorBuilder) Build() (*YamlCollector, error) {
	data, err := io.ReadAll(y.reader)
	if err != nil {
		return nil, fmt.Errorf("%w", errReaderError)
	}

	return &YamlCollector{
		name:       y.name,
		sourceType: y.sourceType,
		revision:   y.revision,
		keepOrder:  y.keepOrder,
		data:       data,
	}, nil
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
