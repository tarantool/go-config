package config_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tarantool/go-config"
	"github.com/tarantool/go-config/collectors"
)

func TestWithInheritance_BasicInheritance(t *testing.T) {
	t.Parallel()

	builder := config.NewBuilder()

	builder = builder.AddCollector(collectors.NewMap(map[string]any{
		"credentials": map[string]any{
			"users": map[string]any{
				"replicator": map[string]any{
					"password": "secret",
					"roles":    []any{"replication"},
				},
			},
		},
		"iproto": map[string]any{
			"advertise": map[string]any{
				"peer": map[string]any{"login": "replicator"},
			},
		},
		"replication": map[string]any{"failover": "manual"},
		"groups": map[string]any{
			"storages": map[string]any{
				"sharding": map[string]any{"roles": []any{"storage"}},
				"credentials": map[string]any{
					"users": map[string]any{
						"monitor": map[string]any{
							"password": "m",
							"roles":    []any{"metrics"},
						},
					},
				},
				"replicasets": map[string]any{
					"s-001": map[string]any{
						"leader": "s-001-a",
						"instances": map[string]any{
							"s-001-a": map[string]any{
								"iproto": map[string]any{
									"listen": []any{map[string]any{"uri": "127.0.0.1:3301"}},
								},
							},
						},
					},
				},
			},
		},
	}).WithName("test").WithSourceType(config.FileSource))

	builder = builder.WithInheritance(
		config.Levels(config.Global, "groups", "replicasets", "instances"),
	)

	cfg, errs := builder.Build()
	require.Empty(t, errs)
	require.NotNil(t, cfg)

	instanceCfg, err := cfg.Effective(config.NewKeyPath("groups/storages/replicasets/s-001/instances/s-001-a"))
	require.NoError(t, err)

	var failover string

	_, err = instanceCfg.Get(config.NewKeyPath("replication/failover"), &failover)
	require.NoError(t, err)
	assert.Equal(t, "manual", failover)

	var roles []string

	_, err = instanceCfg.Get(config.NewKeyPath("sharding/roles"), &roles)
	require.NoError(t, err)
	assert.Equal(t, []string{"storage"}, roles)

	var leader string

	_, err = instanceCfg.Get(config.NewKeyPath("leader"), &leader)
	require.NoError(t, err)
	assert.Equal(t, "s-001-a", leader)

	var listen []map[string]string

	_, err = instanceCfg.Get(config.NewKeyPath("iproto/listen"), &listen)
	require.NoError(t, err)
	assert.Len(t, listen, 1)
	assert.Equal(t, "127.0.0.1:3301", listen[0]["uri"])
}

func TestWithInheritance_ChildOverridesParent(t *testing.T) {
	t.Parallel()

	builder := config.NewBuilder()

	builder = builder.AddCollector(collectors.NewMap(map[string]any{
		"iproto": map[string]any{
			"advertise": map[string]any{
				"peer": map[string]any{"login": "admin"},
			},
		},
		"groups": map[string]any{
			"storages": map[string]any{
				"iproto": map[string]any{
					"advertise": map[string]any{
						"peer": map[string]any{"login": "replicator"},
					},
				},
			},
		},
	}).WithName("test").WithSourceType(config.FileSource))

	builder = builder.WithInheritance(
		config.Levels(config.Global, "groups", "replicasets", "instances"),
	)

	cfg, errs := builder.Build()
	require.Empty(t, errs)

	instanceCfg, err := cfg.Effective(config.NewKeyPath("groups/storages/replicasets/s-001/instances/s-001-a"))
	require.NoError(t, err)

	var login string

	_, err = instanceCfg.Get(config.NewKeyPath("iproto/advertise/peer/login"), &login)
	require.NoError(t, err)
	assert.Equal(t, "replicator", login)
}

func TestWithInheritance_Defaults(t *testing.T) {
	t.Parallel()

	builder := config.NewBuilder()

	builder = builder.WithInheritance(
		config.Levels(config.Global, "groups", "replicasets", "instances"),
		config.WithDefaults(config.DefaultsType{
			"app": map[string]any{"file": "init.lua"},
		}),
	)

	cfg, errs := builder.Build()
	require.Empty(t, errs)

	instanceCfg, err := cfg.Effective(config.NewKeyPath("groups/storages/replicasets/s-001/instances/s-001-a"))
	require.NoError(t, err)

	var app map[string]any

	_, err = instanceCfg.Get(config.NewKeyPath("app"), &app)
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"file": "init.lua"}, app)
}

