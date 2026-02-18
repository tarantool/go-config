package config_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tarantool/go-config"
	"github.com/tarantool/go-config/collectors"
	"github.com/tarantool/go-config/tree"
	"github.com/tarantool/go-config/validator"
)

// alwaysFailingValidator that always fails.
type alwaysFailingValidator struct{}

func (m *alwaysFailingValidator) Validate(_ *tree.Node) []validator.ValidationError {
	return []validator.ValidationError{
		{
			Path:    config.NewKeyPath("test"),
			Range:   validator.NewEmptyRange(),
			Code:    "test",
			Message: "always fails",
		},
	}
}

func (m *alwaysFailingValidator) SchemaType() string {
	return "always-failing"
}

// maxKeysValidator fails validation when number of keys exceeds limit.
type maxKeysValidator struct {
	maxKeys int
}

func (m *maxKeysValidator) Validate(root *tree.Node) []validator.ValidationError {
	if root == nil {
		return nil
	}

	if len(root.Children()) > m.maxKeys {
		return []validator.ValidationError{
			{
				Path:    config.NewKeyPath(""),
				Range:   validator.NewEmptyRange(),
				Code:    "too-many-keys",
				Message: "too many keys",
			},
		}
	}

	return nil
}

func (m *maxKeysValidator) SchemaType() string {
	return "max-keys"
}

// valueChangeValidator fails when key "key" value is not "original".
type valueChangeValidator struct{}

func (v *valueChangeValidator) Validate(root *tree.Node) []validator.ValidationError {
	if root == nil {
		return nil
	}

	child := root.Child("key")
	if child == nil {
		return nil
	}

	if child.Value != "original" {
		return []validator.ValidationError{
			{
				Path:    config.NewKeyPath("key"),
				Range:   validator.NewEmptyRange(),
				Code:    "value-changed",
				Message: "value must be 'original'",
			},
		}
	}

	return nil
}

func (v *valueChangeValidator) SchemaType() string {
	return "value-change"
}

func TestConfig_Get_MissingKey(t *testing.T) {
	t.Parallel()

	data := map[string]any{
		"existing": 42,
	}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.AddCollector(col)

	cfg, errs := builder.Build()
	require.Empty(t, errs)

	var dest int

	_, err := cfg.Get(config.NewKeyPath("missing"), &dest)
	require.Error(t, err)
	assert.ErrorContains(t, err, "key not found")
}

func TestConfig_Get_TypeMismatchError(t *testing.T) {
	t.Parallel()

	data := map[string]any{
		"port": []any{1, 2},
	}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.AddCollector(col)

	cfg, errs := builder.Build()
	require.Empty(t, errs)

	var dest int

	_, err := cfg.Get(config.NewKeyPath("port"), &dest)
	require.Error(t, err)
	t.Logf("conversion error: %v", err)
}

func TestConfig_Stat_NilRoot(t *testing.T) {
	t.Parallel()

	validator := &alwaysFailingValidator{}
	data := map[string]any{"x": 1}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.WithValidator(validator)
	builder = builder.AddCollector(col)

	cfg, errs := builder.Build()
	require.NotNil(t, errs)

	meta, ok := cfg.Stat(config.NewKeyPath("any"))
	assert.False(t, ok)
	assert.Empty(t, meta.Source.Name)
	assert.Equal(t, config.UnknownSource, meta.Source.Type)
}

func TestConfig_Walk_NilRoot(t *testing.T) {
	t.Parallel()

	validator := &alwaysFailingValidator{}
	data := map[string]any{"x": 1}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.WithValidator(validator)
	builder = builder.AddCollector(col)

	cfg, errs := builder.Build()
	require.NotNil(t, errs)

	ctx := context.Background()
	_, err := cfg.Walk(ctx, config.NewKeyPath(""), -1)
	require.Error(t, err)
	assert.ErrorContains(t, err, "path not found")
}

func TestMutableConfig_Update_ValidationError(t *testing.T) {
	t.Parallel()

	// Create mutable config with validator that fails when key "key" value changes.
	v := &valueChangeValidator{}
	data := map[string]any{"key": "original"}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.WithValidator(v)
	builder = builder.AddCollector(col)

	mcfg, errs := builder.BuildMutable()
	require.Empty(t, errs)

	// Create other config with updated value for existing key.
	otherData := map[string]any{"key": "newvalue"}
	col2 := collectors.NewMap(otherData)
	builder2 := config.NewBuilder()

	builder2 = builder2.AddCollector(col2)

	otherCfg, errs2 := builder2.Build()
	require.Empty(t, errs2)

	// Update should fail due to validation error.
	err := mcfg.Update(&otherCfg)
	require.Error(t, err)
	assert.ErrorContains(t, err, "value must be 'original'")
}

func TestConfig_Slice_NilRootNonEmptyPath(t *testing.T) {
	t.Parallel()

	validator := &alwaysFailingValidator{}
	data := map[string]any{"x": 1}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.WithValidator(validator)
	builder = builder.AddCollector(col)

	cfg, errs := builder.Build()
	require.NotNil(t, errs)

	// Slice with non-empty path on nil root should error.
	_, err := cfg.Slice(config.NewKeyPath("some/path"))
	require.Error(t, err)
	assert.ErrorContains(t, err, "path not found")
}

