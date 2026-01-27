package tree

import (
	"slices"

	"github.com/tarantool/go-config/omap"
	"github.com/tarantool/go-config/path"
)

// Value represents a configuration value.
type Value any

// Node represents a node in the configuration tree.
type Node struct {
	// Value holds the node's value if it's a leaf node.
	Value Value

	// Source indicates where this node's value came from (e.g., file, env, flag).
	Source string

	// Revision is a version identifier for the node (e.g., commit hash, timestamp).
	Revision string

	// orderSet indicates whether the order of children has been set by a higher-priority ordered collector.
	orderSet bool

	// children is an ordered map from child keys to their nodes.
	children *omap.OrderedMap[string, *Node]
}

// New creates a new empty node.
func New() *Node {
	return &Node{
		Value:    nil,
		Source:   "",
		Revision: "",

		children: nil,
		orderSet: false,
	}
}

// IsLeaf returns true if the node has no children.
func (n *Node) IsLeaf() bool {
	if n.children == nil {
		return true
	}

	return n.children.Len() == 0
}

// Children returns the child nodes in insertion order.
func (n *Node) Children() []*Node {
	if n.children == nil {
		return nil
	}

	return slices.Collect(n.children.Values())
}

// ChildrenKeys returns the keys of child nodes in insertion order.
func (n *Node) ChildrenKeys() []string {
	if n.children == nil {
		return nil
	}

	return slices.Collect(n.children.Keys())
}

// Child returns the child node for the given key, or nil if not found.
func (n *Node) Child(key string) *Node {
	if n.children == nil {
		return nil
	}

	child, ok := n.children.Get(key)
	if !ok {
		return nil
	}

	return child
}

// SetChild sets or replaces a child node under the given key.
// If a child with this key already exists, it is replaced and the insertion order is preserved.
// If the child is new, it is appended to the end of the order.
func (n *Node) SetChild(key string, child *Node) {
	if n.children == nil {
		n.children = omap.New[string, *Node]()
	}

	n.children.Set(key, child)
}

// DeleteChild removes a child node by key.
// It also removes the key from the insertion order.
func (n *Node) DeleteChild(key string) bool {
	if n.children == nil {
		return false
	}

	return n.children.Delete(key)
}

// Set sets the value at the given path, creating intermediate nodes as needed.
func (n *Node) Set(path path.KeyPath, value Value) {
	if len(path) == 0 {
		n.Value = value
		return
	}

	key := path[0]

	child := n.Child(key)
	if child == nil {
		child = New()
		n.SetChild(key, child)
	}

	child.Set(path[1:], value)
}

// Get returns the node at the given path, or nil if not found.
func (n *Node) Get(path path.KeyPath) *Node {
	if len(path) == 0 {
		return n
	}

	key := path[0]

	child := n.Child(key)
	if child == nil {
		return nil
	}

	return child.Get(path[1:])
}

// GetValue returns the value at the given path, or nil if not found or node is not a leaf.
func (n *Node) GetValue(path path.KeyPath) Value {
	node := n.Get(path)
	if node == nil || !node.IsLeaf() {
		return nil
	}

	return node.Value
}

// HasChild returns true if the node has a child with the given key.
func (n *Node) HasChild(key string) bool {
	return n.Child(key) != nil
}

// ClearChildren removes all child nodes and resets the orderSet flag.
func (n *Node) ClearChildren() {
	if n.children != nil {
		n.children.Clear()
	}

	n.orderSet = false
}

// OrderSet returns true if the order of children has been set by a higher-priority ordered collector.
func (n *Node) OrderSet() bool {
	return n.orderSet
}

// SetOrderSet sets the orderSet flag.
func (n *Node) SetOrderSet(v bool) {
	n.orderSet = v
}

// ReorderChildren reorders the node's children according to the provided keys.
// Only keys present in the keys slice are reordered; other keys keep their relative positions.
// Keys that are not present in the node's children are ignored.
// If the node has no children or keys is empty, nothing happens.
func (n *Node) ReorderChildren(keys []string) error {
	if n.children == nil || len(keys) == 0 {
		return nil
	}

	// Build a set of keys to reorder for quick lookup.
	reorderSet := make(map[string]bool, len(keys))
	for _, k := range keys {
		reorderSet[k] = true
	}

	// Collect existing keys in current order.
	existingKeys := n.ChildrenKeys()
	if len(existingKeys) == 0 {
		return nil
	}

	// Partition keys into those to reorder and those to keep in place.
	var (
		reorderList []string
		keepList    []string
	)

	for _, k := range existingKeys {
		if reorderSet[k] {
			reorderList = append(reorderList, k)
		} else {
			keepList = append(keepList, k)
		}
	}

	// Ensure all keys in the reorderList appear in the input keys in the same order as input.
	// We need to reorder reorderList according to the input order.
	// Create a map from key to its position in input keys.
	inputPos := make(map[string]int, len(keys))
	for i, k := range keys {
		inputPos[k] = i
	}

	// Sort reorderList by input position.
	slices.SortFunc(reorderList, func(a, b string) int {
		posA := inputPos[a]

		posB := inputPos[b]
		if posA < posB {
			return -1
		}

		if posA > posB {
			return 1
		}

		return 0
	})

	// Build new order: first the reordered keys (in input order), then the kept keys.
	newOrder := make([]string, 0, len(existingKeys))

	newOrder = append(newOrder, reorderList...)
	newOrder = append(newOrder, keepList...)

	// Build a new ordered map with the new order.
	newChildren := omap.NewWithCapacity[string, *Node](len(newOrder))
	for _, key := range newOrder {
		value, _ := n.children.Get(key)
		newChildren.Set(key, value)
	}

	n.children = newChildren

	return nil
}
