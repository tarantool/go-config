// Package omap provides a generic ordered map implementation that maintains
// insertion order of keys.
//
// [OrderedMap] is parameterized as OrderedMap[K comparable, V any] and is used
// internally by the configuration tree to preserve the order of configuration
// keys.
//
// # Key Features
//
//   - Maintains insertion order — iterating keys, values, or items always
//     returns them in the order they were first inserted.
//   - Standard map operations: [OrderedMap.Set], [OrderedMap.Get],
//     [OrderedMap.Has], [OrderedMap.Delete], [OrderedMap.Len],
//     [OrderedMap.Clear].
//   - Go 1.23 iterators: [OrderedMap.Keys] (iter.Seq[K]),
//     [OrderedMap.Values] (iter.Seq[V]), [OrderedMap.Items] (iter.Seq2[K, V]).
//   - [NewWithCapacity] for preallocated capacity when the expected size is
//     known in advance.
package omap
