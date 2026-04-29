package config_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tarantool/go-config"
	"github.com/tarantool/go-config/collectors"
)

func TestConfig_Stat_ExistingKey(t *testing.T) {
	t.Parallel()

	data := map[string]any{
		"server": map[string]any{
			"port": 8080,
		},
	}
	col := collectors.NewMap(data).WithName("test")

	builder := config.NewBuilder()

	builder = builder.AddCollector(col)

	cfg, errs := builder.Build(t.Context())
	require.Empty(t, errs)

	meta, ok := cfg.Stat(config.NewKeyPath("server/port"))
	require.True(t, ok)
	assert.Equal(t, "test", meta.Source.Name)
	assert.Equal(t, config.UnknownSource, meta.Source.Type)
}

func TestConfig_Stat_NonExistentKey(t *testing.T) {
	t.Parallel()

	data := map[string]any{}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.AddCollector(col)

	cfg, errs := builder.Build(t.Context())
	require.Empty(t, errs)

	meta, ok := cfg.Stat(config.NewKeyPath("missing"))
	assert.False(t, ok)
	assert.Empty(t, meta.Source.Name)
	assert.Equal(t, config.UnknownSource, meta.Source.Type)
}

func TestConfig_Stat_NestedKey(t *testing.T) {
	t.Parallel()

	data := map[string]any{
		"a": map[string]any{
			"b": map[string]any{
				"c": "value",
			},
		},
	}
	col := collectors.NewMap(data).WithName("nested")
	builder := config.NewBuilder()

	builder = builder.AddCollector(col)

	cfg, errs := builder.Build(t.Context())
	require.Empty(t, errs)

	meta, ok := cfg.Stat(config.NewKeyPath("a/b/c"))
	require.True(t, ok)
	assert.Equal(t, "nested", meta.Source.Name)
}

func TestConfig_Slice_Root(t *testing.T) {
	t.Parallel()

	data := map[string]any{
		"key": "value",
	}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.AddCollector(col)

	cfg, errs := builder.Build(t.Context())
	require.Empty(t, errs)

	sliced, err := cfg.Slice(config.NewKeyPath(""))
	require.NoError(t, err)

	var val string

	_, err = sliced.Get(config.NewKeyPath("key"), &val)
	require.NoError(t, err)
	assert.Equal(t, "value", val)
}

func TestConfig_Slice_ValidPath(t *testing.T) {
	t.Parallel()

	data := map[string]any{
		"parent": map[string]any{
			"child": map[string]any{
				"grandchild": 42,
			},
		},
	}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.AddCollector(col)

	cfg, errs := builder.Build(t.Context())
	require.Empty(t, errs)

	sliced, err := cfg.Slice(config.NewKeyPath("parent/child"))
	require.NoError(t, err)

	var val int

	_, err = sliced.Get(config.NewKeyPath("grandchild"), &val)
	require.NoError(t, err)
	assert.Equal(t, 42, val)
}

func TestConfig_Slice_NonExistentPath(t *testing.T) {
	t.Parallel()

	data := map[string]any{
		"foo": "bar",
	}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.AddCollector(col)

	cfg, errs := builder.Build(t.Context())
	require.Empty(t, errs)

	_, err := cfg.Slice(config.NewKeyPath("nonexistent"))
	assert.Error(t, err)
}

func TestConfig_Walk_RootAllValues(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	data := map[string]any{
		"a": 1,
		"b": 2,
		"c": map[string]any{
			"d": 3,
		},
	}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.AddCollector(col)

	cfg, errs := builder.Build(t.Context())
	require.Empty(t, errs)

	ch, err := cfg.Walk(ctx, config.NewKeyPath(""), -1)
	require.NoError(t, err)

	count := 0
	for v := range ch {
		count++

		var dest any

		err := v.Get(&dest)
		require.NoError(t, err)
	}

	assert.Equal(t, 3, count)
}

