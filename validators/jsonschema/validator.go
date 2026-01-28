package jsonschema

import (
	"fmt"
	"io"

	"github.com/kaptinlin/jsonschema"

	"github.com/tarantool/go-config/tree"
	"github.com/tarantool/go-config/validator"
)

// ValidationErrors collects validation errors during JSON Schema validation.
type ValidationErrors struct {
	errors []validator.ValidationError
}

// All returns all collected validation errors.
func (ve *ValidationErrors) All() []validator.ValidationError {
	return ve.errors
}

// collectErrorsFromPath recursively collects validation errors from the evaluation result and its details.
func (ve *ValidationErrors) collectErrorsFromPath(result *jsonschema.EvaluationResult, basePath string) {
	ve.addErrors(result, basePath)

	for _, detail := range result.Details {
		ve.collectErrorsFromPath(detail, basePath+result.InstanceLocation)
	}
}

// addErrors adds validation errors from the evaluation result at the given base path.
func (ve *ValidationErrors) addErrors(result *jsonschema.EvaluationResult, basePath string) {
	for keyword, err := range result.Errors {
		path := basePath + result.InstanceLocation

		ve.errors = append(ve.errors, validator.ValidationError{
			Path:    jsonPointerToKeyPath(path),
			Range:   validator.NewTODORange(), // Placeholder until position tracking.
			Code:    keyword,
			Message: formatErrorMessage(err),
		})
	}
}

// Validator validates configuration against JSON Schema.
type Validator struct {
	schema   *jsonschema.Schema
	compiler *jsonschema.Compiler
}

// New creates a validator from schema bytes.
func New(schemaData []byte) (*Validator, error) {
	compiler := jsonschema.NewCompiler()

	schema, err := compiler.Compile(schemaData)
	if err != nil {
		return nil, fmt.Errorf("failed to compile JSON schema: %w", err)
	}

	return &Validator{schema: schema, compiler: compiler}, nil
}

// NewFromReader creates a validator from an io.Reader.
func NewFromReader(r io.Reader) (*Validator, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read schema: %w", err)
	}

	return New(data)
}

// Validate implements validator.Validator.
func (v *Validator) Validate(root *tree.Node) []validator.ValidationError {
	// Convert tree to map[string]any.
	data := tree.ToAny(root)

	// Validate using kaptinlin/jsonschema.
	result := v.schema.Validate(data)
	if result.IsValid() {
		return nil
	}

	// Convert errors.
	var ve ValidationErrors
	ve.collectErrorsFromPath(result, "")

	return ve.All()
}

// SchemaType returns JSONSchema string.
func (v *Validator) SchemaType() string {
	return "json-schema"
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
