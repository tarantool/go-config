package tarantool_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tarantool/go-config"
	"github.com/tarantool/go-config/internal/testutil"
	"github.com/tarantool/go-config/tarantool"
)

func TestNew(t *testing.T) {
	t.Parallel()

	b := tarantool.New()
	require.NotNil(t, b)
}

func TestBuild_ConfigFileOnly(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	writeFile(t, cfgPath,
		"groups:\n  storages:\n    replicasets:\n      s-001:\n        instances:\n"+
			"          s-001-a:\n            iproto:\n              listen: 3301\n")

	ctx := context.Background()

	cfg, err := tarantool.New().
		WithConfigFile(cfgPath).
		WithoutSchema().
		Build(ctx)
	require.NoError(t, err)

	var listen string

	_, err = cfg.Get(config.NewKeyPath(
		"groups/storages/replicasets/s-001/instances/s-001-a/iproto/listen"), &listen)
	require.NoError(t, err)
	assert.Equal(t, "3301", listen)
}

func TestBuild_ConfigDirOnly(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "server.yaml"), "server:\n  port: 8080\n")
	writeFile(t, filepath.Join(dir, "database.yaml"), "database:\n  driver: postgres\n")

	ctx := context.Background()

	cfg, err := tarantool.New().
		WithConfigDir(dir).
		WithoutSchema().
		Build(ctx)
	require.NoError(t, err)

	var port string

	_, err = cfg.Get(config.NewKeyPath("server/port"), &port)
	require.NoError(t, err)
	assert.Equal(t, "8080", port)

	var driver string

	_, err = cfg.Get(config.NewKeyPath("database/driver"), &driver)
	require.NoError(t, err)
	assert.Equal(t, "postgres", driver)
}

func TestBuild_MutuallyExclusive(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	_, err := tarantool.New().
		WithConfigFile("/some/file.yaml").
		WithConfigDir("/some/dir").
		WithoutSchema().
		Build(ctx)
	require.ErrorIs(t, err, tarantool.ErrMutuallyExclusive)
}

func TestBuild_EnvOverridesFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	writeFile(t, cfgPath, "server:\n  port: 8080\n  host: localhost\n")

	t.Setenv("TT_SERVER_HOST", "override-host")

	ctx := context.Background()

	cfg, err := tarantool.New().
		WithConfigFile(cfgPath).
		WithoutSchema().
		Build(ctx)
	require.NoError(t, err)

	var host string

	_, err = cfg.Get(config.NewKeyPath("server/host"), &host)
	require.NoError(t, err)
	assert.Equal(t, "override-host", host)

	// File value should still be present for non-overridden keys.
	var port string

	_, err = cfg.Get(config.NewKeyPath("server/port"), &port)
	require.NoError(t, err)
	assert.Equal(t, "8080", port)
}

func TestBuild_DefaultEnvLowestPriority(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	writeFile(t, cfgPath, "server:\n  port: 8080\n")

	// Default env should NOT override the file value.
	t.Setenv("TT_SERVER_PORT_DEFAULT", "9999")
	// Default env for a missing key should appear.
	t.Setenv("TT_SERVER_HOST_DEFAULT", "default-host")

	ctx := context.Background()

	cfg, err := tarantool.New().
		WithConfigFile(cfgPath).
		WithoutSchema().
		Build(ctx)
	require.NoError(t, err)

	// File value wins over default env.
	var port string

	_, err = cfg.Get(config.NewKeyPath("server/port"), &port)
	require.NoError(t, err)
	assert.Equal(t, "8080", port, "file should override default env")

	var host string

	_, err = cfg.Get(config.NewKeyPath("server/host"), &host)
	require.NoError(t, err)
	assert.Equal(t, "default-host", host, "default env should fill missing keys")
}

func TestBuild_StorageOverridesFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	writeFile(t, cfgPath, "server:\n  port: 8080\n  host: file-host\n")

	mock := testutil.NewMockStorage()
	testutil.PutIntegrity(mock, "/config/", "app",
		[]byte("server:\n  host: storage-host\n"))

	typed := testutil.NewRawTyped(mock, "/config/")
	ctx := context.Background()

	cfg, err := tarantool.New().
		WithConfigFile(cfgPath).
		WithStorage(typed).
		WithoutSchema().
		Build(ctx)
	require.NoError(t, err)

	var host string

	_, err = cfg.Get(config.NewKeyPath("server/host"), &host)
	require.NoError(t, err)
	assert.Equal(t, "storage-host", host, "storage should override file")

	var port string

	_, err = cfg.Get(config.NewKeyPath("server/port"), &port)
	require.NoError(t, err)
	assert.Equal(t, "8080", port, "file value should remain for non-overridden keys")
}

