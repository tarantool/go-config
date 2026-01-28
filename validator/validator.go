// Package validator defines interfaces and types for configuration validation.
package validator

import (
	"fmt"

	"github.com/tarantool/go-config/path"
	"github.com/tarantool/go-config/tree"
)

// Position describes a position in source file (for LSP integration).
// Currently placeholder - will be populated when position tracking is implemented.
type Position struct {
	Line   int // Line number (1-based), 0 if unknown.
	Column int // Column number (1-based), 0 if unknown.
}

// Range describes a range in source file for highlighting.
type Range struct {
	Start Position
	End   Position
}

// NewTODORange creates a placeholder Range.
//
// To be replaced with real implementation when position tracking is available.
func NewTODORange() Range {
	return Range{
		Start: Position{Line: 0, Column: 0},
		End:   Position{Line: 0, Column: 0},
	}
}

// ValidationError describes a single validation error.
type ValidationError struct {
	Path    path.KeyPath // Logical path to the field.
	Range   Range        // Physical range (populated when position tracking available).
	Code    string       // Machine-readable code (e.g., "type", "required", "minimum").
	Message string       // Human-readable description.
}

// Error returns a string representation of the validation error.
func (e *ValidationError) Error() string {
	if len(e.Path) == 0 {
		return fmt.Sprintf("[%s] %s", e.Code, e.Message)
	}

	return fmt.Sprintf("%s [%s] %s", e.Path, e.Code, e.Message)
}

// Validator validates configuration against a schema.
type Validator interface {
	// Validate checks the tree and returns all errors found.
	// Returns nil if validation succeeds.
	Validate(root *tree.Node) []ValidationError

	// SchemaType returns the type identifier (e.g., "json-schema").
	SchemaType() string
}
