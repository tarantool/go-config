package validator

import (
	"fmt"

	"github.com/tarantool/go-config/keypath"
)

// ValidationError describes a single validation error.
type ValidationError struct {
	Path    keypath.KeyPath // Logical path to the field.
	Range   Range           // Physical range (populated when position tracking available).
	Code    string          // Machine-readable code (e.g., "type", "required", "minimum").
	Message string          // Human-readable description.
}

// Error returns a string representation of the validation error.
func (e *ValidationError) Error() string {
	if len(e.Path) == 0 {
		return fmt.Sprintf("[%s] %s", e.Code, e.Message)
	}

	return fmt.Sprintf("%s [%s] %s", e.Path, e.Code, e.Message)
}