func TestWithInheritance_DefaultsOverriddenByGlobal(t *testing.T) {
	t.Parallel()

	builder := config.NewBuilder()

	builder = builder.AddCollector(collectors.NewMap(map[string]any{
		"app": map[string]any{"file": "main.lua"},
	}).WithName("test").WithSourceType(config.FileSource))

	builder = builder.WithInheritance(
		config.Levels(config.Global, "groups", "replicasets", "instances"),
		config.WithDefaults(config.DefaultsType{
			"app": map[string]any{"file": "init.lua"},
		}),
	)

	cfg, errs := builder.Build()
	require.Empty(t, errs)

	instanceCfg, err := cfg.Effective(config.NewKeyPath("groups/storages/replicasets/s-001/instances/s-001-a"))
	require.NoError(t, err)

	var file string

	_, err = instanceCfg.Get(config.NewKeyPath("app/file"), &file)
	require.NoError(t, err)
	assert.Equal(t, "main.lua", file)
}

func TestWithInheritance_EffectiveAll(t *testing.T) {
	t.Parallel()

	builder := config.NewBuilder()

	builder = builder.AddCollector(collectors.NewMap(map[string]any{
		"credentials": map[string]any{
			"users": map[string]any{
				"replicator": map[string]any{
					"password": "secret",
					"roles":    []any{"replication"},
				},
			},
		},
		"iproto": map[string]any{
			"advertise": map[string]any{
				"peer": map[string]any{"login": "replicator"},
			},
		},
		"replication": map[string]any{"failover": "manual"},
		"groups": map[string]any{
			"storages": map[string]any{
				"sharding": map[string]any{"roles": []any{"storage"}},
				"credentials": map[string]any{
					"users": map[string]any{
						"monitor": map[string]any{
							"password": "m",
							"roles":    []any{"metrics"},
						},
					},
				},
				"replicasets": map[string]any{
					"s-001": map[string]any{
						"leader": "s-001-a",
						"instances": map[string]any{
							"s-001-a": map[string]any{
								"iproto": map[string]any{
									"listen": []any{map[string]any{"uri": "127.0.0.1:3301"}},
								},
							},
						},
					},
				},
			},
		},
	}).WithName("test").WithSourceType(config.FileSource))

	builder = builder.WithInheritance(
		config.Levels(config.Global, "groups", "replicasets", "instances"),
	)

	cfg, errs := builder.Build()
	require.Empty(t, errs)
	require.NotNil(t, cfg)

	all, err := cfg.EffectiveAll()
	require.NoError(t, err)
	assert.Len(t, all, 1)

	instanceCfg, ok := all["groups/storages/replicasets/s-001/instances/s-001-a"]
	assert.True(t, ok)

	var failover string

	_, err = instanceCfg.Get(config.NewKeyPath("replication/failover"), &failover)
	require.NoError(t, err)
	assert.Equal(t, "manual", failover)

	var roles []string

	_, err = instanceCfg.Get(config.NewKeyPath("sharding/roles"), &roles)
	require.NoError(t, err)
	assert.Equal(t, []string{"storage"}, roles)

	var leader string

	_, err = instanceCfg.Get(config.NewKeyPath("leader"), &leader)
	require.NoError(t, err)
	assert.Equal(t, "s-001-a", leader)
}

func TestWithInheritance_NoInherit(t *testing.T) {
	t.Parallel()

	builder := config.NewBuilder()

	builder = builder.AddCollector(collectors.NewMap(map[string]any{
		"replication": map[string]any{"failover": "manual"},
		"groups": map[string]any{
			"storages": map[string]any{
				"replicasets": map[string]any{
					"s-001": map[string]any{
						"leader": "s-001-a",
						"instances": map[string]any{
							"s-001-a": map[string]any{
								"iproto": map[string]any{
									"listen": []any{map[string]any{"uri": "127.0.0.1:3301"}},
								},
							},
						},
					},
				},
			},
		},
	}).WithName("test").WithSourceType(config.FileSource))

	builder = builder.WithInheritance(
		config.Levels(config.Global, "groups", "replicasets", "instances"),
		config.WithNoInherit("leader"),
	)

	cfg, errs := builder.Build()
	require.Empty(t, errs)
	require.NotNil(t, cfg)

	instanceCfg, err := cfg.Effective(config.NewKeyPath("groups/storages/replicasets/s-001/instances/s-001-a"))
	require.NoError(t, err)

	// Leader should NOT be present in effective config (excluded from inheritance).
	var leader string

	_, err = instanceCfg.Get(config.NewKeyPath("leader"), &leader)
	require.Error(t, err)
	assert.Empty(t, leader)

	var failover string

	_, err = instanceCfg.Get(config.NewKeyPath("replication/failover"), &failover)
	require.NoError(t, err)
	assert.Equal(t, "manual", failover)
}

