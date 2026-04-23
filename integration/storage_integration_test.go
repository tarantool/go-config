// Package integration_test provides integration tests for the Storage collector
// using an embedded etcd instance.
//
// Due to inability to start multiple etcd clusters - we're using one cluster
// and won't start tests in parallel here.
//
//nolint:paralleltest,forbidigo
package integration_test

import (
	"context"
	"fmt"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	etcdclient "go.etcd.io/etcd/client/v3"

	"github.com/tarantool/go-storage/kv"

	"github.com/tarantool/go-config"
	"github.com/tarantool/go-config/collectors"
	"github.com/tarantool/go-config/internal/testutil"
)

const configPrefix = "/config/"

func putYAML(
	ctx context.Context,
	t *testing.T,
	client *etcdclient.Client,
	key string,
	yaml string,
) {
	t.Helper()

	_, err := client.Put(ctx, key, yaml)
	require.NoError(t, err, "Failed to put YAML into etcd")
}

func dumpEtcdKeys(
	ctx context.Context,
	t *testing.T,
	client *etcdclient.Client,
	prefix string,
) []kv.KeyValue {
	t.Helper()

	resp, err := client.Get(ctx, prefix, etcdclient.WithPrefix())
	require.NoError(t, err, "Failed to list etcd keys")

	kvs := make([]kv.KeyValue, 0, len(resp.Kvs))

	for _, entry := range resp.Kvs {
		kvs = append(kvs, kv.KeyValue{
			Key:         entry.Key,
			Value:       entry.Value,
			ModRevision: entry.ModRevision,
		})
	}

	sort.Slice(kvs, func(i, j int) bool {
		return string(kvs[i].Key) < string(kvs[j].Key)
	})

	fmt.Println("=== etcd state ===")

	for _, entry := range kvs {
		fmt.Printf("  %s => %s (rev=%d)\n", entry.Key, entry.Value, entry.ModRevision)
	}

	fmt.Printf("=== %d keys ===\n", len(kvs))

	return kvs
}

func TestStorage_Integration_Etcd(t *testing.T) {
	cluster := testutil.NewEtcdTestCluster(t)
	ctx := context.Background()

	prefix := configPrefix

	// Put YAML configuration via the integrity layer so that
	// the Storage collector can read it with integrity verification.
	typed := testutil.NewRawTyped(cluster.Storage, prefix)

	err := typed.Put(ctx, "app", []byte("server:\n  port: 8080\n  host: localhost"))
	require.NoError(t, err)

	err = typed.Put(ctx, "db", []byte("driver: postgres\nport: 5432"))
	require.NoError(t, err)

	// Dump raw etcd state to stdout.
	kvs := dumpEtcdKeys(ctx, t, cluster.Client, prefix)
	assert.NotEmpty(t, kvs)

	// Build go-config using the Storage collector backed by etcd.
	collector := collectors.NewStorage(
		testutil.NewRawTyped(cluster.Storage, prefix),
		prefix,
		collectors.NewYamlFormat(),
	).WithName("etcd")

	builder := config.NewBuilder()

	builder = builder.AddCollector(collector)

	cfg, errs := builder.Build(t.Context())
	require.Empty(t, errs, "Build should succeed without errors")

	// Verify that config was read correctly.
	var port int

	_, err = cfg.Get(config.NewKeyPath("server/port"), &port)
	require.NoError(t, err)
	assert.Equal(t, 8080, port)

	var host string

	_, err = cfg.Get(config.NewKeyPath("server/host"), &host)
	require.NoError(t, err)
	assert.Equal(t, "localhost", host)

	var dbDriver string

	_, err = cfg.Get(config.NewKeyPath("driver"), &dbDriver)
	require.NoError(t, err)
	assert.Equal(t, "postgres", dbDriver)

	var dbPort int

	_, err = cfg.Get(config.NewKeyPath("port"), &dbPort)
	require.NoError(t, err)
	assert.Equal(t, 5432, dbPort)

	// Verify collector metadata.
	assert.Equal(t, "etcd", collector.Name())
	assert.NotEmpty(t, collector.Revision())
}

