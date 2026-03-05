package collectors

import (
	"context"
	"slices"

	"github.com/tarantool/go-config"
	"github.com/tarantool/go-config/tree"
)

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
			node.Set(path, value)
		}
	}
}

// setSource recursively sets the Source field on a tree node and all its descendants.
func setSource(node *tree.Node, source string) {
	node.Source = source

	for _, key := range node.ChildrenKeys() {
		child := node.Child(key)
		if child != nil {
			setSource(child, source)
		}
	}
}

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