func TestWithInheritance_MergeAppend(t *testing.T) {
	t.Parallel()

	builder := config.NewBuilder()

	builder = builder.AddCollector(collectors.NewMap(map[string]any{
		"groups": map[string]any{
			"storages": map[string]any{
				"roles": []any{"storage"},
				"replicasets": map[string]any{
					"s-001": map[string]any{
						"roles": []any{"metrics"},
						"instances": map[string]any{
							"s-001-a": map[string]any{
								"iproto": map[string]any{
									"listen": []any{map[string]any{"uri": "127.0.0.1:3301"}},
								},
							},
						},
					},
				},
			},
		},
	}).WithName("test").WithSourceType(config.FileSource))

	builder = builder.WithInheritance(
		config.Levels(config.Global, "groups", "replicasets", "instances"),
		config.WithInheritMerge("roles", config.MergeAppend),
	)

	cfg, errs := builder.Build()
	require.Empty(t, errs)
	require.NotNil(t, cfg)

	instanceCfg, err := cfg.Effective(config.NewKeyPath("groups/storages/replicasets/s-001/instances/s-001-a"))
	require.NoError(t, err)

	var roles []string

	_, err = instanceCfg.Get(config.NewKeyPath("roles"), &roles)
	require.NoError(t, err)

	assert.Equal(t, []string{"storage", "metrics"}, roles)
}

func TestWithInheritance_MergeDeep(t *testing.T) {
	t.Parallel()

	builder := config.NewBuilder()

	builder = builder.AddCollector(collectors.NewMap(map[string]any{
		"credentials": map[string]any{
			"users": map[string]any{
				"admin": map[string]any{
					"password": "admin123",
					"roles":    []any{"admin"},
				},
			},
		},
		"groups": map[string]any{
			"storages": map[string]any{
				"credentials": map[string]any{
					"users": map[string]any{
						"monitor": map[string]any{
							"password": "monitor123",
							"roles":    []any{"metrics"},
						},
					},
				},
				"replicasets": map[string]any{
					"s-001": map[string]any{
						"instances": map[string]any{
							"s-001-a": map[string]any{
								"iproto": map[string]any{
									"listen": []any{map[string]any{"uri": "127.0.0.1:3301"}},
								},
							},
						},
					},
				},
			},
		},
	}).WithName("test").WithSourceType(config.FileSource))

	builder = builder.WithInheritance(
		config.Levels(config.Global, "groups", "replicasets", "instances"),
		config.WithInheritMerge("credentials", config.MergeDeep),
	)

	cfg, errs := builder.Build()
	require.Empty(t, errs)
	require.NotNil(t, cfg)

	instanceCfg, err := cfg.Effective(config.NewKeyPath("groups/storages/replicasets/s-001/instances/s-001-a"))
	require.NoError(t, err)

	var users map[string]any

	_, err = instanceCfg.Get(config.NewKeyPath("credentials/users"), &users)
	require.NoError(t, err)
	assert.Len(t, users, 2)

	var admin map[string]any

	_, err = instanceCfg.Get(config.NewKeyPath("credentials/users/admin"), &admin)
	require.NoError(t, err)
	assert.Equal(t, "admin123", admin["password"])

	var monitor map[string]any

	_, err = instanceCfg.Get(config.NewKeyPath("credentials/users/monitor"), &monitor)
	require.NoError(t, err)
	assert.Equal(t, "monitor123", monitor["password"])
}

func TestWithInheritance_NoInheritFrom(t *testing.T) {
	t.Parallel()

	builder := config.NewBuilder()

	builder = builder.AddCollector(collectors.NewMap(map[string]any{
		"snapshot": map[string]any{"dir": "/global/snapshots"},
		"groups": map[string]any{
			"storages": map[string]any{
				"snapshot": map[string]any{"dir": "/group/snapshots"},
				"replicasets": map[string]any{
					"s-001": map[string]any{
						"instances": map[string]any{
							"s-001-a": map[string]any{
								"iproto": map[string]any{
									"listen": []any{map[string]any{"uri": "127.0.0.1:3301"}},
								},
							},
						},
					},
				},
			},
		},
	}).WithName("test").WithSourceType(config.FileSource))

	builder = builder.WithInheritance(
		config.Levels(config.Global, "groups", "replicasets", "instances"),
		config.WithNoInheritFrom(config.Global, "snapshot.dir"),
	)

	cfg, errs := builder.Build()
	require.Empty(t, errs)
	require.NotNil(t, cfg)

	instanceCfg, err := cfg.Effective(config.NewKeyPath("groups/storages/replicasets/s-001/instances/s-001-a"))
	require.NoError(t, err)

	var dir string

	_, err = instanceCfg.Get(config.NewKeyPath("snapshot/dir"), &dir)
	require.NoError(t, err)
	assert.Equal(t, "/group/snapshots", dir)
}