func TestStorageSource_Integration_Etcd(t *testing.T) {
	cluster := testutil.NewEtcdTestCluster(t)
	ctx := context.Background()

	prefix := configPrefix

	// Put a single YAML config document via the integrity layer.
	typed := testutil.NewRawTyped(cluster.Storage, prefix)

	err := typed.Put(ctx, "myapp", []byte("log_level: debug\nworkers: 4"))
	require.NoError(t, err)

	// Dump raw etcd state.
	kvs := dumpEtcdKeys(ctx, t, cluster.Client, prefix)
	assert.NotEmpty(t, kvs)

	// Create a StorageSource for a single document.
	source := collectors.NewStorageSource(cluster.Storage, prefix, "myapp", nil, nil)

	collector, err := collectors.NewSource(t.Context(), source, collectors.NewYamlFormat())
	require.NoError(t, err)

	builder := config.NewBuilder()

	builder = builder.AddCollector(collector)

	cfg, errs := builder.Build(t.Context())
	require.Empty(t, errs)

	var logLevel string

	_, err = cfg.Get(config.NewKeyPath("log_level"), &logLevel)
	require.NoError(t, err)
	assert.Equal(t, "debug", logLevel)

	var workers int

	_, err = cfg.Get(config.NewKeyPath("workers"), &workers)
	require.NoError(t, err)
	assert.Equal(t, 4, workers)

	// Verify collector metadata.
	assert.Equal(t, "storage", collector.Name())
	assert.NotEmpty(t, collector.Revision())
}

func TestStorage_Integration_Etcd_Update(t *testing.T) {
	cluster := testutil.NewEtcdTestCluster(t)
	ctx := context.Background()

	prefix := configPrefix
	typed := testutil.NewRawTyped(cluster.Storage, prefix)

	// Initial config.
	err := typed.Put(ctx, "settings", []byte("mode: development"))
	require.NoError(t, err)

	collector := collectors.NewStorage(
		testutil.NewRawTyped(cluster.Storage, prefix),
		prefix,
		collectors.NewYamlFormat(),
	).WithName("etcd")

	builder := config.NewBuilder()

	builder = builder.AddCollector(collector)

	cfg, errs := builder.Build(t.Context())
	require.Empty(t, errs)

	var mode string

	_, err = cfg.Get(config.NewKeyPath("mode"), &mode)
	require.NoError(t, err)
	assert.Equal(t, "development", mode)

	rev1 := collector.Revision()
	assert.NotEmpty(t, rev1)
	assert.Equal(t, "etcd", collector.Name())

	// Update config in etcd.
	err = typed.Put(ctx, "settings", []byte("mode: production"))
	require.NoError(t, err)

	dumpEtcdKeys(ctx, t, cluster.Client, prefix)

	// Re-read — rebuild config.
	builder = config.NewBuilder()
	builder = builder.AddCollector(collector)

	cfg, errs = builder.Build(t.Context())
	require.Empty(t, errs)

	_, err = cfg.Get(config.NewKeyPath("mode"), &mode)
	require.NoError(t, err)
	assert.Equal(t, "production", mode)

	rev2 := collector.Revision()
	assert.NotEmpty(t, rev2)
	assert.NotEqual(t, rev1, rev2)
}

