package config_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tarantool/go-config"
	"github.com/tarantool/go-config/collectors"
	"github.com/tarantool/go-config/tree"
	"github.com/tarantool/go-config/validator"
)

// mockValidator is a test validator that returns predefined errors.
type mockValidator struct {
	errors []validator.ValidationError
}

func (m *mockValidator) Validate(_ *tree.Node) []validator.ValidationError {
	return m.errors
}

func (m *mockValidator) SchemaType() string {
	return "mock"
}

func TestBuilder_WithValidator_Success(t *testing.T) {
	t.Parallel()

	// Mock validator that passes (no errors).
	mock := &mockValidator{
		errors: nil,
	}
	builder := config.NewBuilder()

	builder = builder.WithValidator(mock)

	// Add some data.
	col := collectors.NewMap(map[string]any{
		"port": 8080,
	})

	builder = builder.AddCollector(col)

	cfg, errs := builder.Build(t.Context())
	require.Empty(t, errs)
	require.NotNil(t, cfg)
}

func TestBuilder_WithValidator_Failure(t *testing.T) {
	t.Parallel()

	// Mock validator that returns a validation error.
	mock := &mockValidator{
		errors: []validator.ValidationError{
			{
				Path:    config.NewKeyPath("port"),
				Range:   validator.NewEmptyRange(),
				Code:    "range",
				Message: "port must be between 1024 and 65535",
			},
		},
	}
	builder := config.NewBuilder()

	builder = builder.WithValidator(mock)

	// Add data that will cause validation error.
	col := collectors.NewMap(map[string]any{
		"port": 80, // invalid according to mock.
	})

	builder = builder.AddCollector(col)

	cfg, errs := builder.Build(t.Context())
	require.NotNil(t, errs)
	assert.Len(t, errs, 1)
	// Root should be nil when validation fails; Lookup should return false.
	_, ok := cfg.Lookup(config.NewKeyPath("port"))
	assert.False(t, ok)
}

func TestBuilder_WithJSONSchema_Success(t *testing.T) {
	t.Parallel()

	schema := `{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"type": "object",
		"properties": {
			"port": {
				"type": "integer",
				"minimum": 1024,
				"maximum": 65535
			}
		},
		"additionalProperties": false
	}`

	builder := config.NewBuilder()
	builder, err := builder.WithJSONSchema(strings.NewReader(schema))
	require.NoError(t, err)

	col := collectors.NewMap(map[string]any{
		"port": 8080,
	})

	builder = builder.AddCollector(col)

	cfg, errs := builder.Build(t.Context())
	require.Empty(t, errs)
	require.NotNil(t, cfg)
}

func TestBuilder_WithJSONSchema_InvalidSchema(t *testing.T) {
	t.Parallel()

	schema := `{ invalid json }`
	builder := config.NewBuilder()
	_, err := builder.WithJSONSchema(strings.NewReader(schema))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create JSON schema validator")
}

func TestBuilder_MustWithJSONSchema_Panic(t *testing.T) {
	t.Parallel()

	schema := `{ invalid json }`
	builder := config.NewBuilder()

	defer func() {
		r := recover()
		require.NotNil(t, r)

		err, ok := r.(error)
		require.True(t, ok)
		assert.Contains(t, err.Error(), "failed to create JSON schema validator")
	}()

	builder.MustWithJSONSchema(strings.NewReader(schema))
}

func TestBuilder_ValidationFailure_NilRoot(t *testing.T) {
	t.Parallel()

	schema := `{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"type": "object",
		"properties": {
			"port": {
				"type": "integer",
				"minimum": 1024
			}
		},
		"additionalProperties": false
	}`

	builder := config.NewBuilder()

	builder = builder.MustWithJSONSchema(strings.NewReader(schema))

	// Add data that fails validation (port too low).
	col := collectors.NewMap(map[string]any{
		"port": 80,
	})

	builder = builder.AddCollector(col)

	cfg, errs := builder.Build(t.Context())
	require.NotNil(t, errs)
	assert.NotEmpty(t, errs)
	// Root should be nil when validation fails; Lookup should return false.
	_, ok := cfg.Lookup(config.NewKeyPath("port"))
	assert.False(t, ok)
}

func TestBuilder_BuildMutable_Validation(t *testing.T) {
	t.Parallel()

	schema := `{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"type": "object",
		"properties": {
			"port": {
				"type": "integer",
				"minimum": 1024
			}
		},
		"additionalProperties": false
	}`

	builder := config.NewBuilder()

	builder = builder.MustWithJSONSchema(strings.NewReader(schema))

	// Add valid data.
	col := collectors.NewMap(map[string]any{
		"port": 8080,
	})

	builder = builder.AddCollector(col)

	mcfg, errs := builder.BuildMutable(t.Context())
	require.Empty(t, errs)
	assert.NotNil(t, mcfg.Config)

	// Try to set invalid value via mutable config - should return error.
	err := mcfg.Set(config.NewKeyPath("port"), 80)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "properties")
}

func TestBuilder_WithoutValidation_SkipsBuildTimeValidation(t *testing.T) {
	t.Parallel()

	mock := &mockValidator{
		errors: []validator.ValidationError{
			{
				Path:    config.NewKeyPath("port"),
				Range:   validator.NewEmptyRange(),
				Code:    "range",
				Message: "port must be between 1024 and 65535",
			},
		},
	}

	builder := config.NewBuilder()

	builder = builder.WithValidator(mock)
	builder = builder.WithoutValidation()
	builder = builder.AddCollector(collectors.NewMap(map[string]any{
		"port": 80,
	}))

	cfg, errs := builder.Build(t.Context())
	require.Empty(t, errs, "Build-time validation must be skipped")

	var port int

	_, err := cfg.Get(config.NewKeyPath("port"), &port)
	require.NoError(t, err)
	assert.Equal(t, 80, port)
}

func TestBuilder_WithoutValidation_MutableConfigStillValidates(t *testing.T) {
	t.Parallel()

	schema := `{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"type": "object",
		"properties": {
			"port": { "type": "integer", "minimum": 1024 }
		}
	}`

	builder := config.NewBuilder()

	builder = builder.MustWithJSONSchema(strings.NewReader(schema))
	builder = builder.WithoutValidation()
	builder = builder.AddCollector(collectors.NewMap(map[string]any{
		"port": 80, // would normally fail validation.
	}))

	mcfg, errs := builder.BuildMutable(t.Context())
	require.Empty(t, errs, "Build-time validation must be skipped")

	// The validator survives — runtime mutations are still validated.
	err := mcfg.Set(config.NewKeyPath("port"), 100)
	require.Error(t, err, "MutableConfig must keep validating runtime mutations")
	assert.Contains(t, err.Error(), "properties")
}
