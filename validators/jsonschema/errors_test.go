package jsonschema //nolint:testpackage

import (
	"testing"

	"github.com/kaptinlin/jsonschema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tarantool/go-config/keypath"
	"github.com/tarantool/go-config/validator"
)

const (
	errorCodeRequired = "required"
	errorCodeType     = "type"
)

func TestValidationErrors_All(t *testing.T) {
	t.Parallel()

	ve := &validationErrors{
		errors: []validator.ValidationError{
			{
				Path:    keypath.KeyPath{"field"},
				Range:   validator.NewEmptyRange(),
				Code:    errorCodeRequired,
				Message: "missing field",
			},
		},
	}

	all := ve.All()
	assert.Len(t, all, 1)
	assert.Equal(t, errorCodeRequired, all[0].Code)
	assert.Equal(t, keypath.KeyPath{"field"}, all[0].Path)
}

func TestValidationErrors_Error_Empty(t *testing.T) {
	t.Parallel()

	ve := &validationErrors{errors: nil}
	assert.Equal(t, "no validation errors", ve.Error())
}

func TestValidationErrors_Error_Single(t *testing.T) {
	t.Parallel()

	ve := &validationErrors{
		errors: []validator.ValidationError{
			{
				Path:    keypath.KeyPath{"field"},
				Range:   validator.NewEmptyRange(),
				Code:    errorCodeRequired,
				Message: "missing field",
			},
		},
	}

	assert.Equal(t, "field [required] missing field", ve.Error())
}

func TestValidationErrors_Error_Multiple(t *testing.T) {
	t.Parallel()

	ve := &validationErrors{
		errors: []validator.ValidationError{
			{
				Path:    keypath.KeyPath{"field1"},
				Range:   validator.NewEmptyRange(),
				Code:    errorCodeRequired,
				Message: "missing field1",
			},
			{
				Path:    keypath.KeyPath{"field2"},
				Range:   validator.NewEmptyRange(),
				Code:    errorCodeType,
				Message: "invalid type",
			},
		},
	}

	assert.Equal(t, "field1 [required] missing field1; field2 [type] invalid type", ve.Error())
}

func TestFormatErrorMessage_Required(t *testing.T) {
	t.Parallel()

	err := &jsonschema.EvaluationError{
		Keyword: "required",
		Message: "name",
		Code:    "",
		Params:  nil,
	}

	msg := formatErrorMessage(err)
	assert.Equal(t, "missing required property: name", msg)
}

func TestFormatErrorMessage_Type(t *testing.T) {
	t.Parallel()

	err := &jsonschema.EvaluationError{
		Keyword: "type",
		Message: "expected string, got number",
		Code:    "",
		Params:  nil,
	}

	msg := formatErrorMessage(err)
	assert.Equal(t, "invalid type: expected string, got number", msg)
}

func TestFormatErrorMessage_Minimum(t *testing.T) {
	t.Parallel()

	err := &jsonschema.EvaluationError{
		Keyword: "minimum",
		Message: "value must be >= 0",
		Code:    "",
		Params:  nil,
	}

	msg := formatErrorMessage(err)
	assert.Equal(t, "value out of range: value must be >= 0", msg)
}

func TestFormatErrorMessage_Maximum(t *testing.T) {
	t.Parallel()

	err := &jsonschema.EvaluationError{
		Keyword: "maximum",
		Message: "value must be <= 100",
		Code:    "",
		Params:  nil,
	}

	msg := formatErrorMessage(err)
	assert.Equal(t, "value out of range: value must be <= 100", msg)
}

func TestFormatErrorMessage_ExclusiveMinimum(t *testing.T) {
	t.Parallel()

	err := &jsonschema.EvaluationError{
		Keyword: "exclusiveMinimum",
		Message: "value must be > 0",
		Code:    "",
		Params:  nil,
	}

	msg := formatErrorMessage(err)
	assert.Equal(t, "value out of range: value must be > 0", msg)
}

