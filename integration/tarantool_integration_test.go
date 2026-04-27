//nolint:paralleltest
package integration_test

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

// bigTarantoolConfig is a realistic multi-group, multi-replicaset
// Tarantool cluster configuration covering:
//   - global-level settings: credentials, iproto, log, fiber, replication
//   - two groups (routers, storages) with group-level overrides
//   - multiple replicasets per group with replicaset-level overrides
//   - multiple instances per replicaset with instance-level overrides
//   - inheritance corner-cases: credentials (MergeDeep), roles (MergeReplace),
//     leader (inherited by default), and scalar overrides (MergeReplace)
const bigTarantoolConfig = `
credentials:
  users:
    admin:
      password: 'global-admin-pw'
      roles: ['super']
    replicator:
      password: 'repl-pw'
      roles: ['replication']

iproto:
  advertise:
    peer:
      login: replicator

log:
  level: info
  format: plain

fiber:
  top:
    enabled: false
  slice:
    warn: 0.5
    err: 1.0

replication:
  failover: election
  synchro_timeout: 5
  connect_timeout: 10
  timeout: 1

groups:
  routers:
    roles:
      - roles.metrics-export

    iproto:
      listen:
        - uri: 0.0.0.0:3301

    credentials:
      users:
        monitor:
          password: 'router-monitor-pw'
          roles: ['monitor']

    replicasets:
      r-001:
        replication:
          failover: off
        instances:
          r-001-a:
            iproto:
              listen:
                - uri: 0.0.0.0:3311

  storages:
    roles:
      - roles.crud-storage

    iproto:
      listen:
        - uri: 0.0.0.0:3302

    credentials:
      users:
        backup:
          password: 'storage-backup-pw'
          roles: ['backup']

    replicasets:
      s-001:
        leader: s-001-a

        credentials:
          users:
            s001_operator:
              password: 'op-pw-s001'
              roles: ['operator']

        replication:
          synchro_timeout: 10

        instances:
          s-001-a:
            iproto:
              listen:
                - uri: 0.0.0.0:3321

          s-001-b:
            iproto:
              listen:
                - uri: 0.0.0.0:3322

      s-002:
        leader: s-002-a

        roles:
          - roles.metrics-export

        instances:
          s-002-a:
            iproto:
              listen:
                - uri: 0.0.0.0:3331

            credentials:
              users:
                instance_admin:
                  password: 'inst-admin-pw'
                  roles: ['admin']
`

// storageOverrideYAML is put into etcd to override specific keys
// from the file-based config.
const storageOverrideYAML = `
log:
  level: warn

replication:
  connect_timeout: 30
`

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()

	err := os.WriteFile(path, []byte(content), 0o600)
	require.NoError(t, err)
}