func TestWithInheritance_MultipleHierarchies(t *testing.T) {
	t.Parallel()

	// Two independent hierarchies: groups/replicasets/instances AND buckets/objects.
	builder := config.NewBuilder()

	builder = builder.AddCollector(collectors.NewMap(map[string]any{
		"global_key": "global_value",
		"groups": map[string]any{
			"storages": map[string]any{
				"group_key": "group_value",
				"replicasets": map[string]any{
					"s-001": map[string]any{
						"replicaset_key": "replicaset_value",
						"instances": map[string]any{
							"s-001-a": map[string]any{
								"instance_key": "instance_value",
							},
						},
					},
				},
			},
		},
		"buckets": map[string]any{
			"data": map[string]any{
				"bucket_key": "bucket_value",
				"objects": map[string]any{
					"obj-001": map[string]any{
						"object_key": "object_value",
					},
				},
			},
		},
	}).WithName("test").WithSourceType(config.FileSource))

	builder = builder.WithInheritance(
		config.Levels(config.Global, "groups", "replicasets", "instances"),
	)
	builder = builder.WithInheritance(
		config.Levels(config.Global, "buckets", "objects"),
	)

	cfg, errs := builder.Build()
	require.Empty(t, errs)
	require.NotNil(t, cfg)

	instanceCfg, err := cfg.Effective(config.NewKeyPath("groups/storages/replicasets/s-001/instances/s-001-a"))
	require.NoError(t, err)

	var globalVal, groupVal, replicasetVal, instanceVal string

	_, err = instanceCfg.Get(config.NewKeyPath("global_key"), &globalVal)
	require.NoError(t, err)
	assert.Equal(t, "global_value", globalVal)

	_, err = instanceCfg.Get(config.NewKeyPath("group_key"), &groupVal)
	require.NoError(t, err)
	assert.Equal(t, "group_value", groupVal)

	_, err = instanceCfg.Get(config.NewKeyPath("replicaset_key"), &replicasetVal)
	require.NoError(t, err)
	assert.Equal(t, "replicaset_value", replicasetVal)

	_, err = instanceCfg.Get(config.NewKeyPath("instance_key"), &instanceVal)
	require.NoError(t, err)
	assert.Equal(t, "instance_value", instanceVal)

	objectCfg, err := cfg.Effective(config.NewKeyPath("buckets/data/objects/obj-001"))
	require.NoError(t, err)

	var bucketVal, objectVal string

	_, err = objectCfg.Get(config.NewKeyPath("bucket_key"), &bucketVal)
	require.NoError(t, err)
	assert.Equal(t, "bucket_value", bucketVal)

	_, err = objectCfg.Get(config.NewKeyPath("object_key"), &objectVal)
	require.NoError(t, err)
	assert.Equal(t, "object_value", objectVal)
}

func TestWithInheritance_InvalidPath(t *testing.T) {
	t.Parallel()

	builder := config.NewBuilder()

	builder = builder.AddCollector(collectors.NewMap(map[string]any{
		"groups": map[string]any{
			"storages": map[string]any{
				"replicasets": map[string]any{
					"s-001": map[string]any{
						"instances": map[string]any{
							"s-001-a": map[string]any{
								"key": "value",
							},
						},
					},
				},
			},
		},
	}).WithName("test").WithSourceType(config.FileSource))

	builder = builder.WithInheritance(
		config.Levels(config.Global, "groups", "replicasets", "instances"),
	)

	cfg, errs := builder.Build()
	require.Empty(t, errs)
	require.NotNil(t, cfg)

	_, err := cfg.Effective(config.NewKeyPath("groups/storages/buckets/s-001/instances/s-001-a"))
	require.Error(t, err, "expected error for invalid path")

	replicasetCfg, err := cfg.Effective(config.NewKeyPath("groups/storages/replicasets/s-001"))
	require.NoError(t, err)

	var key string

	_, err = replicasetCfg.Get(config.NewKeyPath("key"), &key)
	require.Error(t, err, "expected error for key not found at replicaset level")

	missingCfg, err := cfg.Effective(config.NewKeyPath("groups/storages/replicasets/s-001/instances/nonexistent"))
	require.NoError(t, err, "unexpected error for missing named entity")

	var instanceKey string

	_, err = missingCfg.Get(config.NewKeyPath("instance_key"), &instanceKey)
	require.Error(t, err, "instance_key should not be present for missing entity")
}

func TestWithInheritance_PartialHierarchy(t *testing.T) {
	t.Parallel()

	builder := config.NewBuilder()

	builder = builder.AddCollector(collectors.NewMap(map[string]any{
		"global_key": "global_value",
		"groups": map[string]any{
			"storages": map[string]any{
				"replicasets": map[string]any{
					"s-001": map[string]any{
						"replicaset_key": "replicaset_value",
						"instances": map[string]any{
							"s-001-a": map[string]any{
								"instance_key": "instance_value",
							},
						},
					},
				},
			},
		},
	}).WithName("test").WithSourceType(config.FileSource))

	builder = builder.WithInheritance(
		config.Levels(config.Global, "groups", "replicasets", "instances"),
	)

	cfg, errs := builder.Build()
	require.Empty(t, errs)
	require.NotNil(t, cfg)

	instanceCfg, err := cfg.Effective(config.NewKeyPath("groups/storages/replicasets/s-001/instances/s-001-a"))
	require.NoError(t, err)

	var globalVal, replicasetVal, instanceVal string

	_, err = instanceCfg.Get(config.NewKeyPath("global_key"), &globalVal)
	require.NoError(t, err)
	assert.Equal(t, "global_value", globalVal)

	_, err = instanceCfg.Get(config.NewKeyPath("replicaset_key"), &replicasetVal)
	require.NoError(t, err)
	assert.Equal(t, "replicaset_value", replicasetVal)

	_, err = instanceCfg.Get(config.NewKeyPath("instance_key"), &instanceVal)
	require.NoError(t, err)
	assert.Equal(t, "instance_value", instanceVal)
}