func TestConfig_Walk_WithDepthLimit(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	data := map[string]any{
		"a": map[string]any{
			"b": map[string]any{
				"c": 1,
			},
		},
	}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.AddCollector(col)

	cfg, errs := builder.Build(t.Context())
	require.Empty(t, errs)

	ch, err := cfg.Walk(ctx, config.NewKeyPath(""), 1)
	require.NoError(t, err)

	count := 0
	for range ch {
		count++
	}

	assert.Equal(t, 0, count)
}

func TestConfig_Walk_FromSubPath(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	data := map[string]any{
		"parent": map[string]any{
			"child1": 1,
			"child2": 2,
		},
	}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.AddCollector(col)

	cfg, errs := builder.Build(t.Context())
	require.Empty(t, errs)

	ch, err := cfg.Walk(ctx, config.NewKeyPath("parent"), -1)
	require.NoError(t, err)

	count := 0
	for v := range ch {
		count++

		var dest any

		err := v.Get(&dest)
		require.NoError(t, err)
	}

	assert.Equal(t, 2, count)
}

func TestConfig_String_EmptyConfig(t *testing.T) {
	t.Parallel()

	data := map[string]any{}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.AddCollector(col)

	cfg, errs := builder.Build(t.Context())
	require.Empty(t, errs)

	assert.Empty(t, cfg.String())
}

func TestConfig_MarshalYAML_EmptyConfig(t *testing.T) {
	t.Parallel()

	data := map[string]any{}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.AddCollector(col)

	cfg, errs := builder.Build(t.Context())
	require.Empty(t, errs)

	out, err := cfg.MarshalYAML()
	require.NoError(t, err)
	assert.Empty(t, out)
}

func TestMutableConfig_Set(t *testing.T) {
	t.Parallel()

	data := map[string]any{"key": "original"}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.AddCollector(col)

	cfg, errs := builder.BuildMutable(t.Context())
	require.Empty(t, errs)

	err := cfg.Set(config.NewKeyPath("key"), "updated")
	require.NoError(t, err)

	var val string

	meta, err := cfg.Get(config.NewKeyPath("key"), &val)
	require.NoError(t, err)
	assert.Equal(t, "updated", val)
	assert.Equal(t, "modified", meta.Source.Name)
	assert.Equal(t, "1", string(meta.Revision))
}

func TestMutableConfig_Set_RevisionIncrement(t *testing.T) {
	t.Parallel()

	data := map[string]any{"key": "v1"}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.AddCollector(col)

	cfg, errs := builder.BuildMutable(t.Context())
	require.Empty(t, errs)

	err := cfg.Set(config.NewKeyPath("key"), "v2")
	require.NoError(t, err)

	meta, _ := cfg.Stat(config.NewKeyPath("key"))
	assert.Equal(t, "1", string(meta.Revision))

	err = cfg.Set(config.NewKeyPath("key"), "v3")
	require.NoError(t, err)

	meta, _ = cfg.Stat(config.NewKeyPath("key"))
	assert.Equal(t, "2", string(meta.Revision))
}

func TestMutableConfig_Set_NewKey(t *testing.T) {
	t.Parallel()

	data := map[string]any{}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.AddCollector(col)

	cfg, errs := builder.BuildMutable(t.Context())
	require.Empty(t, errs)

	err := cfg.Set(config.NewKeyPath("newkey"), "value")
	require.NoError(t, err)

	var val string

	_, err = cfg.Get(config.NewKeyPath("newkey"), &val)
	require.NoError(t, err)
	assert.Equal(t, "value", val)
}

