package config_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tarantool/go-config"
	"github.com/tarantool/go-config/collectors"
	"github.com/tarantool/go-config/meta"
)

// ---------------------------------------------------------------------------
// Cross-loader MergeReplace map-recursion through Effective
// ---------------------------------------------------------------------------.

// TestLayered_CrossLoader_NonConflictingSubkeysCoexist is the headline
// invariant of accumulateLayerResult: a higher-priority loader that only sets
// one sub-key of a map must not wipe out sibling sub-keys contributed by a
// lower-priority loader. (file loader sets replication/failover, env loader
// sets replication/timeout — both must survive in the effective view.)
func TestLayered_CrossLoader_NonConflictingSubkeysCoexist(t *testing.T) {
	t.Parallel()

	fileLoader := collectors.NewMap(map[string]any{
		"replication": map[string]any{"failover": "election"},
		"groups": map[string]any{
			"storages": map[string]any{
				"replicasets": map[string]any{
					"s-001": map[string]any{
						"instances": map[string]any{"s-001-a": map[string]any{}},
					},
				},
			},
		},
	}).WithName("file").WithSourceType(config.FileSource)

	envLoader := collectors.NewMap(map[string]any{
		"replication": map[string]any{"timeout": 30},
	}).WithName("env").WithSourceType(config.EnvSource)

	builder := config.NewBuilder()

	builder = builder.AddCollector(fileLoader) // lower priority.
	builder = builder.AddCollector(envLoader)  // higher priority.
	builder = builder.WithInheritance(
		config.Levels(config.Global, "groups", "replicasets", "instances"),
	)

	cfg, errs := builder.Build(t.Context())
	require.Empty(t, errs)

	eff, err := cfg.Effective(
		config.NewKeyPath("groups/storages/replicasets/s-001/instances/s-001-a"))
	require.NoError(t, err)

	var failover string

	_, err = eff.Get(config.NewKeyPath("replication/failover"), &failover)
	require.NoError(t, err)
	assert.Equal(t, "election", failover, "lower-priority sub-key must survive")

	var timeout int

	_, err = eff.Get(config.NewKeyPath("replication/timeout"), &timeout)
	require.NoError(t, err)
	assert.Equal(t, 30, timeout, "higher-priority sub-key must be present")
}

// TestLayered_CrossLoader_ConflictingSubkeyHigherPriorityWins verifies that
// when both loaders set the same sub-key, the higher-priority loader wins while
// non-conflicting sub-keys from the lower-priority loader still coexist.
func TestLayered_CrossLoader_ConflictingSubkeyHigherPriorityWins(t *testing.T) {
	t.Parallel()

	fileLoader := collectors.NewMap(map[string]any{
		"replication": map[string]any{"failover": "manual", "timeout": 10},
		"groups": map[string]any{
			"storages": map[string]any{
				"replicasets": map[string]any{
					"s-001": map[string]any{
						"instances": map[string]any{"s-001-a": map[string]any{}},
					},
				},
			},
		},
	}).WithName("file").WithSourceType(config.FileSource)

	envLoader := collectors.NewMap(map[string]any{
		"replication": map[string]any{"failover": "election"},
	}).WithName("env").WithSourceType(config.EnvSource)

	builder := config.NewBuilder()

	builder = builder.AddCollector(fileLoader)
	builder = builder.AddCollector(envLoader)
	builder = builder.WithInheritance(
		config.Levels(config.Global, "groups", "replicasets", "instances"),
	)

	cfg, errs := builder.Build(t.Context())
	require.Empty(t, errs)

	eff, err := cfg.Effective(
		config.NewKeyPath("groups/storages/replicasets/s-001/instances/s-001-a"))
	require.NoError(t, err)

	var failover string

	fMeta, err := eff.Get(config.NewKeyPath("replication/failover"), &failover)
	require.NoError(t, err)
	assert.Equal(t, "election", failover)
	assert.Equal(t, "env", fMeta.Source.Name)

	var timeout int

	tMeta, err := eff.Get(config.NewKeyPath("replication/timeout"), &timeout)
	require.NoError(t, err)
	assert.Equal(t, 10, timeout)
	assert.Equal(t, "file", tMeta.Source.Name)
}

