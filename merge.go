package config

import (
	"context"
	"errors"
	"fmt"

	"github.com/tarantool/go-config/keypath"
	"github.com/tarantool/go-config/tree"
)

// isNumericString reports whether str consists only of ASCII digits.
func isNumericString(str string) bool {
	if str == "" {
		return false
	}

	for i := range len(str) {
		if str[i] < '0' || str[i] > '9' {
			return false
		}
	}

	return true
}

// MergeCollector reads all values from a collector and merges them into the tree
// using the default merging logic.
// The collector's priority is determined by the caller (higher priority collectors
// should be merged later). This function handles primitive replacement, slice
// replacement, map recursive merging, and key order preservation based on the
// collector's KeepOrder flag.
func MergeCollector(ctx context.Context, root *tree.Node, col Collector) error {
	return MergeCollectorWithMerger(ctx, root, col, Default)
}

// MergeCollectorWithMerger reads all values from a collector and merges them into the tree
// using the provided merger. Returns a CollectorError if any errors occur during processing.
// Multiple errors are accumulated and returned together.
func MergeCollectorWithMerger(ctx context.Context, root *tree.Node, col Collector, merger Merger) error {
	mergeCtx := merger.CreateContext(col)
	valueCh := col.Read(ctx)

	var errs []error

	for val := range valueCh {
		meta := val.Meta()

		var raw any

		err := val.Get(&raw)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to get raw value for key %s: %w", meta.Key.String(), err))
			continue
		}

		err = merger.MergeValue(mergeCtx, root, meta.Key, raw)
		if err != nil {
			errs = append(errs, fmt.Errorf("merge value at %s: %w", meta.Key.String(), err))
			continue
		}

		// If the source value carries a format-specific annotation
		// (e.g., the YAML node it was parsed from), forward it onto
		// the destination tree node so that marshalers can reproduce
		// scalar style and comments.
		copyAnnotation(root, meta.Key, val)
	}

	err := mergeCtx.ApplyOrdering(root)
	if err != nil {
		errs = append(errs, fmt.Errorf("apply ordering: %w", err))
	}

	if len(errs) > 0 {
		return NewCollectorError(col.Name(), errors.Join(errs...))
	}

	return nil
}

// mergeValue merges a single value into the tree at the specified path.
func mergeValue(root *tree.Node, keyPath keypath.KeyPath, value any, col Collector) {
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

		if !isLast && isNumericString(keyPath[i+1]) {
			child.MarkArray()
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

// copyAnnotation forwards the format-specific annotation from a source Value
// onto the destination tree node at path, when the source exposes one.
func copyAnnotation(root *tree.Node, path keypath.KeyPath, src Value) {
	carrier, ok := src.(interface{ Annotation() any })
	if !ok {
		return
	}

	anno := carrier.Annotation()
	if anno == nil {
		return
	}

	dest := root.Get(path)
	if dest == nil {
		return
	}

	dest.SetAnnotation(anno)
}

// mergeTreeInto folds src into dst at the tree level.
// Map-into-map is recursive; any other src child replaces the dst child
// (carrying Source, Revision, Range, annotation, and isArray).
// When src.OrderSet() and !dst.OrderSet() the dst children are reordered
// to match src's key order and dst.OrderSet() is set true.
func mergeTreeInto(dst, src *tree.Node) {
	for _, key := range src.ChildrenKeys() {
		srcChild := src.Child(key)
		dstChild := dst.Child(key)

		// Both sides are non-leaf maps → recurse.
		if dstChild != nil && !dstChild.IsLeaf() && !srcChild.IsLeaf() {
			// Carry ordering before recursing so children see the right dst state.
			if srcChild.OrderSet() && !dstChild.OrderSet() {
				_ = dstChild.ReorderChildren(srcChild.ChildrenKeys())
				dstChild.SetOrderSet(true)
			}

			mergeTreeInto(dstChild, srcChild)

			continue
		}

		// Otherwise replace: clone the entire src child subtree into dst.
		dst.SetChild(key, cloneNode(srcChild))
	}

	// Apply ordering at this level.
	if src.OrderSet() && !dst.OrderSet() {
		_ = dst.ReorderChildren(src.ChildrenKeys())
		dst.SetOrderSet(true)
	}
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
