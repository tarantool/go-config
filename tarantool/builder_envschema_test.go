package tarantool_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tarantool/go-config"
	"github.com/tarantool/go-config/tarantool"
)

const fixtureSchemaPath = "testdata/config.schema.json"

func TestBuild_Env_SchemaAware_AuditLog(t *testing.T) {
	t.Setenv("TT_AUDIT_LOG_NONBLOCK", "true")

	ctx := context.Background()

	cfg, err := tarantool.New().
		WithSchemaFile(fixtureSchemaPath).
		Build(ctx)
	require.NoError(t, err)

	var nonblock string

	_, err = cfg.Get(config.NewKeyPath("audit_log/nonblock"), &nonblock)
	require.NoError(t, err)
	assert.Equal(t, "true", nonblock)
}

func TestBuild_Env_SchemaAware_WalQueueMaxSize(t *testing.T) {
	t.Setenv("TT_WAL_QUEUE_MAX_SIZE", "123")

	ctx := context.Background()

	cfg, err := tarantool.New().
		WithSchemaFile(fixtureSchemaPath).
		Build(ctx)
	require.NoError(t, err)

	var size string

	_, err = cfg.Get(config.NewKeyPath("wal_queue_max_size"), &size)
	require.NoError(t, err)
	assert.Equal(t, "123", size)
}

func TestBuild_Env_SchemaAware_ReplicationFailover(t *testing.T) {
	t.Setenv("TT_REPLICATION_FAILOVER", "manual")

	ctx := context.Background()

	cfg, err := tarantool.New().
		WithSchemaFile(fixtureSchemaPath).
		Build(ctx)
	require.NoError(t, err)

	var failover string

	_, err = cfg.Get(config.NewKeyPath("replication/failover"), &failover)
	require.NoError(t, err)
	assert.Equal(t, "manual", failover)
}

func TestBuild_Env_SchemaAware_IprotoListen(t *testing.T) {
	t.Setenv("TT_IPROTO_LISTEN", "3301")

	ctx := context.Background()

	cfg, err := tarantool.New().
		WithSchemaFile(fixtureSchemaPath).
		Build(ctx)
	require.NoError(t, err)

	var listen string

	_, err = cfg.Get(config.NewKeyPath("iproto/listen"), &listen)
	require.NoError(t, err)
	assert.Equal(t, "3301", listen)
}

func TestBuild_Env_SchemaAware_UnknownSkipped(t *testing.T) {
	t.Setenv("TT_UNKNOWN_THING", "x")

	ctx := context.Background()

	cfg, err := tarantool.New().
		WithSchemaFile(fixtureSchemaPath).
		Build(ctx)
	require.NoError(t, err)

	_, ok := cfg.Lookup(config.NewKeyPath("unknown/thing"))
	assert.False(t, ok, "unknown env var should not be applied")

	_, ok = cfg.Lookup(config.NewKeyPath("unknown_thing"))
	assert.False(t, ok, "unknown env var should not be applied as a single segment either")
}

func TestBuild_Env_SchemaAware_DefaultSuffix(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	writeFile(t, cfgPath, "audit_log:\n  nonblock: from-file\n")

	t.Setenv("TT_AUDIT_LOG_NONBLOCK_DEFAULT", "from-default-env")

	ctx := context.Background()

	cfg, err := tarantool.New().
		WithConfigFile(cfgPath).
		WithSchemaFile(fixtureSchemaPath).
		Build(ctx)
	require.NoError(t, err)

	var nonblock string

	_, err = cfg.Get(config.NewKeyPath("audit_log/nonblock"), &nonblock)
	require.NoError(t, err)
	assert.Equal(t, "from-file", nonblock, "file should override default-env")
}

func TestBuild_Env_NoSchema_HeuristicUnchanged(t *testing.T) {
	t.Setenv("TT_AUDIT_LOG_NONBLOCK", "true")

	ctx := context.Background()

	cfg, err := tarantool.New().
		WithoutSchema().
		Build(ctx)
	require.NoError(t, err)

	var nonblock string

	_, err = cfg.Get(config.NewKeyPath("audit/log/nonblock"), &nonblock)
	require.NoError(t, err)
	assert.Equal(t, "true", nonblock,
		"without schema, naive split sends the value to the wrong path")
}

func TestBuild_Env_SchemaAware_Wildcard(t *testing.T) {
	t.Setenv("TT_GROUPS_FOO_REPLICASETS_BAR_INSTANCES_BAZ_IPROTO_LISTEN", "3302")

	ctx := context.Background()

	cfg, err := tarantool.New().
		WithSchemaFile(fixtureSchemaPath).
		Build(ctx)
	require.NoError(t, err)

	var listen string

	_, err = cfg.Get(config.NewKeyPath(
		"groups/foo/replicasets/bar/instances/baz/iproto/listen"), &listen)
	require.NoError(t, err)
	assert.Equal(t, "3302", listen)
}
