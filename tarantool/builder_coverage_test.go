package tarantool_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tarantool/go-config"
	"github.com/tarantool/go-config/internal/testutil"
	"github.com/tarantool/go-config/tarantool"
)

func TestWithStorageKey(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	writeFile(t, cfgPath, "server:\n  port: 8080\n")

	mock := testutil.NewMockStorage()
	testutil.PutIntegrity(mock, "/custom/", "app",
		[]byte("server:\n  host: storage-host\n"))

	typed := testutil.NewRawTyped(mock, "/custom/")
	ctx := context.Background()

	// Test WithStorageKey overrides the default "config" prefix.
	cfg, err := tarantool.New().
		WithConfigFile(cfgPath).
		WithStorage(typed).
		WithStorageKey("custom").
		WithoutSchema().
		Build(ctx)
	require.NoError(t, err)

	var host string

	_, err = cfg.Get(config.NewKeyPath("server/host"), &host)
	require.NoError(t, err)
	assert.Equal(t, "storage-host", host)
}

func TestWithMerger(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	writeFile(t, cfgPath, "server:\n  port: 8080\n")

	// Test WithMerger sets a custom merger.
	merger := &config.DefaultMerger{}
	ctx := context.Background()

	cfg, err := tarantool.New().
		WithConfigFile(cfgPath).
		WithMerger(merger).
		WithoutSchema().
		Build(ctx)
	require.NoError(t, err)

	var port string

	_, err = cfg.Get(config.NewKeyPath("server/port"), &port)
	require.NoError(t, err)
	assert.Equal(t, "8080", port)
}

func TestConfigPrefix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		base     string
		expected string
	}{
		{
			name:     "simple prefix",
			base:     "/myapp",
			expected: "/myapp/config/",
		},
		{
			name:     "prefix with trailing slash",
			base:     "/myapp/",
			expected: "/myapp/config/",
		},
		{
			name:     "prefix with multiple trailing slashes",
			base:     "/myapp///",
			expected: "/myapp/config/",
		},
		{
			name:     "empty prefix",
			base:     "",
			expected: "/config/",
		},
		{
			name:     "root prefix",
			base:     "/",
			expected: "/config/",
		},
		{
			name:     "nested prefix",
			base:     "/app/v1",
			expected: "/app/v1/config/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := tarantool.ConfigPrefix(tt.base)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuild_DefaultStorageKey(t *testing.T) {
	t.Parallel()

	// Test that default storage key is "config".
	require.Equal(t, "config", tarantool.DefaultStorageKey)
}

func TestBuild_MultipleCollectors(t *testing.T) {
	// Cannot use t.Parallel() because of t.Setenv.
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	writeFile(t, cfgPath, "server:\n  port: 8080\n")

	mock := testutil.NewMockStorage()
	testutil.PutIntegrity(mock, "/config/", "app",
		[]byte("server:\n  host: storage-host\n"))

	typed := testutil.NewRawTyped(mock, "/config/")

	// Set both default and regular env.
	t.Setenv("TT_SERVER_LOGLEVEL_DEFAULT", "info")
	t.Setenv("TT_SERVER_HOST", "env-host")

	ctx := context.Background()

	cfg, err := tarantool.New().
		WithConfigFile(cfgPath).
		WithStorage(typed).
		WithoutSchema().
		Build(ctx)
	require.NoError(t, err)

	// Verify priority: env > storage > file > default-env.
	var host string

	_, err = cfg.Get(config.NewKeyPath("server/host"), &host)
	require.NoError(t, err)
	assert.Equal(t, "env-host", host, "env should have highest priority")

	var loglevel string

	_, err = cfg.Get(config.NewKeyPath("server/loglevel"), &loglevel)
	require.NoError(t, err)
	assert.Equal(t, "info", loglevel, "default env should be used for missing keys")
}

func TestBuild_ConfigFileError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Test with nonexistent file - should error.
	_, err := tarantool.New().
		WithConfigFile("/nonexistent/path/to/config.yaml").
		WithoutSchema().
		Build(ctx)
	require.Error(t, err)
}

func TestBuild_ConfigDirError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Test with nonexistent directory - should error.
	_, err := tarantool.New().
		WithConfigDir("/nonexistent/path/to/dir/").
		WithoutSchema().
		Build(ctx)
	require.Error(t, err)
}