func TestMutableConfig_Merge_WalkError(t *testing.T) {
	t.Parallel()

	// Create valid mutable config.
	validData := map[string]any{"key": "value"}
	col := collectors.NewMap(validData)
	builder := config.NewBuilder()

	builder = builder.AddCollector(col)

	mcfg, errs := builder.BuildMutable()
	require.Empty(t, errs)

	// Create invalid config with nil root (validation failure).
	validator := &alwaysFailingValidator{}
	invalidData := map[string]any{"x": 1}
	col2 := collectors.NewMap(invalidData)
	builder2 := config.NewBuilder()

	builder2 = builder2.WithValidator(validator)
	builder2 = builder2.AddCollector(col2)

	invalidCfg, errs2 := builder2.Build()
	require.NotNil(t, errs2) // Validation errors, root is nil.

	// Merge should fail because Walk on invalid config returns error.
	err := mcfg.Merge(&invalidCfg)
	require.Error(t, err)
	assert.ErrorContains(t, err, "path not found")
}

func TestMutableConfig_Merge_ValidationError(t *testing.T) {
	t.Parallel()

	// Use maxKeysValidator that fails when more than 1 key present.
	validator := &maxKeysValidator{maxKeys: 1}
	data := map[string]any{"key": "value"}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.WithValidator(validator)
	builder = builder.AddCollector(col)

	mcfg, errs := builder.BuildMutable()
	require.Empty(t, errs)

	// Sanity check: validator is attached and works for Set.
	err := mcfg.Set(config.NewKeyPath("secondkey"), "value")
	require.Error(t, err)
	require.ErrorContains(t, err, "too many keys")

	// Build other config with newkey.
	otherData := map[string]any{"newkey": "newvalue"}
	col2 := collectors.NewMap(otherData)
	builder2 := config.NewBuilder()

	builder2 = builder2.AddCollector(col2)

	otherCfg, errs2 := builder2.Build()
	require.Empty(t, errs2)

	// Verify other config contains newkey.
	_, ok := otherCfg.Lookup(config.NewKeyPath("newkey"))
	assert.True(t, ok)

	// Merge should fail due to validation error.
	err = mcfg.Merge(&otherCfg)
	require.Error(t, err)
	assert.ErrorContains(t, err, "too many keys")
}

func TestMutableConfig_Update_WalkError(t *testing.T) {
	t.Parallel()

	// Create valid mutable config.
	validData := map[string]any{"key": "value"}
	col := collectors.NewMap(validData)
	builder := config.NewBuilder()

	builder = builder.AddCollector(col)

	mcfg, errs := builder.BuildMutable()
	require.Empty(t, errs)

	// Create invalid config with nil root (validation failure).
	validator := &alwaysFailingValidator{}
	invalidData := map[string]any{"x": 1}
	col2 := collectors.NewMap(invalidData)
	builder2 := config.NewBuilder()

	builder2 = builder2.WithValidator(validator)
	builder2 = builder2.AddCollector(col2)

	invalidCfg, errs2 := builder2.Build()
	require.NotNil(t, errs2) // Validation errors, root is nil.

	// Update should fail because Walk on invalid config returns error.
	err := mcfg.Update(&invalidCfg)
	require.Error(t, err)
	assert.ErrorContains(t, err, "path not found")
}

func TestConfig_Walk_NonExistentPath(t *testing.T) {
	t.Parallel()

	data := map[string]any{"key": "value"}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.AddCollector(col)

	cfg, errs := builder.Build()
	require.Empty(t, errs)

	ctx := context.Background()
	_, err := cfg.Walk(ctx, config.NewKeyPath("nonexistent"), -1)
	require.Error(t, err)
	assert.ErrorContains(t, err, "path not found")
}

// TestConfig_Walk_CtxCancelled removed; see TestWalkNodes_CtxCancelled in inheritance_internal_test.go.

func TestConfig_Slice_NilRootEmptyPath(t *testing.T) {
	t.Parallel()

	// Create config with nil root (validation failure).
	validator := &alwaysFailingValidator{}
	data := map[string]any{"x": 1}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.WithValidator(validator)
	builder = builder.AddCollector(col)

	cfg, errs := builder.Build()
	require.NotNil(t, errs) // Validation errors, root is nil.

	// Slice with empty path should return config with nil root (no error).
	sliced, err := cfg.Slice(config.NewKeyPath(""))
	require.NoError(t, err)
	// Attempt to get a value from sliced config (should return not found).
	_, ok := sliced.Lookup(config.NewKeyPath("any"))
	assert.False(t, ok)
}

func TestConfig_Effective_NilRoot(t *testing.T) {
	t.Parallel()

	// Config with nil root.
	validator := &alwaysFailingValidator{}
	data := map[string]any{"x": 1}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.WithValidator(validator)
	builder = builder.AddCollector(col)

	cfg, errs := builder.Build()
	require.NotNil(t, errs) // Validation errors, root is nil.

	_, err := cfg.Effective(config.NewKeyPath("any"))
	require.Error(t, err)
	assert.ErrorContains(t, err, "path not found")
}
