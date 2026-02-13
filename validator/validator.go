// Package validator defines interfaces and types for configuration validation.
package validator

import (
	"github.com/tarantool/go-config/tree"
)

// Validator validates configuration against a schema.
type Validator interface {
	// Validate checks the tree and returns all errors found.
	// Returns nil if validation succeeds.
	Validate(root *tree.Node) []ValidationError

	// SchemaType returns the type identifier (e.g., "json-schema").
	SchemaType() string
}