func TestWithInheritance_MergeAppendNonSlice(t *testing.T) {
	t.Parallel()

	// Test that MergeAppend falls back to MergeReplace when values are not slices.
	builder := config.NewBuilder()

	builder = builder.AddCollector(collectors.NewMap(map[string]any{
		"groups": map[string]any{
			"storages": map[string]any{
				"key": "group_value",
				"replicasets": map[string]any{
					"s-001": map[string]any{
						"key": "replicaset_value",
						"instances": map[string]any{
							"s-001-a": map[string]any{},
						},
					},
				},
			},
		},
	}).WithName("test").WithSourceType(config.FileSource))

	builder = builder.WithInheritance(
		config.Levels(config.Global, "groups", "replicasets", "instances"),
		config.WithInheritMerge("key", config.MergeAppend),
	)

	cfg, errs := builder.Build()
	require.Empty(t, errs)
	require.NotNil(t, cfg)

	instanceCfg, err := cfg.Effective(config.NewKeyPath("groups/storages/replicasets/s-001/instances/s-001-a"))
	require.NoError(t, err)

	var val string

	_, err = instanceCfg.Get(config.NewKeyPath("key"), &val)
	require.NoError(t, err)

	assert.Equal(t, "replicaset_value", val)
}

func TestWithInheritance_NoInheritanceConfigured(t *testing.T) {
	t.Parallel()

	builder := config.NewBuilder()

	builder = builder.AddCollector(collectors.NewMap(map[string]any{
		"groups": map[string]any{
			"storages": map[string]any{
				"replicasets": map[string]any{
					"s-001": map[string]any{
						"instances": map[string]any{
							"s-001-a": map[string]any{
								"key": "value",
							},
						},
					},
				},
			},
		},
	}).WithName("test").WithSourceType(config.FileSource))

	cfg, errs := builder.Build()
	require.Empty(t, errs)
	require.NotNil(t, cfg)

	instanceCfg, err := cfg.Effective(config.NewKeyPath("groups/storages/replicasets/s-001/instances/s-001-a"))
	require.NoError(t, err)

	var val string

	_, err = instanceCfg.Get(config.NewKeyPath("key"), &val)
	require.NoError(t, err)
	assert.Equal(t, "value", val)

	_, err = cfg.EffectiveAll()
	require.Error(t, err)
	assert.Equal(t, config.ErrNoInheritance, err)
}

func TestWithInheritance_EffectiveAllMultipleLeafs(t *testing.T) {
	t.Parallel()

	builder := config.NewBuilder()

	builder = builder.AddCollector(collectors.NewMap(map[string]any{
		"global_key": "global_value",
		"groups": map[string]any{
			"storages": map[string]any{
				"group_key": "group_value",
				"replicasets": map[string]any{
					"s-001": map[string]any{
						"replicaset_key": "replicaset_value",
						"instances": map[string]any{
							"s-001-a": map[string]any{
								"instance_key": "instance_value_a",
							},
							"s-001-b": map[string]any{
								"instance_key": "instance_value_b",
							},
						},
					},
					"s-002": map[string]any{
						"replicaset_key": "replicaset_value_2",
						"instances": map[string]any{
							"s-002-a": map[string]any{
								"instance_key": "instance_value_2a",
							},
						},
					},
				},
			},
		},
	}).WithName("test").WithSourceType(config.FileSource))

	builder = builder.WithInheritance(
		config.Levels(config.Global, "groups", "replicasets", "instances"),
	)

	cfg, errs := builder.Build()
	require.Empty(t, errs)
	require.NotNil(t, cfg)

	all, err := cfg.EffectiveAll()
	require.NoError(t, err)
	assert.Len(t, all, 3)

	cfg1, ok := all["groups/storages/replicasets/s-001/instances/s-001-a"]
	assert.True(t, ok)

	var val1 string

	_, err = cfg1.Get(config.NewKeyPath("instance_key"), &val1)
	require.NoError(t, err)
	assert.Equal(t, "instance_value_a", val1)

	cfg2, ok := all["groups/storages/replicasets/s-001/instances/s-001-b"]
	assert.True(t, ok)

	var val2 string

	_, err = cfg2.Get(config.NewKeyPath("instance_key"), &val2)
	require.NoError(t, err)
	assert.Equal(t, "instance_value_b", val2)

	cfg3, ok := all["groups/storages/replicasets/s-002/instances/s-002-a"]
	assert.True(t, ok)

	var val3 string

	_, err = cfg3.Get(config.NewKeyPath("instance_key"), &val3)
	require.NoError(t, err)
	assert.Equal(t, "instance_value_2a", val3)

	// All should inherit global and group keys.
	for _, cfg := range all {
		var globalVal, groupVal string

		_, err = cfg.Get(config.NewKeyPath("global_key"), &globalVal)
		require.NoError(t, err)
		assert.Equal(t, "global_value", globalVal)

		_, err = cfg.Get(config.NewKeyPath("group_key"), &groupVal)
		require.NoError(t, err)
		assert.Equal(t, "group_value", groupVal)
	}
}

