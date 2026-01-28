package jsonschema_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tarantool/go-config/keypath"
	"github.com/tarantool/go-config/tree"
	"github.com/tarantool/go-config/validators/jsonschema"
)

func TestNew_ValidSchema(t *testing.T) {
	t.Parallel()

	schema := `{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"type": "object",
		"properties": {
			"name": { "type": "string" }
		},
		"required": ["name"]
	}`
	validator, err := jsonschema.New([]byte(schema))
	require.NoError(t, err)
	require.NotNil(t, validator)
	assert.Equal(t, "json-schema", validator.SchemaType())
}

func TestNew_InvalidSchema(t *testing.T) {
	t.Parallel()

	schema := `{ "type": 123 }` // type must be string, not number.
	_, err := jsonschema.New([]byte(schema))
	require.Error(t, err)
	require.ErrorContains(t, err, "failed to compile JSON schema")
}

func TestNewFromReader(t *testing.T) {
	t.Parallel()

	schema := `{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"type": "object"
	}`
	reader := strings.NewReader(schema)

	validator, err := jsonschema.NewFromReader(reader)
	require.NoError(t, err)
	require.NotNil(t, validator)
	assert.Equal(t, "json-schema", validator.SchemaType())
}

func TestValidate_ValidData(t *testing.T) {
	t.Parallel()

	schema := `{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"type": "object",
		"properties": {
			"name": { "type": "string" },
			"age": { "type": "integer", "minimum": 0 }
		},
		"required": ["name"]
	}`
	validator, err := jsonschema.New([]byte(schema))
	require.NoError(t, err)

	// Build a tree matching the schema.
	root := tree.New()
	root.Set(keypath.NewKeyPath("name"), "Alice")
	root.Set(keypath.NewKeyPath("age"), 30)

	errs := validator.Validate(root)
	assert.Nil(t, errs)
}

func TestValidate_InvalidData(t *testing.T) {
	t.Parallel()

	schema := `{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"type": "object",
		"properties": {
			"name": { "type": "string" },
			"age": { "type": "integer", "minimum": 0 }
		},
		"required": ["name"]
	}`
	validator, err := jsonschema.New([]byte(schema))
	require.NoError(t, err)

	// Missing required field "name".
	root := tree.New()
	root.Set(keypath.NewKeyPath("age"), -5) // Also violates minimum.

	errs := validator.Validate(root)
	for i, validationErr := range errs {
		t.Logf("error %d: path=%v code=%s message=%s", i, validationErr.Path, validationErr.Code, validationErr.Message)
	}

	// Order of errors is not guaranteed, so check each error.
	foundRequired := false
	foundMinimum := false

	for _, validationErr := range errs {
		if validationErr.Code == "required" &&
			strings.Contains(validationErr.Message, "missing required property") &&
			len(validationErr.Path) == 0 {
			foundRequired = true
		}

		if validationErr.Code == "minimum" && strings.Contains(validationErr.Message, "value out of range") &&
			slices.Equal([]string{"age"}, validationErr.Path) {
			foundMinimum = true
		}
	}

	assert.True(t, foundRequired)
	assert.True(t, foundMinimum)
}

func TestValidate_ErrorPathMapping(t *testing.T) {
	t.Parallel()

	schema := `{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"type": "object",
		"properties": {
			"person": {
				"type": "object",
				"properties": {
					"name": { "type": "string" }
				},
				"required": ["name"]
			}
		}
	}`
	validator, err := jsonschema.New([]byte(schema))
	require.NoError(t, err)

	root := tree.New()
	// Create nested structure: person object missing name.
	person := tree.New()
	root.Set(keypath.NewKeyPath("person"), person)

	errs := validator.Validate(root)
	for i, validationErr := range errs {
		t.Logf("error %d: path=%v code=%s message=%s", i, validationErr.Path, validationErr.Code, validationErr.Message)
	}

	var found bool

	for _, validationErr := range errs {
		if slices.Equal([]string{"person"}, validationErr.Path) && validationErr.Code == "required" &&
			strings.Contains(validationErr.Message, "missing required property") {
			found = true
			break
		}
	}

	assert.True(t, found)
}

func TestSchemaType(t *testing.T) {
	t.Parallel()

	schema := `{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"type": "object"
	}`
	validator, err := jsonschema.New([]byte(schema))
	require.NoError(t, err)
	assert.Equal(t, "json-schema", validator.SchemaType())
}