func TestMutableConfig_Set_ValidationFailure_Reverts(t *testing.T) {
	t.Parallel()

	v := &valueChangeValidator{}
	data := map[string]any{"key": "original"}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.WithValidator(v)
	builder = builder.AddCollector(col)

	cfg, errs := builder.BuildMutable(t.Context())
	require.Empty(t, errs)

	err := cfg.Set(config.NewKeyPath("key"), "bad")
	require.Error(t, err)

	// Value should be reverted to original.
	var val string

	_, err = cfg.Get(config.NewKeyPath("key"), &val)
	require.NoError(t, err)
	assert.Equal(t, "original", val)
}

func TestMutableConfig_Set_ValidationFailure_RevertsNewPath(t *testing.T) {
	t.Parallel()

	v := &maxKeysValidator{maxKeys: 1}
	data := map[string]any{"key": "value"}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.WithValidator(v)
	builder = builder.AddCollector(col)

	cfg, errs := builder.BuildMutable(t.Context())
	require.Empty(t, errs)

	err := cfg.Set(config.NewKeyPath("secondkey"), "value")
	require.Error(t, err)

	// New key should not exist after revert.
	_, ok := cfg.Lookup(config.NewKeyPath("secondkey"))
	assert.False(t, ok)
}

func TestMutableConfig_Merge(t *testing.T) {
	t.Parallel()

	data := map[string]any{"key": "value"}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.AddCollector(col)

	cfg, errs := builder.BuildMutable(t.Context())
	require.Empty(t, errs)

	err := cfg.Merge(&cfg.Config)
	require.NoError(t, err)
}

func TestMutableConfig_Merge_Metadata(t *testing.T) {
	t.Parallel()

	data := map[string]any{"key": "value"}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.AddCollector(col)

	cfg, errs := builder.BuildMutable(t.Context())
	require.Empty(t, errs)

	otherData := map[string]any{"key": "newval", "extra": "added"}
	col2 := collectors.NewMap(otherData)
	builder2 := config.NewBuilder()

	builder2 = builder2.AddCollector(col2)

	otherCfg, errs2 := builder2.Build(t.Context())
	require.Empty(t, errs2)

	err := cfg.Merge(&otherCfg)
	require.NoError(t, err)

	meta, ok := cfg.Stat(config.NewKeyPath("key"))
	assert.True(t, ok)
	assert.Equal(t, "modified", meta.Source.Name)

	meta, ok = cfg.Stat(config.NewKeyPath("extra"))
	assert.True(t, ok)
	assert.Equal(t, "modified", meta.Source.Name)
}

func TestMutableConfig_Merge_ValidationFailure_Reverts(t *testing.T) {
	t.Parallel()

	v := &maxKeysValidator{maxKeys: 1}
	data := map[string]any{"key": "value"}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.WithValidator(v)
	builder = builder.AddCollector(col)

	cfg, errs := builder.BuildMutable(t.Context())
	require.Empty(t, errs)

	otherData := map[string]any{"newkey": "newvalue"}
	col2 := collectors.NewMap(otherData)
	builder2 := config.NewBuilder()

	builder2 = builder2.AddCollector(col2)

	otherCfg, errs2 := builder2.Build(t.Context())
	require.Empty(t, errs2)

	err := cfg.Merge(&otherCfg)
	require.Error(t, err)

	// newkey should not exist after revert.
	_, ok := cfg.Lookup(config.NewKeyPath("newkey"))
	assert.False(t, ok)
}

func TestMutableConfig_Update(t *testing.T) {
	t.Parallel()

	data := map[string]any{"key": "value"}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.AddCollector(col)

	cfg, errs := builder.BuildMutable(t.Context())
	require.Empty(t, errs)

	err := cfg.Update(&cfg.Config)
	require.NoError(t, err)
}

