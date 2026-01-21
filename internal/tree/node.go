package tree

import (
	"slices"

	"github.com/tarantool/go-config/internal/omap"
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