func TestFormatErrorMessage_ExclusiveMaximum(t *testing.T) {
	t.Parallel()

	err := &jsonschema.EvaluationError{
		Keyword: "exclusiveMaximum",
		Message: "value must be < 100",
		Code:    "",
		Params:  nil,
	}

	msg := formatErrorMessage(err)
	assert.Equal(t, "value out of range: value must be < 100", msg)
}

func TestFormatErrorMessage_Pattern(t *testing.T) {
	t.Parallel()

	err := &jsonschema.EvaluationError{
		Keyword: "pattern",
		Message: "must match ^[a-z]+$",
		Code:    "",
		Params:  nil,
	}

	msg := formatErrorMessage(err)
	assert.Equal(t, "value does not match pattern: must match ^[a-z]+$", msg)
}

func TestFormatErrorMessage_Enum(t *testing.T) {
	t.Parallel()

	err := &jsonschema.EvaluationError{
		Keyword: "enum",
		Message: "must be one of [red, green, blue]",
		Code:    "",
		Params:  nil,
	}

	msg := formatErrorMessage(err)
	assert.Equal(t, "value not in allowed set: must be one of [red, green, blue]", msg)
}

func TestFormatErrorMessage_Default(t *testing.T) {
	t.Parallel()

	err := &jsonschema.EvaluationError{
		Keyword: "unknown",
		Message: "some error",
		Code:    "",
		Params:  nil,
	}

	msg := formatErrorMessage(err)
	assert.Equal(t, "some error", msg)
}

func TestAddErrors_EmptyResult(t *testing.T) {
	t.Parallel()

	ve := &validationErrors{errors: nil}
	result := &jsonschema.EvaluationResult{
		Valid:            false,
		EvaluationPath:   "",
		SchemaLocation:   "",
		InstanceLocation: "",
		Annotations:      nil,
		Errors:           map[string]*jsonschema.EvaluationError{},
		Details:          nil,
	}

	ve.addErrors(result, "")
	assert.Empty(t, ve.errors)
}

func TestAddErrors_SingleError(t *testing.T) {
	t.Parallel()

	ve := &validationErrors{errors: nil}
	result := &jsonschema.EvaluationResult{
		InstanceLocation: "/field",
		Errors: map[string]*jsonschema.EvaluationError{
			"required": {
				Keyword: "required",
				Message: "name",
				Code:    "",
				Params:  nil,
			},
		},
	}

	ve.addErrors(result, "")
	require.Len(t, ve.errors, 1)
	assert.Equal(t, keypath.KeyPath{"field"}, ve.errors[0].Path)
	assert.Equal(t, errorCodeRequired, ve.errors[0].Code)
	assert.Equal(t, "missing required property: name", ve.errors[0].Message)
}

func TestAddErrors_MultipleErrors(t *testing.T) {
	t.Parallel()

	ve := &validationErrors{errors: nil}
	result := &jsonschema.EvaluationResult{
		InstanceLocation: "/person",
		Errors: map[string]*jsonschema.EvaluationError{
			"required": {
				Keyword: "required",
				Message: "name",
				Code:    "",
				Params:  nil,
			},
			"type": {
				Keyword: "type",
				Message: "expected string",
				Code:    "",
				Params:  nil,
			},
		},
	}

	ve.addErrors(result, "")
	require.Len(t, ve.errors, 2)

	// Order is not guaranteed because map iteration.
	foundRequired := false
	foundType := false

	for _, err := range ve.errors {
		if err.Code == errorCodeRequired {
			foundRequired = true

			assert.Equal(t, keypath.KeyPath{"person"}, err.Path)
			assert.Equal(t, "missing required property: name", err.Message)
		}

		if err.Code == errorCodeType {
			foundType = true

			assert.Equal(t, keypath.KeyPath{"person"}, err.Path)
			assert.Equal(t, "invalid type: expected string", err.Message)
		}
	}

	assert.True(t, foundRequired)
	assert.True(t, foundType)
}