func TestLevels_Panic(t *testing.T) {
	t.Parallel()

	// Test empty arguments panics.
	require.Panics(t, func() {
		config.Levels()
	})

	// Test first argument not Global panics.
	require.Panics(t, func() {
		config.Levels("not-empty")
	})
}

func TestWithNoInheritFrom_InvalidLevelPanic(t *testing.T) {
	t.Parallel()

	// This tests that WithNoInheritFrom panics when level not in hierarchy.
	require.Panics(t, func() {
		builder := config.NewBuilder()

		builder = builder.WithInheritance(
			config.Levels(config.Global, "groups", "replicasets"),
			config.WithNoInheritFrom("invalid-level", "key"),
		)
	})
}

func TestWithInheritance_NoInheritFrom_PrefixMatching(t *testing.T) {
	t.Parallel()

	builder := config.NewBuilder()

	builder = builder.AddCollector(collectors.NewMap(map[string]any{
		"credentials": map[string]any{
			"users": map[string]any{
				"admin": map[string]any{
					"password": "global_password",
					"roles":    []any{"global_role"},
				},
			},
		},
		"groups": map[string]any{
			"storages": map[string]any{
				"credentials": map[string]any{
					"users": map[string]any{
						"admin": map[string]any{
							"password": "group_password",
							// Roles not defined at group level.
						},
					},
				},
				"replicasets": map[string]any{
					"s-001": map[string]any{
						"instances": map[string]any{
							"s-001-a": map[string]any{},
						},
					},
				},
			},
		},
	}).WithName("test").WithSourceType(config.FileSource))

	// Exclude credentials.users at global level (prefix matching).
	builder = builder.WithInheritance(
		config.Levels(config.Global, "groups", "replicasets", "instances"),
		config.WithNoInheritFrom(config.Global, "credentials.users"),
	)

	cfg, errs := builder.Build()
	require.Empty(t, errs)
	require.NotNil(t, cfg)

	instanceCfg, err := cfg.Effective(config.NewKeyPath("groups/storages/replicasets/s-001/instances/s-001-a"))
	require.NoError(t, err)

	// credentials.users.admin.password should come from group (global excluded).
	var password string

	_, err = instanceCfg.Get(config.NewKeyPath("credentials/users/admin/password"), &password)
	require.NoError(t, err)
	assert.Equal(t, "group_password", password)

	// credentials.users.admin.roles should NOT be inherited from global (excluded by prefix).
	// Since not defined at group level, should be missing.
	var roles []string

	_, err = instanceCfg.Get(config.NewKeyPath("credentials/users/admin/roles"), &roles)
	require.Error(t, err)
}

func TestWithInheritance_MergeDeep_LeafMapMismatch(t *testing.T) {
	t.Parallel()

	builder := config.NewBuilder()

	builder = builder.AddCollector(collectors.NewMap(map[string]any{
		"config": map[string]any{
			"setting": "global_value",
		},
		"groups": map[string]any{
			"storages": map[string]any{
				"config": "group_string", // Leaf string, not map.
				"replicasets": map[string]any{
					"s-001": map[string]any{
						"instances": map[string]any{
							"s-001-a": map[string]any{},
						},
					},
				},
			},
		},
	}).WithName("test").WithSourceType(config.FileSource))

	builder = builder.WithInheritance(
		config.Levels(config.Global, "groups", "replicasets", "instances"),
		config.WithInheritMerge("config", config.MergeDeep),
	)

	cfg, errs := builder.Build()
	require.Empty(t, errs)
	require.NotNil(t, cfg)

	instanceCfg, err := cfg.Effective(config.NewKeyPath("groups/storages/replicasets/s-001/instances/s-001-a"))
	require.NoError(t, err)

	// With MergeDeep, if child value is not a map, should fall back to replace.
	// So config should be "group_string" (leaf), not a map.
	var configVal string

	_, err = instanceCfg.Get(config.NewKeyPath("config"), &configVal)
	require.NoError(t, err)
	assert.Equal(t, "group_string", configVal)

	// Verify it's not a map (setting key should not exist).
	var setting string

	_, err = instanceCfg.Get(config.NewKeyPath("config/setting"), &setting)
	require.Error(t, err)
}

