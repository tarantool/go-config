// Package keypath provides the [KeyPath] type and utilities for working with
// hierarchical configuration key paths.
//
// A [KeyPath] is a []string representing a hierarchical key. For example,
// "server/http/port" becomes []string{"server", "http", "port"}.
//
// # Constructors
//
//   - [NewKeyPath] — creates a KeyPath by splitting on "/".
//   - [NewKeyPathWithDelim] — creates a KeyPath with a custom delimiter.
//   - [NewKeyPathFromSegments] — creates a KeyPath from an existing slice.
//
// # Methods
//
// [KeyPath] provides methods for common path operations: [KeyPath.String],
// [KeyPath.MakeString], [KeyPath.Parent], [KeyPath.Last], [KeyPath.Append],
// [KeyPath.Equal], [KeyPath.HasPrefix], [KeyPath.TrimPrefix], [KeyPath.Len],
// and [KeyPath.IsEmpty].
package keypath