func TestBuild_EnvOverridesStorage(t *testing.T) {
	mock := testutil.NewMockStorage()
	testutil.PutIntegrity(mock, "/config/", "app",
		[]byte("server:\n  host: storage-host\n"))

	typed := testutil.NewRawTyped(mock, "/config/")

	t.Setenv("TT_SERVER_HOST", "env-host")

	ctx := context.Background()

	cfg, err := tarantool.New().
		WithStorage(typed).
		WithoutSchema().
		Build(ctx)
	require.NoError(t, err)

	var host string

	_, err = cfg.Get(config.NewKeyPath("server/host"), &host)
	require.NoError(t, err)
	assert.Equal(t, "env-host", host, "env should override storage")
}

func TestBuild_FullStack(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	writeFile(t, cfgPath, "server:\n  port: 8080\n  host: file-host\n  mode: production\n")

	mock := testutil.NewMockStorage()
	testutil.PutIntegrity(mock, "/config/", "app",
		[]byte("server:\n  host: storage-host\n  timeout: 30\n"))

	typed := testutil.NewRawTyped(mock, "/config/")

	// Default env: lowest.
	t.Setenv("TT_SERVER_LOGLEVEL_DEFAULT", "info")
	// Regular env: highest.
	t.Setenv("TT_SERVER_HOST", "env-host")

	ctx := context.Background()

	cfg, err := tarantool.New().
		WithConfigFile(cfgPath).
		WithStorage(typed).
		WithoutSchema().
		Build(ctx)
	require.NoError(t, err)

	// env-host wins (env > storage > file > default-env).
	var host string

	_, err = cfg.Get(config.NewKeyPath("server/host"), &host)
	require.NoError(t, err)
	assert.Equal(t, "env-host", host)

	// From file (not overridden).
	var port string

	_, err = cfg.Get(config.NewKeyPath("server/port"), &port)
	require.NoError(t, err)
	assert.Equal(t, "8080", port)

	// From storage (not overridden).
	var timeout string

	_, err = cfg.Get(config.NewKeyPath("server/timeout"), &timeout)
	require.NoError(t, err)
	assert.Equal(t, "30", timeout)

	// From default env.
	var loglevel string

	_, err = cfg.Get(config.NewKeyPath("server/loglevel"), &loglevel)
	require.NoError(t, err)
	assert.Equal(t, "info", loglevel)
}

func TestBuild_Inheritance(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	writeFile(t, cfgPath, `
replication:
  failover: manual
groups:
  storages:
    replicasets:
      s-001:
        leader: s-001-a
        instances:
          s-001-a:
            iproto:
              listen: 3301
`)

	ctx := context.Background()

	cfg, err := tarantool.New().
		WithConfigFile(cfgPath).
		WithoutSchema().
		Build(ctx)
	require.NoError(t, err)

	instanceCfg, err := cfg.Effective(
		config.NewKeyPath("groups/storages/replicasets/s-001/instances/s-001-a"))
	require.NoError(t, err)

	// Inherited from global.
	var failover string

	_, err = instanceCfg.Get(config.NewKeyPath("replication/failover"), &failover)
	require.NoError(t, err)
	assert.Equal(t, "manual", failover)

	// leader should NOT be inherited (NoInherit).
	_, ok := instanceCfg.Lookup(config.NewKeyPath("leader"))
	assert.False(t, ok, "leader should not be inherited")
}