func TestStorage_Integration_Etcd_MultipleKeys(t *testing.T) {
	cluster := testutil.NewEtcdTestCluster(t)
	ctx := context.Background()

	prefix := "/cluster/"
	typed := testutil.NewRawTyped(cluster.Storage, prefix)

	// Each key holds a complete YAML document; key names are only for
	// distinguishing documents in storage. The YAML content itself
	// determines the config tree structure.
	err := typed.Put(ctx, "cfg-instances", []byte(
		"instances:\n"+
			"  i001:\n"+
			"    roles:\n"+
			"      - router\n"+
			"    memory: 1G\n"+
			"  i002:\n"+
			"    roles:\n"+
			"      - storage\n"+
			"    memory: 2G"))
	require.NoError(t, err)

	err = typed.Put(ctx, "cfg-global",
		[]byte("replication:\n  failover: manual"))
	require.NoError(t, err)

	// Dump raw etcd state.
	kvs := dumpEtcdKeys(ctx, t, cluster.Client, prefix)
	assert.NotEmpty(t, kvs)

	// Direct etcd Get to verify what was actually stored.
	resp, err := cluster.Client.Get(ctx, prefix, etcdclient.WithPrefix())
	require.NoError(t, err)
	fmt.Printf("\n=== raw etcd keys (direct) ===\n")

	for _, entry := range resp.Kvs {
		fmt.Printf("  %s => %q\n", entry.Key, entry.Value)
	}

	// Fetch via go-config.
	collector := collectors.NewStorage(
		testutil.NewRawTyped(cluster.Storage, prefix),
		prefix,
		collectors.NewYamlFormat(),
	).WithName("etcd")

	builder := config.NewBuilder()

	builder = builder.AddCollector(collector)

	cfg, errs := builder.Build(t.Context())
	require.Empty(t, errs)

	var memory string

	_, err = cfg.Get(config.NewKeyPath("instances/i001/memory"), &memory)
	require.NoError(t, err)
	assert.Equal(t, "1G", memory)

	var role1 string

	_, err = cfg.Get(config.NewKeyPath("instances/i001/roles/0"), &role1)
	require.NoError(t, err)
	assert.Equal(t, "router", role1)

	_, err = cfg.Get(config.NewKeyPath("instances/i002/memory"), &memory)
	require.NoError(t, err)
	assert.Equal(t, "2G", memory)

	var role2 string

	_, err = cfg.Get(config.NewKeyPath("instances/i002/roles/0"), &role2)
	require.NoError(t, err)
	assert.Equal(t, "storage", role2)

	var failover string

	_, err = cfg.Get(config.NewKeyPath("replication/failover"), &failover)
	require.NoError(t, err)
	assert.Equal(t, "manual", failover)

	// Verify collector metadata.
	assert.Equal(t, "etcd", collector.Name())
	assert.NotEmpty(t, collector.Revision())
}

func TestStorage_Integration_Etcd_RawKV(t *testing.T) {
	cluster := testutil.NewEtcdTestCluster(t)
	ctx := context.Background()

	prefix := "/raw/"

	// Put raw YAML documents directly into etcd (no integrity layer).
	putYAML(ctx, t, cluster.Client, prefix+"app", "server:\n  port: 9090\n  host: 0.0.0.0")
	putYAML(ctx, t, cluster.Client, prefix+"logging", "level: info\nformat: json")

	// Dump what's in etcd.
	dumpEtcdKeys(ctx, t, cluster.Client, prefix)

	// Verify data is retrievable via raw etcd client.
	resp, err := cluster.Client.Get(ctx, prefix, etcdclient.WithPrefix())
	require.NoError(t, err)
	assert.Len(t, resp.Kvs, 2)

	fmt.Printf("\n=== etcd client results ===\n")

	for _, entry := range resp.Kvs {
		fmt.Printf("  %s => %s\n", entry.Key, entry.Value)
	}

	// Verify individual key values from raw etcd response.
	keys := make(map[string]string, len(resp.Kvs))
	for _, entry := range resp.Kvs {
		keys[string(entry.Key)] = string(entry.Value)
	}

	assert.Equal(t, "server:\n  port: 9090\n  host: 0.0.0.0", keys["/raw/app"])
	assert.Equal(t, "level: info\nformat: json", keys["/raw/logging"])
}

