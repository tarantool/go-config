package jsonschema

import (
	"strings"

	"github.com/kaptinlin/jsonschema"

	"github.com/tarantool/go-config/keypath"
	"github.com/tarantool/go-config/tree"
	"github.com/tarantool/go-config/validator"
)

// validationErrors collects validation errors during JSON Schema validation.
//
//nolint:errname
type validationErrors struct {
	root   *tree.Node
	errors []validator.ValidationError
}

// All returns all collected validation errors.
func (ve *validationErrors) All() []validator.ValidationError {
	return ve.errors
}

// Error returns a concatenated string representation of all validation errors.
func (ve *validationErrors) Error() string {
	if len(ve.errors) == 0 {
		return "no validation errors"
	}

	var builder strings.Builder

	for i, err := range ve.errors {
		if i > 0 {
			builder.WriteString("; ")
		}

		builder.WriteString(err.Error())
	}

	return builder.String()
}

// rangeForPath returns the Range for the given path from the root node.
func (ve *validationErrors) rangeForPath(p keypath.KeyPath) validator.Range {
	if ve.root == nil {
		return validator.NewEmptyRange()
	}

	node := ve.root.Get(p)
	if node == nil {
		return validator.NewEmptyRange()
	}

	return validator.RangeFromTree(node.Range)
}

// collectErrorsFromPath recursively collects validation errors from the evaluation result and its details.
func (ve *validationErrors) collectErrorsFromPath(result *jsonschema.EvaluationResult, basePath string) {
	ve.addErrors(result, basePath)

	for _, detail := range result.Details {
		ve.collectErrorsFromPath(detail, basePath+result.InstanceLocation)
	}
}

// addErrors adds validation errors from the evaluation result at the given base path.
func (ve *validationErrors) addErrors(result *jsonschema.EvaluationResult, basePath string) {
	for keyword, err := range result.Errors {
		path := basePath + result.InstanceLocation
		keyPath := jsonPointerToKeyPath(path)

		ve.errors = append(ve.errors, validator.ValidationError{
			Path:    keyPath,
			Range:   ve.rangeForPath(keyPath),
			Code:    keyword,
			Message: formatErrorMessage(err),
		})
	}
}

// formatErrorMessage creates a user-friendly error message.
func formatErrorMessage(err *jsonschema.EvaluationError) string {
	// Format based on keyword type for better readability.
	switch err.Keyword {
	case "required":
		return "missing required property: " + err.Message
	case "type":
		return "invalid type: " + err.Message
	case "minimum", "maximum", "exclusiveMinimum", "exclusiveMaximum":
		return "value out of range: " + err.Message
	case "pattern":
		return "value does not match pattern: " + err.Message
	case "enum":
		return "value not in allowed set: " + err.Message
	default:
		return err.Message
	}
}
