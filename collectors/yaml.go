package collectors

import (
	"context"
	"io"
	"strconv"

	"github.com/tarantool/go-config"
	"github.com/tarantool/go-config/internal/tree"
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

// NewYamlCollector creates a Yaml collector with the given io.Reader.
// The source type defaults to config.UnknownSource.
func NewYamlCollector(reader io.Reader) *YamlCollector {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil
	}

	return &YamlCollector{
		name:       "yaml",
		sourceType: config.UnknownSource,
		revision:   "",
		keepOrder:  true,
		data:       data,
	}
}

// WithName sets a custom name for the collector.
func (yc *YamlCollector) WithName(name string) *YamlCollector {
	yc.name = name
	return yc
}

// WithSourceType sets the source type for the collector.
func (yc *YamlCollector) WithSourceType(source config.SourceType) *YamlCollector {
	yc.sourceType = source
	return yc
}

// WithRevision sets the revision for the collector.
func (yc *YamlCollector) WithRevision(rev config.RevisionType) *YamlCollector {
	yc.revision = rev
	return yc
}

// WithKeepOrder sets whether the collector preserves key order.
func (yc *YamlCollector) WithKeepOrder(keep bool) *YamlCollector {
	yc.keepOrder = keep
	return yc
}

// Name implements the Collector interface.
func (yc *YamlCollector) Name() string {
	return yc.name
}

// Source implements the Collector interface.
func (yc *YamlCollector) Source() config.SourceType {
	return yc.sourceType
}

// Revision implements the Collector interface.
func (yc *YamlCollector) Revision() config.RevisionType {
	return yc.revision
}

// KeepOrder implements the Collector interface.
func (yc *YamlCollector) KeepOrder() bool {
	return yc.keepOrder
}

// Read implements the Collector interface.
func (yc *YamlCollector) Read(ctx context.Context) <-chan config.Value {
	valueCh := make(chan config.Value)

	go func() {
		defer close(valueCh)

		var err error

		var node yaml.Node

		err = yaml.Unmarshal(yc.data, &node)
		if err != nil {
			return
		}

		// Build a tree.
		root := tree.New()

		flattenYamlIntoTree(root, node, config.NewKeyPath(""))

		// Walk the tree and send leaf values.
		// For simplicity, we traverse recursively.
		walkTree(ctx, root, config.NewKeyPath(""), valueCh)
	}()

	return valueCh
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