func TestStorage_Integration_Etcd_MergeKeys(t *testing.T) {
	cluster := testutil.NewEtcdTestCluster(t)
	ctx := context.Background()

	prefix := "/merge/"
	typed := testutil.NewRawTyped(cluster.Storage, prefix)

	// Two storage keys contribute to the same "database" subtree.
	// Key "database/primary" defines host and port.
	err := typed.Put(ctx, "database/primary",
		[]byte("host: db-primary.local\nport: 5432"))
	require.NoError(t, err)

	// Key "database/settings" defines pool_size and timeout.
	err = typed.Put(ctx, "database/settings",
		[]byte("pool_size: 10\ntimeout: 30"))
	require.NoError(t, err)

	// Key "cache" defines a separate subtree.
	err = typed.Put(ctx, "cache",
		[]byte("driver: redis\nttl: 3600"))
	require.NoError(t, err)

	dumpEtcdKeys(ctx, t, cluster.Client, prefix)

	collector := collectors.NewStorage(
		testutil.NewRawTyped(cluster.Storage, prefix),
		prefix,
		collectors.NewYamlFormat(),
	).WithName("etcd")

	builder := config.NewBuilder()

	builder = builder.AddCollector(collector)

	cfg, errs := builder.Build(t.Context())
	require.Empty(t, errs)

	// Verify that both "database/*" keys are merged into the tree.
	var host string

	_, err = cfg.Get(config.NewKeyPath("host"), &host)
	require.NoError(t, err)
	assert.Equal(t, "db-primary.local", host)

	var port int

	_, err = cfg.Get(config.NewKeyPath("port"), &port)
	require.NoError(t, err)
	assert.Equal(t, 5432, port)

	var poolSize int

	_, err = cfg.Get(config.NewKeyPath("pool_size"), &poolSize)
	require.NoError(t, err)
	assert.Equal(t, 10, poolSize)

	var timeout int

	_, err = cfg.Get(config.NewKeyPath("timeout"), &timeout)
	require.NoError(t, err)
	assert.Equal(t, 30, timeout)

	// Verify the separate "cache" subtree.
	var driver string

	_, err = cfg.Get(config.NewKeyPath("driver"), &driver)
	require.NoError(t, err)
	assert.Equal(t, "redis", driver)

	var ttl int

	_, err = cfg.Get(config.NewKeyPath("ttl"), &ttl)
	require.NoError(t, err)
	assert.Equal(t, 3600, ttl)

	// Verify collector metadata.
	assert.Equal(t, "etcd", collector.Name())
	assert.NotEmpty(t, collector.Revision())
}

func TestStorage_Integration_Etcd_MergeCollectors(t *testing.T) {
	cluster := testutil.NewEtcdTestCluster(t)
	ctx := context.Background()

	prefix := "/override/"
	typed := testutil.NewRawTyped(cluster.Storage, prefix)

	// Storage has partial config that should override map defaults.
	err := typed.Put(ctx, "server",
		[]byte("server:\n  port: 9090\n  log_level: warn"))
	require.NoError(t, err)

	dumpEtcdKeys(ctx, t, cluster.Client, prefix)

	// Map collector provides defaults (added first = lower priority).
	defaults := collectors.NewMap(map[string]any{
		"server": map[string]any{
			"port":      8080,
			"host":      "0.0.0.0",
			"log_level": "info",
		},
		"database": map[string]any{
			"host": "localhost",
			"port": 5432,
		},
	}).WithName("defaults")

	// Storage collector overrides some keys (added second = higher priority).
	storageCollector := collectors.NewStorage(
		testutil.NewRawTyped(cluster.Storage, prefix),
		prefix,
		collectors.NewYamlFormat(),
	).WithName("etcd")

	builder := config.NewBuilder()

	builder = builder.AddCollector(defaults)
	builder = builder.AddCollector(storageCollector)

	cfg, errs := builder.Build(t.Context())
	require.Empty(t, errs)

	// "server/port" overridden by storage collector.
	var port int

	_, err = cfg.Get(config.NewKeyPath("server/port"), &port)
	require.NoError(t, err)
	assert.Equal(t, 9090, port)

	// "server/host" only in defaults, not overridden.
	var host string

	_, err = cfg.Get(config.NewKeyPath("server/host"), &host)
	require.NoError(t, err)
	assert.Equal(t, "0.0.0.0", host)

	// "server/log_level" overridden by storage collector.
	var logLevel string

	_, err = cfg.Get(config.NewKeyPath("server/log_level"), &logLevel)
	require.NoError(t, err)
	assert.Equal(t, "warn", logLevel)

	// "database" subtree only in defaults, untouched by storage.
	var dbHost string

	_, err = cfg.Get(config.NewKeyPath("database/host"), &dbHost)
	require.NoError(t, err)
	assert.Equal(t, "localhost", dbHost)

	var dbPort int

	_, err = cfg.Get(config.NewKeyPath("database/port"), &dbPort)
	require.NoError(t, err)
	assert.Equal(t, 5432, dbPort)
}

