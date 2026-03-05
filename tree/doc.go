// Package tree provides an in-memory configuration tree used as the central
// data structure by go-config.
//
// # Key Types
//
//   - [Node] — a tree node that can be a leaf (holding a scalar [Value]) or a
//     branch (with ordered children via [omap.OrderedMap]). Each node tracks
//     its Source, Revision, and [Range] (source position). Nodes can represent
//     YAML maps or arrays.
//   - Value (type alias for any) — the raw value stored in leaf nodes.
//   - [Range] / [Position] — source position tracking for nodes
//     (line/column).
//
// # Constructors and Converters
//
//   - [NewValue] — creates a [value.Value] from a tree node, implementing
//     type conversion (Get) and metadata (Meta).
//   - [ToAny] — converts a [Node] tree to a generic Go value
//     (map[string]any, []any, or primitive).
//
// # Node Operations
//
// The tree supports operations such as Child, SetChild, ChildrenKeys,
// DeepCopy, Merge, Walk, IsLeaf, MarkArray, and more.
package tree
