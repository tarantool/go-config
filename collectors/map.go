// Package collectors provides standard implementations of the Collector interface.
package collectors

import (
	"context"
	"slices"

	"github.com/tarantool/go-config"
	"github.com/tarantool/go-config/tree"
)

// Map reads configuration data from a map.
type Map struct {
	data       map[string]any
	name       string
	sourceType config.SourceType
	revision   config.RevisionType
	keepOrder  bool
}

// NewMap creates a Map with the given data.
// The source type defaults to config.UnknownSource.
func NewMap(data map[string]any) *Map {
	return &Map{
		data:       data,
		name:       "map",
		sourceType: config.UnknownSource,
		revision:   "",
		keepOrder:  false,
	}
}

// WithName sets a custom name for the collector.
func (mc *Map) WithName(name string) *Map {
	mc.name = name
	return mc
}

// WithSourceType sets the source type for the collector.
func (mc *Map) WithSourceType(source config.SourceType) *Map {
	mc.sourceType = source
	return mc
}

// WithRevision sets the revision for the collector.
func (mc *Map) WithRevision(rev config.RevisionType) *Map {
	mc.revision = rev
	return mc
}

// WithKeepOrder sets whether the collector preserves key order.
func (mc *Map) WithKeepOrder(keep bool) *Map {
	mc.keepOrder = keep
	return mc
}

// Read implements the Collector interface.
func (mc *Map) Read(ctx context.Context) <-chan config.Value {
	valueCh := make(chan config.Value)

	go func() {
		defer close(valueCh)
		// Build a tree from the map.
		root := tree.New()
		flattenMapIntoTree(root, config.NewKeyPath(""), mc.data, mc.keepOrder)
		// Walk the tree and send leaf values.
		// For simplicity, we traverse recursively.
		walkTree(ctx, root, config.NewKeyPath(""), valueCh)
	}()

	return valueCh
}

// Name implements the Collector interface.
func (mc *Map) Name() string {
	return mc.name
}

// Source implements the Collector interface.
func (mc *Map) Source() config.SourceType {
	return mc.sourceType
}

// Revision implements the Collector interface.
func (mc *Map) Revision() config.RevisionType {
	return mc.revision
}

// KeepOrder implements the Collector interface.
func (mc *Map) KeepOrder() bool {
	return mc.keepOrder
}

// flattenMapIntoTree recursively inserts map values into a tree node.
// If keepOrder is true, keys are inserted in sorted order to provide deterministic ordering.
func flattenMapIntoTree(node *tree.Node, prefix config.KeyPath, m map[string]any, keepOrder bool) {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	if keepOrder {
		slices.Sort(keys)
	}

	for _, k := range keys {
		value := m[k]
		path := prefix.Append(k)

		switch val := value.(type) {
		case map[string]any:
			flattenMapIntoTree(node, path, val, keepOrder)
		default:
			// Leaf value.
			node.Set(path, value)
		}
	}
}

// walkTree sends leaf values into the channel.
func walkTree(ctx context.Context, node *tree.Node, prefix config.KeyPath, valueCh chan<- config.Value) {
	if node.IsLeaf() {
		select {
		case <-ctx.Done():
			return
		case valueCh <- tree.NewValue(node, prefix):
		}

		return
	}

	for _, key := range node.ChildrenKeys() {
		child := node.Child(key)
		if child == nil {
			continue
		}

		walkTree(ctx, child, prefix.Append(key), valueCh)
	}
}
