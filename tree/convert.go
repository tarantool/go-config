package tree

// ToAny converts a Node tree to a generic Go value suitable for JSON validation.
// Returns:
//   - primitive value for leaf nodes
//   - map[string]any for object nodes
//   - []any for array nodes (if Value is a slice)
func ToAny(node *Node) any {
	if node == nil {
		return nil
	}

	// Special case: node with nil value and no children (empty node) should be empty object.
	if node.Value == nil && node.children == nil {
		return map[string]any{}
	}

	// Leaf node - return value directly.
	if node.IsLeaf() {
		return node.Value
	}

	// Non-leaf node - build map from children.
	keys := node.ChildrenKeys()

	result := make(map[string]any, len(keys))
	for _, key := range keys {
		child := node.Child(key)
		if child != nil {
			result[key] = ToAny(child)
		}
	}

	return result
}