func TestBuild_InheritanceMergeDeep(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	writeFile(t, cfgPath, `
groups:
  storages:
    credentials:
      users:
        admin:
          password: admin123
    replicasets:
      s-001:
        credentials:
          users:
            monitor:
              password: monitor123
        instances:
          s-001-a:
            credentials:
              users:
                operator:
                  password: op123
`)

	ctx := context.Background()

	cfg, err := tarantool.New().
		WithConfigFile(cfgPath).
		WithoutSchema().
		Build(ctx)
	require.NoError(t, err)

	instanceCfg, err := cfg.Effective(
		config.NewKeyPath("groups/storages/replicasets/s-001/instances/s-001-a"))
	require.NoError(t, err)

	// Credentials: MergeDeep — all users present from all levels.
	var adminPass string

	_, err = instanceCfg.Get(config.NewKeyPath("credentials/users/admin/password"), &adminPass)
	require.NoError(t, err)
	assert.Equal(t, "admin123", adminPass)

	var monitorPass string

	_, err = instanceCfg.Get(config.NewKeyPath("credentials/users/monitor/password"), &monitorPass)
	require.NoError(t, err)
	assert.Equal(t, "monitor123", monitorPass)

	var opPass string

	_, err = instanceCfg.Get(config.NewKeyPath("credentials/users/operator/password"), &opPass)
	require.NoError(t, err)
	assert.Equal(t, "op123", opPass)
}

func TestBuild_CustomInheritanceOption(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	// Use maps (not arrays) so MergeDeep works with YAML-sourced data.
	writeFile(t, cfgPath, `
groups:
  storages:
    labels:
      env: production
    replicasets:
      s-001:
        labels:
          region: us-east
        instances:
          s-001-a: {}
`)

	ctx := context.Background()

	cfg, err := tarantool.New().
		WithConfigFile(cfgPath).
		WithoutSchema().
		WithInheritanceOption(
			config.WithInheritMerge("labels", config.MergeDeep),
		).
		Build(ctx)
	require.NoError(t, err)

	instanceCfg, err := cfg.Effective(
		config.NewKeyPath("groups/storages/replicasets/s-001/instances/s-001-a"))
	require.NoError(t, err)

	var env string

	_, err = instanceCfg.Get(config.NewKeyPath("labels/env"), &env)
	require.NoError(t, err)
	assert.Equal(t, "production", env)

	var region string

	_, err = instanceCfg.Get(config.NewKeyPath("labels/region"), &region)
	require.NoError(t, err)
	assert.Equal(t, "us-east", region)
}

func TestBuild_WithSchema(t *testing.T) {
	t.Parallel()

	// YAML parser stores all scalars as strings, so schema must accept strings.
	schema := []byte(`{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"type": "object",
		"properties": {
			"server": {
				"type": "object",
				"properties": {
					"host": { "type": "string" },
					"port": { "type": "integer" }
				}
			}
		},
		"additionalProperties": false
	}`)

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	writeFile(t, cfgPath, "server:\n  host: localhost\n  port: 8080\n")

	ctx := context.Background()

	cfg, err := tarantool.New().
		WithConfigFile(cfgPath).
		WithSchema(schema).
		Build(ctx)
	require.NoError(t, err)

	var host string

	_, err = cfg.Get(config.NewKeyPath("server/host"), &host)
	require.NoError(t, err)
	assert.Equal(t, "localhost", host)
}

func TestBuild_WithSchemaValidationError(t *testing.T) {
	t.Parallel()

	schema := []byte(`{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"type": "object",
		"properties": {
			"server": {
				"type": "object",
				"properties": {
					"port": { "type": "string" }
				}
			}
		},
		"additionalProperties": false
	}`)

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	// YAML stores 80 as int64, schema expects string → validation fails.
	writeFile(t, cfgPath, "server:\n  port: 80\n")

	ctx := context.Background()

	_, err := tarantool.New().
		WithConfigFile(cfgPath).
		WithSchema(schema).
		Build(ctx)
	require.Error(t, err)
}

func TestBuild_WithSchemaFile(t *testing.T) {
	t.Parallel()

	schema := `{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"type": "object",
		"properties": {
			"port": { "type": "integer" }
		},
		"additionalProperties": false
	}`

	dir := t.TempDir()
	schemaPath := filepath.Join(dir, "schema.json")
	writeFile(t, schemaPath, schema)

	cfgPath := filepath.Join(dir, "config.yaml")
	writeFile(t, cfgPath, "port: 8080\n")

	ctx := context.Background()

	cfg, err := tarantool.New().
		WithConfigFile(cfgPath).
		WithSchemaFile(schemaPath).
		Build(ctx)
	require.NoError(t, err)

	var port int64

	_, err = cfg.Get(config.NewKeyPath("port"), &port)
	require.NoError(t, err)
	assert.Equal(t, int64(8080), port)
}