func TestAddErrors_WithBasePath(t *testing.T) {
	t.Parallel()

	ve := &validationErrors{errors: nil}
	result := &jsonschema.EvaluationResult{
		InstanceLocation: "/field",
		Errors: map[string]*jsonschema.EvaluationError{
			"required": {
				Keyword: "required",
				Message: "name",
				Code:    "",
				Params:  nil,
			},
		},
	}

	ve.addErrors(result, "/parent")
	require.Len(t, ve.errors, 1)
	assert.Equal(t, keypath.KeyPath{"parent", "field"}, ve.errors[0].Path)
}

func TestCollectErrorsFromPath_NoDetails(t *testing.T) {
	t.Parallel()

	ve := &validationErrors{errors: nil}
	result := &jsonschema.EvaluationResult{
		InstanceLocation: "/field",
		Errors: map[string]*jsonschema.EvaluationError{
			"required": {
				Keyword: "required",
				Message: "name",
				Code:    "",
				Params:  nil,
			},
		},
		Details: []*jsonschema.EvaluationResult{},
	}

	ve.collectErrorsFromPath(result, "")
	require.Len(t, ve.errors, 1)
	assert.Equal(t, keypath.KeyPath{"field"}, ve.errors[0].Path)
	assert.Equal(t, errorCodeRequired, ve.errors[0].Code)
}

func TestCollectErrorsFromPath_WithDetails(t *testing.T) {
	t.Parallel()

	ve := &validationErrors{errors: nil}
	detail := &jsonschema.EvaluationResult{
		InstanceLocation: "/subfield",
		Errors: map[string]*jsonschema.EvaluationError{
			"type": {
				Keyword: "type",
				Message: "expected integer",
				Code:    "",
				Params:  nil,
			},
		},
	}

	result := &jsonschema.EvaluationResult{
		InstanceLocation: "/field",
		Errors: map[string]*jsonschema.EvaluationError{
			"required": {
				Keyword: "required",
				Message: "name",
				Code:    "",
				Params:  nil,
			},
		},
		Details: []*jsonschema.EvaluationResult{detail},
	}

	ve.collectErrorsFromPath(result, "")
	require.Len(t, ve.errors, 2)

	// Find errors.
	var foundRequired, foundType bool

	for _, err := range ve.errors {
		if err.Code == errorCodeRequired && len(err.Path) == 1 && err.Path[0] == "field" {
			foundRequired = true
		}

		if err.Code == errorCodeType && len(err.Path) == 2 &&
			err.Path[0] == "field" && err.Path[1] == "subfield" {
			foundType = true
		}
	}

	assert.True(t, foundRequired)
	assert.True(t, foundType)
}

func TestCollectErrorsFromPath_DeepNesting(t *testing.T) {
	t.Parallel()

	ve := &validationErrors{errors: nil}
	deepDetail := &jsonschema.EvaluationResult{
		InstanceLocation: "/deep",
		Errors: map[string]*jsonschema.EvaluationError{
			"minimum": {
				Keyword: "minimum",
				Message: "value >= 0",
				Code:    "",
				Params:  nil,
			},
		},
	}

	detail := &jsonschema.EvaluationResult{
		InstanceLocation: "/sub",
		Errors:           map[string]*jsonschema.EvaluationError{},
		Details:          []*jsonschema.EvaluationResult{deepDetail},
	}

	result := &jsonschema.EvaluationResult{
		InstanceLocation: "/top",
		Errors: map[string]*jsonschema.EvaluationError{
			"required": {
				Keyword: "required",
				Message: "name",
				Code:    "",
				Params:  nil,
			},
		},
		Details: []*jsonschema.EvaluationResult{detail},
	}

	ve.collectErrorsFromPath(result, "")
	require.Len(t, ve.errors, 2)

	var foundRequired, foundMinimum bool

	for _, err := range ve.errors {
		if err.Code == errorCodeRequired && len(err.Path) == 1 && err.Path[0] == "top" {
			foundRequired = true
		}

		if err.Code == "minimum" && len(err.Path) == 3 &&
			err.Path[0] == "top" && err.Path[1] == "sub" && err.Path[2] == "deep" {
			foundMinimum = true
		}
	}

	assert.True(t, foundRequired)
	assert.True(t, foundMinimum)
}