func TestMutableConfig_Update_Metadata(t *testing.T) {
	t.Parallel()

	data := map[string]any{"key": "value"}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.AddCollector(col)

	cfg, errs := builder.BuildMutable(t.Context())
	require.Empty(t, errs)

	otherData := map[string]any{"key": "updated"}
	col2 := collectors.NewMap(otherData)
	builder2 := config.NewBuilder()

	builder2 = builder2.AddCollector(col2)

	otherCfg, errs2 := builder2.Build(t.Context())
	require.Empty(t, errs2)

	err := cfg.Update(&otherCfg)
	require.NoError(t, err)

	meta, ok := cfg.Stat(config.NewKeyPath("key"))
	assert.True(t, ok)
	assert.Equal(t, "modified", meta.Source.Name)
}

func TestMutableConfig_Update_ValidationFailure_Reverts(t *testing.T) {
	t.Parallel()

	v := &valueChangeValidator{}
	data := map[string]any{"key": "original"}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.WithValidator(v)
	builder = builder.AddCollector(col)

	cfg, errs := builder.BuildMutable(t.Context())
	require.Empty(t, errs)

	otherData := map[string]any{"key": "newvalue"}
	col2 := collectors.NewMap(otherData)
	builder2 := config.NewBuilder()

	builder2 = builder2.AddCollector(col2)

	otherCfg, errs2 := builder2.Build(t.Context())
	require.Empty(t, errs2)

	err := cfg.Update(&otherCfg)
	require.Error(t, err)

	// Value should be reverted to original.
	var val string

	_, err = cfg.Get(config.NewKeyPath("key"), &val)
	require.NoError(t, err)
	assert.Equal(t, "original", val)
}

func TestMutableConfig_Delete_ExistingKey(t *testing.T) {
	t.Parallel()

	data := map[string]any{"key": "value", "other": "data"}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.AddCollector(col)

	cfg, errs := builder.BuildMutable(t.Context())
	require.Empty(t, errs)

	deleted := cfg.Delete(config.NewKeyPath("key"))
	assert.True(t, deleted)

	_, ok := cfg.Lookup(config.NewKeyPath("key"))
	assert.False(t, ok)

	// Other key should still exist.
	_, ok = cfg.Lookup(config.NewKeyPath("other"))
	assert.True(t, ok)
}

func TestMutableConfig_Delete_MissingKey(t *testing.T) {
	t.Parallel()

	data := map[string]any{"key": "value"}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.AddCollector(col)

	cfg, errs := builder.BuildMutable(t.Context())
	require.Empty(t, errs)

	deleted := cfg.Delete(config.NewKeyPath("nonexistent"))
	assert.False(t, deleted)
}

func TestMutableConfig_Delete_EmptyPath(t *testing.T) {
	t.Parallel()

	data := map[string]any{"key": "value"}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.AddCollector(col)

	cfg, errs := builder.BuildMutable(t.Context())
	require.Empty(t, errs)

	deleted := cfg.Delete(config.NewKeyPath(""))
	assert.False(t, deleted)
}

func TestMutableConfig_Delete_NestedKey(t *testing.T) {
	t.Parallel()

	data := map[string]any{
		"server": map[string]any{
			"host": "localhost",
			"port": 8080,
		},
	}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.AddCollector(col)

	cfg, errs := builder.BuildMutable(t.Context())
	require.Empty(t, errs)

	deleted := cfg.Delete(config.NewKeyPath("server/host"))
	assert.True(t, deleted)

	_, ok := cfg.Lookup(config.NewKeyPath("server/host"))
	assert.False(t, ok)

	// server/port should still exist.
	var port int

	_, err := cfg.Get(config.NewKeyPath("server/port"), &port)
	require.NoError(t, err)
	assert.Equal(t, 8080, port)
}

func TestMutableConfig_Delete_ValidationFailure_Reverts(t *testing.T) {
	t.Parallel()

	// minKeysValidator requires at least 2 keys. Start with 2 keys (build passes).
	// Deleting one would drop to 1 key, failing validation, so tree should be restored.
	v := &minKeysValidator{minKeys: 2}
	data := map[string]any{"key": "value", "other": "data"}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.WithValidator(v)
	builder = builder.AddCollector(col)

	cfg, errs := builder.BuildMutable(t.Context())
	require.Empty(t, errs)

	deleted := cfg.Delete(config.NewKeyPath("key"))
	assert.False(t, deleted)

	// Key should still exist after revert.
	_, ok := cfg.Lookup(config.NewKeyPath("key"))
	assert.True(t, ok)
}

