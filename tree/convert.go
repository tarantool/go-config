package tree

// ToAny converts a Node tree to a generic Go value suitable for JSON validation.
// Returns:
//   - primitive value for leaf nodes
//   - []any for array nodes (nodes marked with MarkArray)
//   - map[string]any for object nodes
func ToAny(node *Node) any {
	switch {
	case node == nil:
		return nil
	case node.Value == nil && node.children == nil:
		if node.isArray {
			return []any{}
		}

		return map[string]any{}
	case node.IsLeaf():
		return node.Value
	}

	// Array node - build slice from children in order.
	if node.isArray {
		keys := node.ChildrenKeys()

		result := make([]any, 0, len(keys))
		for _, key := range keys {
			child := node.Child(key)
			if child != nil {
				result = append(result, ToAny(child))
			}
		}

		return result
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