func TestWithInheritance_MergeDeep_MapLeafMismatch(t *testing.T) {
	t.Parallel()

	builder := config.NewBuilder()

	builder = builder.AddCollector(collectors.NewMap(map[string]any{
		"config": "global_string", // Leaf string.
		"groups": map[string]any{
			"storages": map[string]any{
				"config": map[string]any{ // Map at group level.
					"setting": "group_value",
				},
				"replicasets": map[string]any{
					"s-001": map[string]any{
						"instances": map[string]any{
							"s-001-a": map[string]any{},
						},
					},
				},
			},
		},
	}).WithName("test").WithSourceType(config.FileSource))

	builder = builder.WithInheritance(
		config.Levels(config.Global, "groups", "replicasets", "instances"),
		config.WithInheritMerge("config", config.MergeDeep),
	)

	cfg, errs := builder.Build()
	require.Empty(t, errs)
	require.NotNil(t, cfg)

	instanceCfg, err := cfg.Effective(config.NewKeyPath("groups/storages/replicasets/s-001/instances/s-001-a"))
	require.NoError(t, err)

	// With MergeDeep, if parent value is not a map, should fall back to replace.
	// So config should be map with setting = "group_value".
	var configVal map[string]any

	_, err = instanceCfg.Get(config.NewKeyPath("config"), &configVal)
	require.NoError(t, err)
	assert.Equal(t, "group_value", configVal["setting"])
}

func TestWithInheritance_MergeAppend_ParentSliceChildNotSlice(t *testing.T) {
	t.Parallel()

	builder := config.NewBuilder()

	builder = builder.AddCollector(collectors.NewMap(map[string]any{
		"tags": []any{"global"},
		"groups": map[string]any{
			"storages": map[string]any{
				"tags": "group_string",
				"replicasets": map[string]any{
					"s-001": map[string]any{
						"instances": map[string]any{
							"s-001-a": map[string]any{},
						},
					},
				},
			},
		},
	}).WithName("test").WithSourceType(config.FileSource))

	builder = builder.WithInheritance(
		config.Levels(config.Global, "groups", "replicasets", "instances"),
		config.WithInheritMerge("tags", config.MergeAppend),
	)

	cfg, errs := builder.Build()
	require.Empty(t, errs)
	require.NotNil(t, cfg)

	instanceCfg, err := cfg.Effective(config.NewKeyPath("groups/storages/replicasets/s-001/instances/s-001-a"))
	require.NoError(t, err)

	// With MergeAppend, if child value is not a slice, should fall back to replace.
	// So tags should be "group_string".
	var tags string

	_, err = instanceCfg.Get(config.NewKeyPath("tags"), &tags)
	require.NoError(t, err)
	assert.Equal(t, "group_string", tags)
}

func TestWithInheritance_MergeAppend_ParentMissingChildSlice(t *testing.T) {
	t.Parallel()

	builder := config.NewBuilder()

	builder = builder.AddCollector(collectors.NewMap(map[string]any{
		// Global has no tags.
		"groups": map[string]any{
			"storages": map[string]any{
				"tags": []any{"storage"},
				"replicasets": map[string]any{
					"s-001": map[string]any{
						"instances": map[string]any{
							"s-001-a": map[string]any{},
						},
					},
				},
			},
		},
	}).WithName("test").WithSourceType(config.FileSource))

	builder = builder.WithInheritance(
		config.Levels(config.Global, "groups", "replicasets", "instances"),
		config.WithInheritMerge("tags", config.MergeAppend),
	)

	cfg, errs := builder.Build()
	require.Empty(t, errs)
	require.NotNil(t, cfg)

	instanceCfg, err := cfg.Effective(config.NewKeyPath("groups/storages/replicasets/s-001/instances/s-001-a"))
	require.NoError(t, err)

	// With MergeAppend, parent missing (nil), child slice -> should use child slice.
	var tags []string

	_, err = instanceCfg.Get(config.NewKeyPath("tags"), &tags)
	require.NoError(t, err)
	assert.Equal(t, []string{"storage"}, tags)
}

