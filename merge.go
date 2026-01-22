package config

import (
	"context"
	"errors"
	"fmt"

	"github.com/tarantool/go-config/path"
	"github.com/tarantool/go-config/tree"
)

// MergeCollector reads all values from a collector and merges them into the tree
// using the default merging logic.
// The collector's priority is determined by the caller (higher priority collectors
// should be merged later). This function handles primitive replacement, slice
// replacement, map recursive merging, and key order preservation based on the
// collector's KeepOrder flag.
func MergeCollector(root *tree.Node, col Collector) error {
	return MergeCollectorWithMerger(root, col, Default)
}

// MergeCollectorWithMerger reads all values from a collector and merges them into the tree
// using the provided merger. Returns a CollectorError if any errors occur during processing.
// Multiple errors are accumulated and returned together.
func MergeCollectorWithMerger(root *tree.Node, col Collector, merger Merger) error {
	ctx := merger.CreateContext(col)
	valueCh := col.Read(context.Background())

	var errs []error

	for val := range valueCh {
		meta := val.Meta()

		var raw any

		err := val.Get(&raw)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to get raw value for key %s: %w", meta.Key.String(), err))
			continue
		}

		err = merger.MergeValue(ctx, root, meta.Key, raw)
		if err != nil {
			errs = append(errs, fmt.Errorf("merge value at %s: %w", meta.Key.String(), err))
			continue
		}
	}

	err := ctx.ApplyOrdering(root)
	if err != nil {
		errs = append(errs, fmt.Errorf("apply ordering: %w", err))
	}

	if len(errs) > 0 {
		return NewCollectorError(col.Name(), errors.Join(errs...))
	}

	return nil
}

// mergeValue merges a single value into the tree at the specified path.
func mergeValue(root *tree.Node, keyPath path.KeyPath, value any, col Collector) {
	if len(keyPath) == 0 {
		// Merge at root.
		mergeNodeValue(root, value, col)
		return
	}

	// Navigate or create nodes along the path.
	node := root

	for i, segment := range keyPath {
		isLast := i == len(keyPath)-1

		child := node.Child(segment)
		if child == nil {
			// If node currently has a leaf value, clear it because we're adding a child.
			if node.Value != nil {
				node.Value = nil
			}

			child = tree.New()
			node.SetChild(segment, child)
		}

		if isLast {
			// This is the target node for the value.
			mergeNodeValue(child, value, col)
		} else {
			node = child
		}
	}
}

// mergeNodeValue merges a value into an existing node, handling type-specific merging logic.
func mergeNodeValue(node *tree.Node, value any, col Collector) {
	// Determine the merging behavior based on the type of the new value and existing node.
	// If the node currently holds a map (has children) and the new value is also a map,
	// we need to merge recursively. Otherwise, we replace the node's value entirely.
	//
	// Slices are always replaced completely (no element‑wise merging).
	// Primitives are replaced.
	m, newIsMap := value.(map[string]any)
	currentIsMap := !node.IsLeaf()

	if newIsMap {
		if currentIsMap {
			// Recursive map merging.
			mergeMapIntoNode(node, m, col)
		} else {
			// Convert leaf to map: clear leaf value, then merge map into children.
			node.ClearChildren()

			node.Value = nil
			mergeMapIntoNode(node, m, col)
		}
	} else {
		// Replacement (primitive, slice, or map overwriting non‑map).
		// Clear children and reset order flag.
		node.ClearChildren()

		node.Value = value
	}

	// Update node metadata.
	node.Source = col.Name()
	node.Revision = string(col.Revision())
}

// mergeMapIntoNode merges a map into a node's children recursively.
func mergeMapIntoNode(node *tree.Node, m map[string]any, col Collector) {
	// If node has a leaf value, clear it because we're merging a map into it.
	if node.Value != nil {
		node.Value = nil
	}

	for key, val := range m {
		child := node.Child(key)
		if child == nil {
			child = tree.New()
			node.SetChild(key, child)
		}
		// Recurse for this child.
		mergeNodeValue(child, val, col)
	}
}