// TestTarantool_Integration_FullStack exercises the tarantool wrapper with:
//   - A big YAML config file on disk
//   - etcd storage overriding some keys
//   - Environment variable overrides (highest priority)
//   - Default environment variables (lowest priority)
//   - Inheritance resolution (Effective / EffectiveAll)
//   - No schema validation (WithoutSchema) to test without network dependency
//
// Priority: default-env < file < storage < env.
func TestTarantool_Integration_FullStack(t *testing.T) {
	cluster := testutil.NewEtcdTestCluster(t)
	ctx := context.Background()

	// Write YAML config to disk.
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	writeTestFile(t, cfgPath, bigTarantoolConfig)

	// Put overrides into etcd.
	basePrefix := "/"
	cfgPrefix := tarantool.ConfigPrefix(basePrefix) // "/config/".
	typed := testutil.NewRawTyped(cluster.Storage, cfgPrefix)

	err := typed.Put(ctx, "overrides", []byte(storageOverrideYAML))
	require.NoError(t, err)

	// Set environment variables.
	// Regular env (highest priority): override replication.timeout.
	t.Setenv("TT_REPLICATION_TIMEOUT", "99")
	// Default env (lowest priority): fill a missing key.
	t.Setenv("TT_FIBER_IO_COLLECT_INTERVAL_DEFAULT", "0.01")

	// Build config.
	cfg, err := tarantool.New().
		WithConfigFile(cfgPath).
		WithStorage(testutil.NewRawTyped(cluster.Storage, cfgPrefix)).
		WithoutSchema().
		Build(ctx)
	require.NoError(t, err)

	// Verify raw config (before inheritance) — priority checks.

	// 5a. From file (not overridden).
	var failover string

	_, err = cfg.Get(config.NewKeyPath("replication/failover"), &failover)
	require.NoError(t, err)
	assert.Equal(t, "election", failover, "file value should be present")

	// 5b. Storage overrides file: log.level.
	var logLevel string

	_, err = cfg.Get(config.NewKeyPath("log/level"), &logLevel)
	require.NoError(t, err)
	assert.Equal(t, "warn", logLevel, "storage should override file for log.level")

	// 5c. Storage overrides file: replication.connect_timeout.
	var connectTimeout int64

	_, err = cfg.Get(config.NewKeyPath("replication/connect_timeout"), &connectTimeout)
	require.NoError(t, err)
	assert.Equal(t, int64(30), connectTimeout, "storage should override file for connect_timeout")

	// 5d. Regular env overrides everything: replication.timeout.
	var timeout string

	_, err = cfg.Get(config.NewKeyPath("replication/timeout"), &timeout)
	require.NoError(t, err)
	assert.Equal(t, "99", timeout, "env should override storage+file for timeout")

	// 5e. Default env fills missing key.
	var ioCollect string

	_, err = cfg.Get(config.NewKeyPath("fiber/io/collect/interval"), &ioCollect)
	require.NoError(t, err)
	assert.Equal(t, "0.01", ioCollect, "default env should fill missing key")

	// 5f. File value not overridden by default env.
	var logFormat string

	_, err = cfg.Get(config.NewKeyPath("log/format"), &logFormat)
	require.NoError(t, err)
	assert.Equal(t, "plain", logFormat, "file value should persist")

	// Verify raw hierarchical values from the big config.

	// 6a. Global credential.
	var adminPw string

	_, err = cfg.Get(config.NewKeyPath("credentials/users/admin/password"), &adminPw)
	require.NoError(t, err)
	assert.Equal(t, "global-admin-pw", adminPw)

	// 6b. Group-level iproto for routers.
	var routerURI string

	_, err = cfg.Get(config.NewKeyPath(
		"groups/routers/iproto/listen/0/uri"), &routerURI)
	require.NoError(t, err)
	assert.Equal(t, "0.0.0.0:3301", routerURI)

	// 6c. Instance-level iproto.
	var instanceURI string

	_, err = cfg.Get(config.NewKeyPath(
		"groups/storages/replicasets/s-001/instances/s-001-a/iproto/listen/0/uri"), &instanceURI)
	require.NoError(t, err)
	assert.Equal(t, "0.0.0.0:3321", instanceURI)

	// Inheritance: Effective for a specific instance.

	// 7a. Router instance r-001-a.
	routerCfg, err := cfg.Effective(
		config.NewKeyPath("groups/routers/replicasets/r-001/instances/r-001-a"))
	require.NoError(t, err)

	// Inherited from global: credentials/users/admin.
	var rAdminPw string

	_, err = routerCfg.Get(config.NewKeyPath("credentials/users/admin/password"), &rAdminPw)
	require.NoError(t, err)
	assert.Equal(t, "global-admin-pw", rAdminPw, "admin should be inherited from global")

	// MergeDeep: group-level monitor user merged with global users.
	var rMonitorPw string

	_, err = routerCfg.Get(config.NewKeyPath("credentials/users/monitor/password"), &rMonitorPw)
	require.NoError(t, err)
	assert.Equal(t, "router-monitor-pw", rMonitorPw,
		"MergeDeep should merge group monitor user into inherited credentials")

	// Replicaset-level override: replication.failover = off.
	var rFailover string

	_, err = routerCfg.Get(config.NewKeyPath("replication/failover"), &rFailover)
	require.NoError(t, err)
	assert.Equal(t, "off", rFailover,
		"replicaset-level replication.failover should override global")

	// Instance-level override: iproto.listen.
	var rInstanceURI string

	_, err = routerCfg.Get(config.NewKeyPath("iproto/listen/0/uri"), &rInstanceURI)
	require.NoError(t, err)
	assert.Equal(t, "0.0.0.0:3311", rInstanceURI,
		"instance iproto.listen should override group value")

	// MergeReplace: group roles inherited.
	var rRole0 string

	_, err = routerCfg.Get(config.NewKeyPath("roles/0"), &rRole0)
	require.NoError(t, err)
	assert.Equal(t, "roles.metrics-export", rRole0,
		"group-level role should be present")

	// 7b. Storage instance s-001-a.
	s001aCfg, err := cfg.Effective(
		config.NewKeyPath("groups/storages/replicasets/s-001/instances/s-001-a"))
	require.NoError(t, err)

	// MergeDeep: credentials from global + group + replicaset all merged.
	var sAdminPw string

	_, err = s001aCfg.Get(config.NewKeyPath("credentials/users/admin/password"), &sAdminPw)
	require.NoError(t, err)
	assert.Equal(t, "global-admin-pw", sAdminPw,
		"MergeDeep: global admin should be present")

	var sBackupPw string

	_, err = s001aCfg.Get(config.NewKeyPath("credentials/users/backup/password"), &sBackupPw)
	require.NoError(t, err)
	assert.Equal(t, "storage-backup-pw", sBackupPw,
		"MergeDeep: group-level backup user should be present")

	var sOpPw string

	_, err = s001aCfg.Get(config.NewKeyPath("credentials/users/s001_operator/password"), &sOpPw)
	require.NoError(t, err)
	assert.Equal(t, "op-pw-s001", sOpPw,
		"MergeDeep: replicaset-level operator should be present")

	// leader is inherited by default from replicaset level.
	var leader string

	_, err = s001aCfg.Get(config.NewKeyPath("leader"), &leader)
	require.NoError(t, err)
	assert.Equal(t, "s-001-a", leader,
		"leader should be inherited from replicaset")

	// Replicaset-level override: synchro_timeout = 10.
	var synchroTimeout int64

	_, err = s001aCfg.Get(config.NewKeyPath("replication/synchro_timeout"), &synchroTimeout)
	require.NoError(t, err)
	assert.Equal(t, int64(10), synchroTimeout,
		"replicaset-level synchro_timeout should override global")

	// Instance-level iproto.
	var sInstanceURI string

	_, err = s001aCfg.Get(config.NewKeyPath("iproto/listen/0/uri"), &sInstanceURI)
	require.NoError(t, err)
	assert.Equal(t, "0.0.0.0:3321", sInstanceURI)

	// MergeReplace: group roles.
	var sRole0 string

	_, err = s001aCfg.Get(config.NewKeyPath("roles/0"), &sRole0)
	require.NoError(t, err)
	assert.Equal(t, "roles.crud-storage", sRole0,
		"group-level role should be present")

	// 7c. Storage instance s-001-b — shares replicaset with s-001-a.
	s001bCfg, err := cfg.Effective(
		config.NewKeyPath("groups/storages/replicasets/s-001/instances/s-001-b"))
	require.NoError(t, err)

	var s001bURI string

	_, err = s001bCfg.Get(config.NewKeyPath("iproto/listen/0/uri"), &s001bURI)
	require.NoError(t, err)
	assert.Equal(t, "0.0.0.0:3322", s001bURI,
		"s-001-b should have its own iproto.listen")

	// Same replicaset credentials.
	var s001bOpPw string

	_, err = s001bCfg.Get(config.NewKeyPath("credentials/users/s001_operator/password"), &s001bOpPw)
	require.NoError(t, err)
	assert.Equal(t, "op-pw-s001", s001bOpPw,
		"MergeDeep: replicaset credentials inherited to both instances")

	// 7d. Storage instance s-002-a — different replicaset.
	s002aCfg, err := cfg.Effective(
		config.NewKeyPath("groups/storages/replicasets/s-002/instances/s-002-a"))
	require.NoError(t, err)

	// MergeDeep: instance-level credential.
	var s002aInstAdminPw string

	_, err = s002aCfg.Get(config.NewKeyPath("credentials/users/instance_admin/password"), &s002aInstAdminPw)
	require.NoError(t, err)
	assert.Equal(t, "inst-admin-pw", s002aInstAdminPw,
		"MergeDeep: instance-level credential should be present")

	// s-002 also has global admin via MergeDeep.
	var s002aAdminPw string

	_, err = s002aCfg.Get(config.NewKeyPath("credentials/users/admin/password"), &s002aAdminPw)
	require.NoError(t, err)
	assert.Equal(t, "global-admin-pw", s002aAdminPw,
		"MergeDeep: global admin still present in s-002-a")

	// roles: replicaset-level roles replace group-level roles.
	var s002Role0 string

	_, err = s002aCfg.Get(config.NewKeyPath("roles/0"), &s002Role0)
	require.NoError(t, err)
	assert.Equal(t, "roles.metrics-export", s002Role0,
		"replicaset-level roles should replace group-level roles")

	// s-002-a should NOT have s-001 operator (different replicaset).
	_, s001OpFound := s002aCfg.Lookup(config.NewKeyPath("credentials/users/s001_operator"))
	assert.False(t, s001OpFound,
		"s-002-a should not have s-001 replicaset credentials")

	// EffectiveAll — resolve all instances at once.

	all, err := cfg.EffectiveAll()
	require.NoError(t, err)

	expectedInstances := []string{
		"groups/routers/replicasets/r-001/instances/r-001-a",
		"groups/storages/replicasets/s-001/instances/s-001-a",
		"groups/storages/replicasets/s-001/instances/s-001-b",
		"groups/storages/replicasets/s-002/instances/s-002-a",
	}

	assert.Len(t, all, len(expectedInstances), "EffectiveAll should find all instances")

	for _, path := range expectedInstances {
		instanceCfg, ok := all[path]
		assert.True(t, ok, "EffectiveAll should include %s", path)

		// Every instance should have the global admin credential (MergeDeep).
		var pw string

		_, err = instanceCfg.Get(config.NewKeyPath("credentials/users/admin/password"), &pw)
		require.NoError(t, err, "instance %s should have admin credential", path)
		assert.Equal(t, "global-admin-pw", pw)
	}
}