func TestMutableConfig_Delete_Cascade(t *testing.T) {
	t.Parallel()

	data := map[string]any{
		"server": map[string]any{
			"host": "localhost",
			"port": 8080,
			"tls": map[string]any{
				"cert": "a.pem",
				"key":  "a.key",
			},
		},
	}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.AddCollector(col)

	cfg, errs := builder.BuildMutable(t.Context())
	require.Empty(t, errs)

	// Deleting the "server" map should remove all descendants.
	deleted := cfg.Delete(config.NewKeyPath("server"))
	assert.True(t, deleted)

	gone := []string{
		"server",
		"server/host",
		"server/port",
		"server/tls",
		"server/tls/cert",
		"server/tls/key",
	}
	for _, path := range gone {
		_, ok := cfg.Lookup(config.NewKeyPath(path))
		assert.Falsef(t, ok, "expected %q to be gone", path)
	}
}

func TestMutableConfig_Delete_CleanupEmptyParents(t *testing.T) {
	t.Parallel()

	data := map[string]any{
		"groups": map[string]any{
			"g1": map[string]any{
				"replicasets": map[string]any{
					"r1": map[string]any{
						"instances": map[string]any{
							"i1": map[string]any{"role": "leader"},
						},
					},
				},
			},
			"g2": map[string]any{"keep": "me"},
		},
	}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.AddCollector(col)

	cfg, errs := builder.BuildMutable(t.Context())
	require.Empty(t, errs)

	// Deleting the only instance under g1 should collapse all empty
	// ancestors up to and including "groups/g1", but stop at "groups"
	// because g2 keeps it non-empty.
	deleted := cfg.Delete(config.NewKeyPath("groups/g1/replicasets/r1/instances/i1"))
	assert.True(t, deleted)

	for _, path := range []string{
		"groups/g1/replicasets/r1/instances/i1",
		"groups/g1/replicasets/r1/instances",
		"groups/g1/replicasets/r1",
		"groups/g1/replicasets",
		"groups/g1",
	} {
		_, ok := cfg.Lookup(config.NewKeyPath(path))
		assert.Falsef(t, ok, "expected %q to be cleaned up", path)
	}

	// Sibling and the first non-empty ancestor must survive.
	_, ok := cfg.Lookup(config.NewKeyPath("groups"))
	assert.True(t, ok, "groups should remain because g2 still exists under it")

	var keep string

	_, err := cfg.Get(config.NewKeyPath("groups/g2/keep"), &keep)
	require.NoError(t, err)
	assert.Equal(t, "me", keep)
}

func TestMutableConfig_Delete_Idempotent(t *testing.T) {
	t.Parallel()

	data := map[string]any{"a": map[string]any{"b": "c"}}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.AddCollector(col)

	cfg, errs := builder.BuildMutable(t.Context())
	require.Empty(t, errs)

	// First delete succeeds and cascades.
	assert.True(t, cfg.Delete(config.NewKeyPath("a/b")))

	// Re-delete is a no-op, returns false, does not panic.
	assert.False(t, cfg.Delete(config.NewKeyPath("a/b")))
	assert.False(t, cfg.Delete(config.NewKeyPath("a")))
	assert.False(t, cfg.Delete(config.NewKeyPath("never-existed")))
}