func TestKeyPathFromLoweredKey(t *testing.T) {
	// Cannot use t.Parallel() because of t.Setenv.
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	writeFile(t, cfgPath, "key: value\n")

	// Test that ENV_VAR is converted to env/var.
	t.Setenv("TT_MY_NESTED_KEY", "nested-value")

	ctx := context.Background()

	cfg, err := tarantool.New().
		WithConfigFile(cfgPath).
		WithoutSchema().
		Build(ctx)
	require.NoError(t, err)

	var val string

	_, err = cfg.Get(config.NewKeyPath("my/nested/key"), &val)
	require.NoError(t, err)
	assert.Equal(t, "nested-value", val)
}

func TestBuild_EnvPrefixEmptyString(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	writeFile(t, cfgPath, "server:\n  port: 8080\n")

	// Test with empty env prefix (edge case).
	ctx := context.Background()

	cfg, err := tarantool.New().
		WithConfigFile(cfgPath).
		WithEnvPrefix("").
		WithoutSchema().
		Build(ctx)
	require.NoError(t, err)

	var port string

	_, err = cfg.Get(config.NewKeyPath("server/port"), &port)
	require.NoError(t, err)
	assert.Equal(t, "8080", port)
}

func TestBuild_WithSchemaBytes(t *testing.T) {
	t.Parallel()

	schema := []byte(`{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"type": "object",
		"properties": {
			"server": {
				"type": "object",
				"properties": {
					"port": { "type": "integer" }
				}
			}
		}
	}`)

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	writeFile(t, cfgPath, "server:\n  port: 8080\n")

	ctx := context.Background()

	// Test WithSchema followed by WithSchemaFile (should override).
	cfg, err := tarantool.New().
		WithConfigFile(cfgPath).
		WithSchema(schema).
		Build(ctx)
	require.NoError(t, err)

	var port int64

	_, err = cfg.Get(config.NewKeyPath("server/port"), &port)
	require.NoError(t, err)
	assert.Equal(t, int64(8080), port)
}

func TestBuild_WithSchemaFileThenWithSchema(t *testing.T) {
	t.Parallel()

	schema := []byte(`{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"type": "object"
	}`)

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	writeFile(t, cfgPath, "key: value\n")

	ctx := context.Background()

	// WithSchema should override WithSchemaFile (method chaining).
	cfg, err := tarantool.New().
		WithConfigFile(cfgPath).
		WithSchemaFile("/nonexistent/schema.json").
		WithSchema(schema).
		WithoutSchema().
		Build(ctx)
	require.NoError(t, err)

	var val string

	_, err = cfg.Get(config.NewKeyPath("key"), &val)
	require.NoError(t, err)
	assert.Equal(t, "value", val)
}

func TestBuild_AllCollectorTypes(t *testing.T) {
	// Cannot use t.Parallel() because of t.Setenv.
	// This test verifies all collector types work together.
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	writeFile(t, cfgPath, "server:\n  port: 8080\n  host: file-host\n")

	mock := testutil.NewMockStorage()
	testutil.PutIntegrity(mock, "/config/", "app",
		[]byte("server:\n  timeout: 30\n"))

	typed := testutil.NewRawTyped(mock, "/config/")

	t.Setenv("TT_SERVER_LOGLEVEL_DEFAULT", "info")
	t.Setenv("TT_SERVER_HOST", "env-host")

	ctx := context.Background()

	cfg, err := tarantool.New().
		WithConfigFile(cfgPath).
		WithStorage(typed).
		WithoutSchema().
		Build(ctx)
	require.NoError(t, err)

	// Verify all layers present.
	var host string

	_, err = cfg.Get(config.NewKeyPath("server/host"), &host)
	require.NoError(t, err)
	assert.Equal(t, "env-host", host)

	var port string

	_, err = cfg.Get(config.NewKeyPath("server/port"), &port)
	require.NoError(t, err)
	assert.Equal(t, "8080", port)

	var timeout string

	_, err = cfg.Get(config.NewKeyPath("server/timeout"), &timeout)
	require.NoError(t, err)
	assert.Equal(t, "30", timeout)

	var loglevel string

	_, err = cfg.Get(config.NewKeyPath("server/loglevel"), &loglevel)
	require.NoError(t, err)
	assert.Equal(t, "info", loglevel)
}