func writeRealSchemaConfig(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	writeTestFile(t, cfgPath, `
log:
  level: info
  format: plain

replication:
  failover: election
  synchro_timeout: 5

groups:
  storages:
    roles:
      - roles.crud-storage
    replicasets:
      s-001:
        instances:
          s-001-a: {}
          s-001-b:
            roles:
              - roles.metrics-export
`)

	return cfgPath
}

func buildRealSchemaConfig(t *testing.T) config.Config {
	t.Helper()

	cfgPath := writeRealSchemaConfig(t)

	cfg, err := tarantool.New().
		WithConfigFile(cfgPath).
		WithEnvPrefix("TT_TESTONLY_").
		Build(context.Background())
	require.NoError(t, err, "config should pass real Tarantool schema validation")

	return cfg
}

// TestTarantool_Integration_WithRealSchema_Build validates a config against
// the embedded Tarantool schema bundle and reads top-level values.
func TestTarantool_Integration_WithRealSchema_Build(t *testing.T) {
	cfg := buildRealSchemaConfig(t)

	var logLevel string

	_, err := cfg.Get(config.NewKeyPath("log/level"), &logLevel)
	require.NoError(t, err)
	assert.Equal(t, "info", logLevel)

	var failover string

	_, err = cfg.Get(config.NewKeyPath("replication/failover"), &failover)
	require.NoError(t, err)
	assert.Equal(t, "election", failover)
}

