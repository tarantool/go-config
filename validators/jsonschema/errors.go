package jsonschema

import (
	"slices"
	"strings"

	"github.com/kaptinlin/jsonschema"

	"github.com/tarantool/go-config/v2/keypath"
	"github.com/tarantool/go-config/v2/tree"
	"github.com/tarantool/go-config/v2/validator"
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
	if result.IsValid() {
		return
	}

	ve.addErrors(result, basePath)

	for _, detail := range result.Details {
		keyword, _, _ := strings.Cut(strings.TrimPrefix(detail.EvaluationPath, "/"), "/")
		switch keyword {
		case "if", "not":
			// A failed condition selects else; a failed negated schema makes
			// not succeed. Neither failure is an error in the input.
			continue
		case "anyOf", "oneOf":
			// Another keyword (e.g. enum) can fail at the same node even
			// though an alternative matched. For oneOf with multiple matches,
			// retain its own error, not failures of the remaining alternatives.
			if result.Errors[keyword] == nil {
				continue
			}

			matched := slices.ContainsFunc(result.Details, func(other *jsonschema.EvaluationResult) bool {
				return strings.HasPrefix(other.EvaluationPath, "/"+keyword+"/") && other.IsValid()
			})
			if matched {
				continue
			}
		}

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
	// err.Message is a raw template with {property}/{received}/{expected} placeholders;
	// only (*EvaluationError).Error() substitutes them from err.Params.
	body := err.Error()

	// Format based on keyword type for better readability.
	switch err.Keyword {
	case "required":
		return "missing required property: " + body
	case "type":
		return "invalid type: " + body
	case "minimum", "maximum", "exclusiveMinimum", "exclusiveMaximum":
		return "value out of range: " + body
	case "pattern":
		return "value does not match pattern: " + body
	case "enum":
		return "value not in allowed set: " + body
	default:
		return body
	}
}