func TestWithInheritance_MergeDeep_NestedStrategyPath(t *testing.T) {
	t.Parallel()

	builder := config.NewBuilder()

	builder = builder.AddCollector(collectors.NewMap(map[string]any{
		"credentials": map[string]any{
			"users": map[string]any{
				"replicator": map[string]any{
					"password": "secret",
					"roles":    []any{"replication"},
				},
			},
		},
		"groups": map[string]any{
			"storages": map[string]any{
				"credentials": map[string]any{
					"users": map[string]any{
						"monitor": map[string]any{
							"password": "m",
							"roles":    []any{"metrics"},
						},
					},
				},
				"replicasets": map[string]any{
					"s-001": map[string]any{
						"instances": map[string]any{
							"s-001-a": map[string]any{},
						},
					},
				},
			},
		},
	}).WithName("test").WithSourceType(config.FileSource))

	// Strategy is on a nested path "credentials/users", not on the
	// top-level "credentials" key. The merge must walk down to "users"
	// and apply MergeDeep there.
	builder = builder.WithInheritance(
		config.Levels(config.Global, "groups", "replicasets", "instances"),
		config.WithInheritMerge("credentials/users", config.MergeDeep),
	)

	cfg, errs := builder.Build()
	require.Empty(t, errs)
	require.NotNil(t, cfg)

	instanceCfg, err := cfg.Effective(config.NewKeyPath("groups/storages/replicasets/s-001/instances/s-001-a"))
	require.NoError(t, err)

	// Both global "replicator" and group "monitor" must be present.
	var users map[string]any

	_, err = instanceCfg.Get(config.NewKeyPath("credentials/users"), &users)
	require.NoError(t, err)
	assert.Contains(t, users, "replicator", "global-level user should be preserved by MergeDeep")
	assert.Contains(t, users, "monitor", "group-level user should be present")

	// Verify values are intact.
	var password string

	_, err = instanceCfg.Get(config.NewKeyPath("credentials/users/replicator/password"), &password)
	require.NoError(t, err)
	assert.Equal(t, "secret", password)

	_, err = instanceCfg.Get(config.NewKeyPath("credentials/users/monitor/password"), &password)
	require.NoError(t, err)
	assert.Equal(t, "m", password)
}

func TestWithInheritance_ParentAndChildStrategies(t *testing.T) {
	t.Parallel()

	builder := config.NewBuilder()

	builder = builder.AddCollector(collectors.NewMap(map[string]any{
		"credentials": map[string]any{
			"users": map[string]any{
				"admin": map[string]any{
					"password": "admin123",
				},
			},
			"settings": map[string]any{
				"timeout": 30,
				"retries": 3,
			},
		},
		"groups": map[string]any{
			"storages": map[string]any{
				"credentials": map[string]any{
					"users": map[string]any{
						"monitor": map[string]any{
							"password": "monitor123",
						},
					},
					"settings": map[string]any{
						"timeout": 60,
					},
				},
				"replicasets": map[string]any{
					"s-001": map[string]any{
						"instances": map[string]any{
							"s-001-a": map[string]any{},
						},
					},
				},
			},
		},
	}).WithName("test").WithSourceType(config.FileSource))

	// Parent "credentials" has MergeDeep; child "credentials/users" overrides with MergeReplace.
	// Sibling keys under "credentials" (e.g. "settings") should inherit MergeDeep from parent.
	builder = builder.WithInheritance(
		config.Levels(config.Global, "groups", "replicasets", "instances"),
		config.WithInheritMerge("credentials", config.MergeDeep),
		config.WithInheritMerge("credentials/users", config.MergeReplace),
	)

	cfg, errs := builder.Build()
	require.Empty(t, errs)
	require.NotNil(t, cfg)

	instanceCfg, err := cfg.Effective(config.NewKeyPath("groups/storages/replicasets/s-001/instances/s-001-a"))
	require.NoError(t, err)

	// credentials/users: MergeReplace overrides parent MergeDeep.
	// Only the group-level "monitor" user should be present; "admin" is replaced away.
	var users map[string]any

	_, err = instanceCfg.Get(config.NewKeyPath("credentials/users"), &users)
	require.NoError(t, err)
	assert.Len(t, users, 1)
	assert.Contains(t, users, "monitor")
	assert.NotContains(t, users, "admin")

	// credentials/settings: inherits MergeDeep from parent.
	// Group sets timeout=60; global has retries=3. Deep merge should preserve both.
	var timeout int

	_, err = instanceCfg.Get(config.NewKeyPath("credentials/settings/timeout"), &timeout)
	require.NoError(t, err)
	assert.Equal(t, 60, timeout)

	var retries int

	_, err = instanceCfg.Get(config.NewKeyPath("credentials/settings/retries"), &retries)
	require.NoError(t, err)
	assert.Equal(t, 3, retries)
}

func TestConfig_Walk_ContextCancellation(t *testing.T) {
	t.Parallel()

	builder := config.NewBuilder()

	builder = builder.AddCollector(collectors.NewMap(map[string]any{
		"key1": "value1",
		"key2": "value2",
		"nested": map[string]any{
			"key3": "value3",
		},
	}).WithName("test").WithSourceType(config.FileSource))

	cfg, errs := builder.Build()
	require.Empty(t, errs)
	require.NotNil(t, cfg)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	valueCh, err := cfg.Walk(ctx, config.NewKeyPath(""), -1)
	require.NoError(t, err)

	// Channel should be closed, may have zero or one values.
	val, ok := <-valueCh
	if ok {
		// One value received; channel should now be closed.
		_, ok = <-valueCh
		assert.False(t, ok)
		// Optional: check val.
		_ = val
	}
}