// TestTarantool_Integration_WithRealSchema_Inheritance checks that
// inheritance works correctly for instances with and without own roles.
func TestTarantool_Integration_WithRealSchema_Inheritance(t *testing.T) {
	cfg := buildRealSchemaConfig(t)

	instanceACfg, err := cfg.Effective(
		config.NewKeyPath("groups/storages/replicasets/s-001/instances/s-001-a"))
	require.NoError(t, err)

	var inheritedLevel string

	_, err = instanceACfg.Get(config.NewKeyPath("log/level"), &inheritedLevel)
	require.NoError(t, err)
	assert.Equal(t, "info", inheritedLevel,
		"global log.level should be inherited")

	var inheritedFailover string

	_, err = instanceACfg.Get(config.NewKeyPath("replication/failover"), &inheritedFailover)
	require.NoError(t, err)
	assert.Equal(t, "election", inheritedFailover,
		"global replication.failover should be inherited")

	var rolesA []string

	_, err = instanceACfg.Get(config.NewKeyPath("roles"), &rolesA)
	require.NoError(t, err)
	assert.Equal(t, []string{"roles.crud-storage"}, rolesA,
		"instance without own roles should inherit group roles")

	instanceBCfg, err := cfg.Effective(
		config.NewKeyPath("groups/storages/replicasets/s-001/instances/s-001-b"))
	require.NoError(t, err)

	var rolesB []string

	_, err = instanceBCfg.Get(config.NewKeyPath("roles"), &rolesB)
	require.NoError(t, err)
	assert.Equal(t, []string{"roles.metrics-export"}, rolesB,
		"instance-level roles should replace group-level roles")
}

