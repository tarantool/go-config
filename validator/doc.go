// Package validator defines interfaces and types for configuration validation.
//
// This package provides the contract layer for validation. Concrete
// implementations live in subpackages (e.g., validators/jsonschema).
//
// # Key Types
//
//   - [Validator] validates a [tree.Node] and returns a slice of
//     [ValidationError]. It also reports its [Validator.SchemaType].
//   - [ValidationError] describes a single validation failure with
//     Path ([keypath.KeyPath]), Range (source position), Code
//     (machine-readable), and Message (human-readable).
//   - [Position] represents a line/column in a source file (1-based,
//     0 if unknown).
//   - [Range] is a start/end [Position] pair used for source highlighting.
//
// # Helpers
//
//   - [NewEmptyRange] creates a placeholder Range when position
//     information is unavailable.
//   - [RangeFromTree] converts a [tree.Range] to a validator [Range].
package validator