func TestBuild_WithSchemaFile_NotFound(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	writeFile(t, cfgPath, "port: 8080\n")

	ctx := context.Background()

	_, err := tarantool.New().
		WithConfigFile(cfgPath).
		WithSchemaFile("/nonexistent/schema.json").
		Build(ctx)
	require.ErrorIs(t, err, tarantool.ErrSchemaRead)
}

func TestBuild_WithoutSchema(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	writeFile(t, cfgPath, "anything:\n  goes: true\n")

	ctx := context.Background()

	cfg, err := tarantool.New().
		WithConfigFile(cfgPath).
		WithoutSchema().
		Build(ctx)
	require.NoError(t, err)

	var goes string

	_, err = cfg.Get(config.NewKeyPath("anything/goes"), &goes)
	require.NoError(t, err)
	assert.Equal(t, "true", goes)
}

func TestBuild_CustomEnvPrefix(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	writeFile(t, cfgPath, "server:\n  port: 8080\n")

	t.Setenv("MYAPP_SERVER_HOST", "custom-host")

	ctx := context.Background()

	cfg, err := tarantool.New().
		WithConfigFile(cfgPath).
		WithEnvPrefix("MYAPP_").
		WithoutSchema().
		Build(ctx)
	require.NoError(t, err)

	var host string

	_, err = cfg.Get(config.NewKeyPath("server/host"), &host)
	require.NoError(t, err)
	assert.Equal(t, "custom-host", host)
}

func TestBuild_BuildMutable(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	writeFile(t, cfgPath, "server:\n  port: 8080\n")

	ctx := context.Background()

	cfg, err := tarantool.New().
		WithConfigFile(cfgPath).
		WithoutSchema().
		BuildMutable(ctx)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	var port string

	_, err = cfg.Get(config.NewKeyPath("server/port"), &port)
	require.NoError(t, err)
	assert.Equal(t, "8080", port)
}

func TestBuild_EmptyBuilder(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	_, err := tarantool.New().
		WithoutSchema().
		Build(ctx)
	require.NoError(t, err)
}

func TestBuild_MutuallyExclusive_BuildMutable(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	_, err := tarantool.New().
		WithConfigFile("/some/file.yaml").
		WithConfigDir("/some/dir").
		WithoutSchema().
		BuildMutable(ctx)
	require.ErrorIs(t, err, tarantool.ErrMutuallyExclusive)
}

func TestDefaultEnvTransform_SkipsNonDefault(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	writeFile(t, cfgPath, "server:\n  port: 8080\n")

	t.Setenv("TT_SERVER_PORT", "9999")

	ctx := context.Background()

	cfg, err := tarantool.New().
		WithConfigFile(cfgPath).
		WithoutSchema().
		Build(ctx)
	require.NoError(t, err)

	// The regular env (highest priority) overrides the file.
	var port string

	_, err = cfg.Get(config.NewKeyPath("server/port"), &port)
	require.NoError(t, err)
	assert.Equal(t, "9999", port)
}

func TestWithSchema_OverridesWithSchemaFile(t *testing.T) {
	t.Parallel()

	schema := []byte(`{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"type": "object"
	}`)

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	writeFile(t, cfgPath, "key: value\n")

	ctx := context.Background()

	cfg, err := tarantool.New().
		WithConfigFile(cfgPath).
		WithSchemaFile("/nonexistent/should-not-be-read.json").
		WithSchema(schema).
		Build(ctx)
	require.NoError(t, err)

	var val string

	_, err = cfg.Get(config.NewKeyPath("key"), &val)
	require.NoError(t, err)
	assert.Equal(t, "value", val)
}

func TestWithoutSchema_OverridesWithSchema(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	writeFile(t, cfgPath, "key: value\n")

	ctx := context.Background()

	cfg, err := tarantool.New().
		WithConfigFile(cfgPath).
		WithSchema([]byte(`{"type":"integer"}`)).
		WithoutSchema().
		Build(ctx)
	require.NoError(t, err)

	var val string

	_, err = cfg.Get(config.NewKeyPath("key"), &val)
	require.NoError(t, err)
	assert.Equal(t, "value", val)
}

// writeFile is a test helper that writes content to a file.
func writeFile(t *testing.T, path, content string) {
	t.Helper()

	err := os.WriteFile(path, []byte(content), 0o600)
	require.NoError(t, err)
}