// TestTarantool_Integration_BuildMutable_WithEtcd tests BuildMutable with
// real etcd, then modifies the config at runtime.
func TestTarantool_Integration_BuildMutable_WithEtcd(t *testing.T) {
	cluster := testutil.NewEtcdTestCluster(t)
	ctx := context.Background()

	basePrefix := "/mutable/"
	cfgPrefix := tarantool.ConfigPrefix(basePrefix) // "/mutable/config/".
	typed := testutil.NewRawTyped(cluster.Storage, cfgPrefix)

	err := typed.Put(ctx, "base", []byte(`
log:
  level: debug
  format: json
`))
	require.NoError(t, err)

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	writeTestFile(t, cfgPath, `
log:
  level: info

groups:
  routers:
    replicasets:
      r-001:
        instances:
          r-001-a: {}
`)

	cfg, err := tarantool.New().
		WithConfigFile(cfgPath).
		WithStorage(testutil.NewRawTyped(cluster.Storage, cfgPrefix)).
		WithoutSchema().
		BuildMutable(ctx)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Storage overrides file: log.level = debug.
	var logLevel string

	_, err = cfg.Get(config.NewKeyPath("log/level"), &logLevel)
	require.NoError(t, err)
	assert.Equal(t, "debug", logLevel, "storage should override file")

	// Mutate at runtime.
	err = cfg.Set(config.NewKeyPath("log/level"), "error")
	require.NoError(t, err)

	_, err = cfg.Get(config.NewKeyPath("log/level"), &logLevel)
	require.NoError(t, err)
	assert.Equal(t, "error", logLevel, "Set should update value")

	// Storage value for log.format should still be present.
	var logFormat string

	_, err = cfg.Get(config.NewKeyPath("log/format"), &logFormat)
	require.NoError(t, err)
	assert.Equal(t, "json", logFormat)
}

// TestTarantool_Integration_ConfigDir_WithEtcd tests WithConfigDir combined
// with etcd storage.
func TestTarantool_Integration_ConfigDir_WithEtcd(t *testing.T) {
	cluster := testutil.NewEtcdTestCluster(t)
	ctx := context.Background()

	// Config directory with two YAML files.
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "global.yaml"), `
credentials:
  users:
    admin:
      password: 'dir-admin-pw'

log:
  level: info
`)
	writeTestFile(t, filepath.Join(dir, "topology.yaml"), `
groups:
  storages:
    replicasets:
      s-001:
        instances:
          s-001-a:
            iproto:
              listen:
                - uri: 0.0.0.0:3301
`)

	// etcd overrides log level.
	basePrefix := "/dirtest/"
	cfgPrefix := tarantool.ConfigPrefix(basePrefix) // "/dirtest/config/".
	typed := testutil.NewRawTyped(cluster.Storage, cfgPrefix)

	err := typed.Put(ctx, "override", []byte("log:\n  level: error\n"))
	require.NoError(t, err)

	cfg, err := tarantool.New().
		WithConfigDir(dir).
		WithStorage(testutil.NewRawTyped(cluster.Storage, cfgPrefix)).
		WithoutSchema().
		Build(ctx)
	require.NoError(t, err)

	// Storage overrides dir: log.level.
	var logLevel string

	_, err = cfg.Get(config.NewKeyPath("log/level"), &logLevel)
	require.NoError(t, err)
	assert.Equal(t, "error", logLevel, "storage should override config dir")

	// Dir value present for non-overridden key.
	var adminPw string

	_, err = cfg.Get(config.NewKeyPath("credentials/users/admin/password"), &adminPw)
	require.NoError(t, err)
	assert.Equal(t, "dir-admin-pw", adminPw)

	// Inheritance from dir topology.
	instanceCfg, err := cfg.Effective(
		config.NewKeyPath("groups/storages/replicasets/s-001/instances/s-001-a"))
	require.NoError(t, err)

	var uri string

	_, err = instanceCfg.Get(config.NewKeyPath("iproto/listen/0/uri"), &uri)
	require.NoError(t, err)
	assert.Equal(t, "0.0.0.0:3301", uri)

	// Inherited global credential.
	var inheritedPw string

	_, err = instanceCfg.Get(config.NewKeyPath("credentials/users/admin/password"), &inheritedPw)
	require.NoError(t, err)
	assert.Equal(t, "dir-admin-pw", inheritedPw,
		"MergeDeep: global credential from dir should be inherited")
}