func TestStorage_Integration_Etcd_MergeOverwrite(t *testing.T) {
	cluster := testutil.NewEtcdTestCluster(t)
	ctx := context.Background()

	prefix := "/overwrite/"
	typed := testutil.NewRawTyped(cluster.Storage, prefix)

	// Two keys contain YAML with overlapping paths. Since processing
	// order is non-deterministic (integrity layer iterates a Go map),
	// overlapping leaves will be overwritten by whichever key is
	// processed last. We assert that exactly one of the two possible
	// values wins, and that non-overlapping leaves from both keys
	// are always present.
	err := typed.Put(ctx, "cfg-base", []byte(
		"database:\n"+
			"  host: db-base.local\n"+
			"  port: 3306\n"+
			"  pool_size: 10\n"))
	require.NoError(t, err)

	err = typed.Put(ctx, "cfg-override", []byte(
		"database:\n"+
			"  host: db-override.local\n"+
			"  port: 5432\n"+
			"  max_idle: 5\n"))
	require.NoError(t, err)

	dumpEtcdKeys(ctx, t, cluster.Client, prefix)

	collector := collectors.NewStorage(
		testutil.NewRawTyped(cluster.Storage, prefix),
		prefix,
		collectors.NewYamlFormat(),
	).WithName("etcd")

	builder := config.NewBuilder()

	builder = builder.AddCollector(collector)

	cfg, errs := builder.Build(t.Context())
	require.Empty(t, errs)

	ch, err := cfg.Walk(ctx, nil, 1000)
	require.NoError(t, err)

	for entry := range ch {
		fmt.Println(entry)
	}

	// Overlapping key "database/host": one of the two values wins.
	var dbHost string

	_, err = cfg.Get(config.NewKeyPath("database/host"), &dbHost)
	require.NoError(t, err)
	assert.Contains(t,
		[]string{"db-base.local", "db-override.local"}, dbHost)

	// Overlapping key "database/port": one of the two values wins.
	var dbPort int

	_, err = cfg.Get(config.NewKeyPath("database/port"), &dbPort)
	require.NoError(t, err)
	assert.Contains(t, []int{3306, 5432}, dbPort)

	// Non-overlapping key "database/pool_size": always present.
	var poolSize int

	_, err = cfg.Get(config.NewKeyPath("database/pool_size"), &poolSize)
	require.NoError(t, err)
	assert.Equal(t, 10, poolSize)

	// Non-overlapping key "database/max_idle": always present.
	var maxIdle int

	_, err = cfg.Get(config.NewKeyPath("database/max_idle"), &maxIdle)
	require.NoError(t, err)
	assert.Equal(t, 5, maxIdle)

	// Verify collector metadata.
	assert.Equal(t, "etcd", collector.Name())
	assert.NotEmpty(t, collector.Revision())
}