// TestLayered_CrossLoader_TypeMismatchHigherPriorityReplaces verifies that when
// a higher-priority loader provides a value of a different shape (scalar vs
// map), the higher-priority loader replaces the lower-priority contribution
// wholesale (no attempt to merge incompatible nodes).
func TestLayered_CrossLoader_TypeMismatchHigherPriorityReplaces(t *testing.T) {
	t.Parallel()

	fileLoader := collectors.NewMap(map[string]any{
		"thing": map[string]any{"a": 1, "b": 2},
		"groups": map[string]any{
			"storages": map[string]any{
				"replicasets": map[string]any{
					"s-001": map[string]any{
						"instances": map[string]any{"s-001-a": map[string]any{}},
					},
				},
			},
		},
	}).WithName("file").WithSourceType(config.FileSource)

	envLoader := collectors.NewMap(map[string]any{
		"thing": "scalar",
	}).WithName("env").WithSourceType(config.EnvSource)

	builder := config.NewBuilder()

	builder = builder.AddCollector(fileLoader)
	builder = builder.AddCollector(envLoader)
	builder = builder.WithInheritance(
		config.Levels(config.Global, "groups", "replicasets", "instances"),
	)

	cfg, errs := builder.Build(t.Context())
	require.Empty(t, errs)

	eff, err := cfg.Effective(
		config.NewKeyPath("groups/storages/replicasets/s-001/instances/s-001-a"))
	require.NoError(t, err)

	var thing string

	_, err = eff.Get(config.NewKeyPath("thing"), &thing)
	require.NoError(t, err)
	assert.Equal(t, "scalar", thing)

	// The lower-priority map sub-keys must be gone — they were replaced, not merged.
	_, err = eff.Get(config.NewKeyPath("thing/a"), new(int))
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// Tombstone suppression at an intermediate scope level
// ---------------------------------------------------------------------------.

// TestMutableConfig_Layered_Delete_IntermediateScope_FallsBackToGlobal deletes
// a key that was contributed at a group-level scope. The tombstone must be
// resolved against the group scope prefix (not the global one), suppressing
// only that level's contribution, so the global-scope value shines through.
func TestMutableConfig_Layered_Delete_IntermediateScope_FallsBackToGlobal(t *testing.T) {
	t.Parallel()

	builder := config.NewBuilder()

	builder = builder.AddCollector(collectors.NewMap(map[string]any{
		"replication": map[string]any{"failover": "manual"}, // global scope.
		"groups": map[string]any{
			"storages": map[string]any{
				"replication": map[string]any{"failover": "election"}, // group scope.
				"replicasets": map[string]any{
					"s-001": map[string]any{
						"instances": map[string]any{"s-001-a": map[string]any{}},
					},
				},
			},
		},
	}).WithName("file"))
	builder = builder.WithInheritance(
		config.Levels(config.Global, "groups", "replicasets", "instances"),
	)

	cfg, errs := builder.BuildMutable(t.Context())
	require.Empty(t, errs)

	leafPath := config.NewKeyPath("groups/storages/replicasets/s-001/instances/s-001-a")

	// Baseline: group-scope value wins over global-scope value within the loader.
	eff0, err := cfg.Effective(leafPath)
	require.NoError(t, err)

	var base string

	_, err = eff0.Get(config.NewKeyPath("replication/failover"), &base)
	require.NoError(t, err)
	assert.Equal(t, "election", base)

	// Delete the group-scoped contribution.
	require.True(t, cfg.Delete(config.NewKeyPath("groups/storages/replication/failover")))

	eff, err := cfg.Effective(leafPath)
	require.NoError(t, err)

	var failover string

	_, err = eff.Get(config.NewKeyPath("replication/failover"), &failover)
	require.NoError(t, err)
	assert.Equal(t, "manual", failover,
		"group-scope contribution suppressed; global-scope value must shine through")
}

// ---------------------------------------------------------------------------
// MutableConfig.Merge / Update in layered mode
// ---------------------------------------------------------------------------.

func mergeSourceConfig(t *testing.T, data map[string]any) config.Config {
	t.Helper()

	b := config.NewBuilder()

	b = b.AddCollector(collectors.NewMap(data))

	cfg, errs := b.Build(t.Context())
	require.Empty(t, errs)

	return cfg
}

func TestMutableConfig_Layered_Merge_EffectiveReflectsModified(t *testing.T) {
	t.Parallel()

	cfg := buildLayeredMutable(t)
	leafPath := config.NewKeyPath("groups/storages/replicasets/s-001/instances/s-001-a")

	other := mergeSourceConfig(t, map[string]any{
		"replication": map[string]any{"failover": "supervised"},
	})
	require.NoError(t, cfg.Merge(&other))

	eff, err := cfg.Effective(leafPath)
	require.NoError(t, err)

	var failover string

	m, err := eff.Get(config.NewKeyPath("replication/failover"), &failover)
	require.NoError(t, err)
	assert.Equal(t, "supervised", failover, "merged value must win over every loader")
	assert.Equal(t, meta.ModifiedSourceName, m.Source.Name)
}

func TestMutableConfig_Layered_Update_EffectiveReflectsModified(t *testing.T) {
	t.Parallel()

	cfg := buildLayeredMutable(t)
	leafPath := config.NewKeyPath("groups/storages/replicasets/s-001/instances/s-001-a")

	// Update only touches keys that already exist in the root. The merged root
	// already carries replication/failover from the env loader.
	other := mergeSourceConfig(t, map[string]any{
		"replication": map[string]any{"failover": "supervised"},
	})
	require.NoError(t, cfg.Update(&other))

	eff, err := cfg.Effective(leafPath)
	require.NoError(t, err)

	var failover string

	m, err := eff.Get(config.NewKeyPath("replication/failover"), &failover)
	require.NoError(t, err)
	assert.Equal(t, "supervised", failover)
	assert.Equal(t, meta.ModifiedSourceName, m.Source.Name)
}

// ---------------------------------------------------------------------------
// Ancestor-scope deletion -> ErrPathNotFound (entityTombstoned prefix match)
// ---------------------------------------------------------------------------.

func TestMutableConfig_Layered_Delete_AncestorScope_ErrPathNotFound(t *testing.T) {
	t.Parallel()

	cfg := buildLayeredMutable(t)
	leafPath := config.NewKeyPath("groups/storages/replicasets/s-001/instances/s-001-a")

	_, err := cfg.Effective(leafPath)
	require.NoError(t, err)

	// Delete a strict ancestor of the entity (the replicaset node).
	require.True(t, cfg.Delete(config.NewKeyPath("groups/storages/replicasets/s-001")))

	_, err = cfg.Effective(leafPath)
	require.Error(t, err)
	assert.ErrorIs(t, err, config.ErrPathNotFound,
		"deleting an ancestor scope must make the entity unresolvable, got: %v", err)
}

// ---------------------------------------------------------------------------
// Delete validation failure (layered): no tombstone, state stays consistent
// ---------------------------------------------------------------------------.

func TestMutableConfig_Layered_Delete_ValidationFailure_NoTombstone(t *testing.T) {
	t.Parallel()

	builder := config.NewBuilder()
	// Root will carry exactly 3 top-level keys; any delete drops below the minimum.
	builder = builder.WithValidator(&minKeysValidator{minKeys: 3})
	builder = builder.AddCollector(collectors.NewMap(map[string]any{
		"replication": map[string]any{"failover": "manual"},
		"alpha":       1,
		"groups": map[string]any{
			"storages": map[string]any{
				"replicasets": map[string]any{
					"s-001": map[string]any{
						"instances": map[string]any{"s-001-a": map[string]any{}},
					},
				},
			},
		},
	}).WithName("file"))
	builder = builder.WithInheritance(
		config.Levels(config.Global, "groups", "replicasets", "instances"),
	)

	cfg, errs := builder.BuildMutable(t.Context())
	require.Empty(t, errs)

	leafPath := config.NewKeyPath("groups/storages/replicasets/s-001/instances/s-001-a")

	// Deleting any top-level key would drop below the minimum and must be rejected.
	require.False(t, cfg.Delete(config.NewKeyPath("alpha")))
	require.False(t, cfg.Delete(config.NewKeyPath("replication/failover")))

	// The rejected deletes left no tombstone behind and no state change:
	// alpha is still resolvable and the entity still resolves.
	var alpha int

	_, err := cfg.Get(config.NewKeyPath("alpha"), &alpha)
	require.NoError(t, err)
	assert.Equal(t, 1, alpha)

	eff, err := cfg.Effective(leafPath)
	require.NoError(t, err)

	var failover string

	_, err = eff.Get(config.NewKeyPath("replication/failover"), &failover)
	require.NoError(t, err)
	assert.Equal(t, "manual", failover)
}

// ---------------------------------------------------------------------------
// WithNoInherit combined with multi-loader layering
// ---------------------------------------------------------------------------.

// TestLayered_NoInherit_MultiLoader verifies that WithNoInherit interacts
// correctly with cross-loader layering: a non-leaf-scope contribution of a
// no-inherit key is dropped even from a higher-priority loader, while a
// leaf-scope contribution from a lower-priority loader is kept.
func TestLayered_NoInherit_MultiLoader(t *testing.T) {
	t.Parallel()

	// Loader 1 (lower priority): leader set at the instance (leaf) scope.
	loader1 := collectors.NewMap(map[string]any{
		"groups": map[string]any{
			"storages": map[string]any{
				"replicasets": map[string]any{
					"s-001": map[string]any{
						"instances": map[string]any{
							"s-001-a": map[string]any{"leader": "from-instance"},
						},
					},
				},
			},
		},
	}).WithName("file").WithSourceType(config.FileSource)

	// Loader 2 (higher priority): leader set at the replicaset scope.
	loader2 := collectors.NewMap(map[string]any{
		"groups": map[string]any{
			"storages": map[string]any{
				"replicasets": map[string]any{
					"s-001": map[string]any{"leader": "from-replicaset"},
				},
			},
		},
	}).WithName("env").WithSourceType(config.EnvSource)

	builder := config.NewBuilder()

	builder = builder.AddCollector(loader1)
	builder = builder.AddCollector(loader2)
	builder = builder.WithInheritance(
		config.Levels(config.Global, "groups", "replicasets", "instances"),
		config.WithNoInherit("leader"),
	)

	cfg, errs := builder.Build(t.Context())
	require.Empty(t, errs)

	eff, err := cfg.Effective(
		config.NewKeyPath("groups/storages/replicasets/s-001/instances/s-001-a"))
	require.NoError(t, err)

	var leader string

	_, err = eff.Get(config.NewKeyPath("leader"), &leader)
	require.NoError(t, err)
	assert.Equal(t, "from-instance", leader,
		"non-leaf-scope no-inherit key from the higher-priority loader must be dropped; "+
			"leaf-scope contribution from the lower-priority loader survives")
}

// ---------------------------------------------------------------------------
// MultiCollector counts as a single layer
// ---------------------------------------------------------------------------.

// multiMapCollector wraps a base Map (so it satisfies config.Collector) and a
// fixed list of sub-collectors, implementing config.MultiCollector.
type multiMapCollector struct {
	*collectors.Map

	subs []config.Collector
}

func (m *multiMapCollector) Collectors(_ context.Context) ([]config.Collector, error) {
	return m.subs, nil
}

// TestLayered_MultiCollector_CountsAsOneLayer verifies that a MultiCollector is
// folded into one layer tree: a higher-priority top-level collector outranks the
// whole MultiCollector regardless of which sub-collector or scope a value came from.
func TestLayered_MultiCollector_CountsAsOneLayer(t *testing.T) {
	t.Parallel()

	sub1 := collectors.NewMap(map[string]any{
		"groups": map[string]any{
			"storages": map[string]any{
				"replicasets": map[string]any{
					"s-001": map[string]any{
						"instances": map[string]any{
							"s-001-a": map[string]any{"mode": "from-sub1-instance"},
						},
					},
				},
			},
		},
	}).WithName("sub1").WithSourceType(config.FileSource)

	sub2 := collectors.NewMap(map[string]any{
		"mode": "from-sub2-global",
	}).WithName("sub2").WithSourceType(config.FileSource)

	multi := &multiMapCollector{
		Map:  collectors.NewMap(nil).WithName("multi").WithSourceType(config.FileSource),
		subs: []config.Collector{sub1, sub2},
	}

	// Higher-priority top-level collector, global scope.
	top := collectors.NewMap(map[string]any{
		"mode": "from-top-global",
	}).WithName("top").WithSourceType(config.EnvSource)

	builder := config.NewBuilder()

	builder = builder.AddCollector(multi) // lower priority (one layer).
	builder = builder.AddCollector(top)   // higher priority.
	builder = builder.WithInheritance(
		config.Levels(config.Global, "groups", "replicasets", "instances"),
	)

	cfg, errs := builder.Build(t.Context())
	require.Empty(t, errs)

	eff, err := cfg.Effective(
		config.NewKeyPath("groups/storages/replicasets/s-001/instances/s-001-a"))
	require.NoError(t, err)

	var mode string

	m, err := eff.Get(config.NewKeyPath("mode"), &mode)
	require.NoError(t, err)
	assert.Equal(t, "from-top-global", mode,
		"higher-priority top-level collector must beat the whole MultiCollector layer")
	assert.Equal(t, "top", m.Source.Name)
}

// ---------------------------------------------------------------------------
// Cross-loader: arrays are opaque to map-merge (mergeTreeInto path)
// ---------------------------------------------------------------------------.

// TestLayered_CrossLoader_NestedArrayWholesaleReplace is the cross-loader
// twin of TestWithInheritance_CrossScope_NestedArrayShapes: arrays nested
// inside a deep-merged map must be replaced wholesale by the higher-priority
// loader, with no index-by-index merge that would leak orphan elements.
func TestLayered_CrossLoader_NestedArrayWholesaleReplace(t *testing.T) {
	t.Parallel()

	fileLoader := collectors.NewMap(map[string]any{
		"iproto": map[string]any{
			"listen": []any{
				map[string]any{"uri": "file-1"},
				map[string]any{"uri": "file-2"},
				map[string]any{"uri": "file-3"},
			},
		},
		"groups": map[string]any{
			"storages": map[string]any{
				"replicasets": map[string]any{
					"s-001": map[string]any{
						"instances": map[string]any{"s-001-a": map[string]any{}},
					},
				},
			},
		},
	}).WithName("file").WithSourceType(config.FileSource)

	envLoader := collectors.NewMap(map[string]any{
		"iproto": map[string]any{
			"listen": []any{
				map[string]any{"uri": "env-1"},
			},
		},
	}).WithName("env").WithSourceType(config.EnvSource)

	builder := config.NewBuilder()

	builder = builder.AddCollector(fileLoader) // lower priority.
	builder = builder.AddCollector(envLoader)  // higher priority.
	builder = builder.WithInheritance(
		config.Levels(config.Global, "groups", "replicasets", "instances"),
	)

	cfg, errs := builder.Build(t.Context())
	require.Empty(t, errs)

	eff, err := cfg.Effective(
		config.NewKeyPath("groups/storages/replicasets/s-001/instances/s-001-a"))
	require.NoError(t, err)

	var listen []map[string]string

	_, err = eff.Get(config.NewKeyPath("iproto/listen"), &listen)
	require.NoError(t, err)
	require.Len(t, listen, 1, "env loader's iproto.listen must replace file loader's wholesale")
	assert.Equal(t, "env-1", listen[0]["uri"])
}

// TestLayered_CrossLoader_MapSiblingsPreservedWithArrayReplace verifies the
// combined invariant for cross-loader merging: sibling map sub-keys still
// deep-merge across loaders while sibling arrays still wholesale-replace.
func TestLayered_CrossLoader_MapSiblingsPreservedWithArrayReplace(t *testing.T) {
	t.Parallel()

	fileLoader := collectors.NewMap(map[string]any{
		"iproto": map[string]any{
			"advertise": map[string]any{
				"peer":     map[string]any{"login": "replicator"},
				"sharding": map[string]any{"login": "storage"},
			},
			"listen": []any{
				map[string]any{"uri": "file-1"},
				map[string]any{"uri": "file-2"},
			},
		},
		"groups": map[string]any{
			"storages": map[string]any{
				"replicasets": map[string]any{
					"s-001": map[string]any{
						"instances": map[string]any{"s-001-a": map[string]any{}},
					},
				},
			},
		},
	}).WithName("file").WithSourceType(config.FileSource)

	envLoader := collectors.NewMap(map[string]any{
		"iproto": map[string]any{
			"advertise": map[string]any{
				"peer": map[string]any{"login": "replicator-env"},
			},
			"listen": []any{
				map[string]any{"uri": "env-1"},
			},
		},
	}).WithName("env").WithSourceType(config.EnvSource)

	builder := config.NewBuilder()

	builder = builder.AddCollector(fileLoader)
	builder = builder.AddCollector(envLoader)
	builder = builder.WithInheritance(
		config.Levels(config.Global, "groups", "replicasets", "instances"),
	)

	cfg, errs := builder.Build(t.Context())
	require.Empty(t, errs)

	eff, err := cfg.Effective(
		config.NewKeyPath("groups/storages/replicasets/s-001/instances/s-001-a"))
	require.NoError(t, err)

	// Map merge: env wins on the conflicting leaf, sibling sub-key from
	// file loader survives.
	var peerLogin string

	_, err = eff.Get(config.NewKeyPath("iproto/advertise/peer/login"), &peerLogin)
	require.NoError(t, err)
	assert.Equal(t, "replicator-env", peerLogin)

	var shardingLogin string

	_, err = eff.Get(config.NewKeyPath("iproto/advertise/sharding/login"), &shardingLogin)
	require.NoError(t, err)
	assert.Equal(t, "storage", shardingLogin)

	// Array replace: env fully replaces file, no leaked file[1].
	var listen []map[string]string

	_, err = eff.Get(config.NewKeyPath("iproto/listen"), &listen)
	require.NoError(t, err)
	require.Len(t, listen, 1)
	assert.Equal(t, "env-1", listen[0]["uri"])
}

// TestLayered_CrossLoader_ArrayMapTypeMismatch exercises the type-boundary
// invariant across loaders: when one loader has an array and another has
// a map at the same path, the higher-priority loader wins wholesale.
func TestLayered_CrossLoader_ArrayMapTypeMismatch(t *testing.T) {
	t.Parallel()

	t.Run("ArrayLow_MapHigh", func(t *testing.T) {
		t.Parallel()

		fileLoader := collectors.NewMap(map[string]any{
			"thing": []any{"f1", "f2"},
			"groups": map[string]any{
				"storages": map[string]any{
					"replicasets": map[string]any{
						"s-001": map[string]any{
							"instances": map[string]any{"s-001-a": map[string]any{}},
						},
					},
				},
			},
		}).WithName("file").WithSourceType(config.FileSource)

		envLoader := collectors.NewMap(map[string]any{
			"thing": map[string]any{"key": "value"},
		}).WithName("env").WithSourceType(config.EnvSource)

		builder := config.NewBuilder()

		builder = builder.AddCollector(fileLoader)
		builder = builder.AddCollector(envLoader)
		builder = builder.WithInheritance(
			config.Levels(config.Global, "groups", "replicasets", "instances"),
		)

		cfg, errs := builder.Build(t.Context())
		require.Empty(t, errs)

		eff, err := cfg.Effective(
			config.NewKeyPath("groups/storages/replicasets/s-001/instances/s-001-a"))
		require.NoError(t, err)

		var got map[string]string

		_, err = eff.Get(config.NewKeyPath("thing"), &got)
		require.NoError(t, err)
		assert.Equal(t, map[string]string{"key": "value"}, got)
	})

	t.Run("MapLow_ArrayHigh", func(t *testing.T) {
		t.Parallel()

		fileLoader := collectors.NewMap(map[string]any{
			"thing": map[string]any{"a": 1, "b": 2},
			"groups": map[string]any{
				"storages": map[string]any{
					"replicasets": map[string]any{
						"s-001": map[string]any{
							"instances": map[string]any{"s-001-a": map[string]any{}},
						},
					},
				},
			},
		}).WithName("file").WithSourceType(config.FileSource)

		envLoader := collectors.NewMap(map[string]any{
			"thing": []any{"e1", "e2"},
		}).WithName("env").WithSourceType(config.EnvSource)

		builder := config.NewBuilder()

		builder = builder.AddCollector(fileLoader)
		builder = builder.AddCollector(envLoader)
		builder = builder.WithInheritance(
			config.Levels(config.Global, "groups", "replicasets", "instances"),
		)

		cfg, errs := builder.Build(t.Context())
		require.Empty(t, errs)

		eff, err := cfg.Effective(
			config.NewKeyPath("groups/storages/replicasets/s-001/instances/s-001-a"))
		require.NoError(t, err)

		var got []any

		_, err = eff.Get(config.NewKeyPath("thing"), &got)
		require.NoError(t, err)
		assert.Equal(t, []any{"e1", "e2"}, got)

		_, err = eff.Get(config.NewKeyPath("thing/a"), new(int))
		require.Error(t, err)
	})
}