func TestMutableConfig_Effective_AfterMutation(t *testing.T) {
	t.Parallel()

	build := func(t *testing.T) *config.MutableConfig {
		t.Helper()

		builder := config.NewBuilder()

		builder = builder.AddCollector(collectors.NewMap(map[string]any{
			"replication": map[string]any{"failover": "manual"},
			"groups": map[string]any{
				"storages": map[string]any{
					"replicasets": map[string]any{
						"s-001": map[string]any{
							"instances": map[string]any{
								"s-001-a": map[string]any{},
							},
						},
					},
				},
			},
		}).WithName("test"))

		builder = builder.WithInheritance(
			config.Levels(config.Global, "groups", "replicasets", "instances"),
		)

		cfg, errs := builder.BuildMutable(t.Context())
		require.Empty(t, errs)

		return &cfg
	}

	leafPath := config.NewKeyPath("groups/storages/replicasets/s-001/instances/s-001-a")

	t.Run("Set on parent group propagates to Effective", func(t *testing.T) {
		t.Parallel()

		cfg := build(t)

		// Override at the global level — must be visible through Effective on the leaf.
		require.NoError(t, cfg.Set(config.NewKeyPath("replication/failover"), "election"))

		eff, err := cfg.Effective(leafPath)
		require.NoError(t, err)

		var failover string

		_, err = eff.Get(config.NewKeyPath("replication/failover"), &failover)
		require.NoError(t, err)
		assert.Equal(t, "election", failover)
	})

	t.Run("Merge propagates to Effective", func(t *testing.T) {
		t.Parallel()

		cfg := build(t)

		patchBuilder := config.NewBuilder()

		patchBuilder = patchBuilder.AddCollector(collectors.NewMap(map[string]any{
			"replication": map[string]any{"failover": "supervised"},
		}))

		patch, errs := patchBuilder.Build(t.Context())
		require.Empty(t, errs)
		require.NoError(t, cfg.Merge(&patch))

		eff, err := cfg.Effective(leafPath)
		require.NoError(t, err)

		var failover string

		_, err = eff.Get(config.NewKeyPath("replication/failover"), &failover)
		require.NoError(t, err)
		assert.Equal(t, "supervised", failover)
	})

	t.Run("Delete propagates to Effective", func(t *testing.T) {
		t.Parallel()

		cfg := build(t)

		require.True(t, cfg.Delete(config.NewKeyPath("replication/failover")))

		eff, err := cfg.Effective(leafPath)
		require.NoError(t, err)

		_, ok := eff.Lookup(config.NewKeyPath("replication/failover"))
		assert.False(t, ok, "Effective should not see a key removed via Delete")
	})
}

func TestMutableConfig_Snapshot_Isolation(t *testing.T) {
	t.Parallel()

	data := map[string]any{"key": "original"}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.AddCollector(col)

	cfg, errs := builder.BuildMutable(t.Context())
	require.Empty(t, errs)

	snap := cfg.Snapshot()

	// Mutate the live config after taking the snapshot.
	require.NoError(t, cfg.Set(config.NewKeyPath("key"), "updated"))

	// Snapshot keeps the original value.
	var snapVal string

	_, err := snap.Get(config.NewKeyPath("key"), &snapVal)
	require.NoError(t, err)
	assert.Equal(t, "original", snapVal)

	// Live config sees the new value.
	var liveVal string

	_, err = cfg.Get(config.NewKeyPath("key"), &liveVal)
	require.NoError(t, err)
	assert.Equal(t, "updated", liveVal)
}

func TestMutableConfig_Snapshot_KeyAddedAfterSnapshotInvisible(t *testing.T) {
	t.Parallel()

	data := map[string]any{"existing": "value"}
	col := collectors.NewMap(data)
	builder := config.NewBuilder()

	builder = builder.AddCollector(col)

	cfg, errs := builder.BuildMutable(t.Context())
	require.Empty(t, errs)

	snap := cfg.Snapshot()

	require.NoError(t, cfg.Set(config.NewKeyPath("added"), "later"))

	_, ok := snap.Lookup(config.NewKeyPath("added"))
	assert.False(t, ok, "snapshot should not observe keys added after Snapshot()")
}
