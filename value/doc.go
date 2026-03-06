// Package value defines the [Value] interface, the primary abstraction for
// reading configuration values in go-config.
//
// # Key Types
//
//   - [Value] — the central interface with two methods:
//   - Get(dest any) error — extracts and type-converts the internal value
//     into dest (passed by pointer), similar to [encoding/json.Unmarshal].
//   - Meta() [meta.Info] — returns metadata about the value: its key path,
//     source info, and revision.
//
// # Implementation
//
// This package only defines the contract. The concrete implementation lives
// in the [tree] package ([tree.NewValue]).
package value
