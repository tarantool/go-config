package config_test

import (
	"context"
	"os"
	"path/filepath"
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

	cfg, errs := builder.Build(t.Context())
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

// TestWithInheritance_CrossScope_DeeplyNestedMapMerge verifies that the
// default deep-merge recurses arbitrarily deep into map structures: a
// global a/b/c/d/e/{x:1} and an instance a/b/c/d/e/{y:2} both survive
// end-to-end. The previous wholesale-replace default would have lost x.
func TestWithInheritance_CrossScope_DeeplyNestedMapMerge(t *testing.T) {
	t.Parallel()

	builder := config.NewBuilder()

	builder = builder.AddCollector(collectors.NewMap(map[string]any{
		"a": map[string]any{
			"b": map[string]any{
				"c": map[string]any{
					"d": map[string]any{
						"e": map[string]any{"x": 1},
					},
				},
			},
		},
		"groups": map[string]any{
			"storages": map[string]any{
				"replicasets": map[string]any{
					"s-001": map[string]any{
						"instances": map[string]any{
							"s-001-a": map[string]any{
								"a": map[string]any{
									"b": map[string]any{
										"c": map[string]any{
											"d": map[string]any{
												"e": map[string]any{"y": 2},
											},
										},
									},
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

	cfg, errs := builder.Build(t.Context())
	require.Empty(t, errs)

	eff, err := cfg.Effective(
		config.NewKeyPath("groups/storages/replicasets/s-001/instances/s-001-a"))
	require.NoError(t, err)

	var valX, valY int

	_, err = eff.Get(config.NewKeyPath("a/b/c/d/e/x"), &valX)
	require.NoError(t, err, "deeply nested lower-priority sub-key must survive")
	assert.Equal(t, 1, valX)

	_, err = eff.Get(config.NewKeyPath("a/b/c/d/e/y"), &valY)
	require.NoError(t, err, "deeply nested higher-priority sub-key must be present")
	assert.Equal(t, 2, valY)
}

// TestWithInheritance_CrossScope_FourLevelMapMerge verifies that every
// scope in the Global → group → replicaset → instance chain can contribute
// a disjoint leaf to the same nested map and all four survive the merge.
// This is the scope-chain version of the existing TestLayered_CrossLoader_
// non-conflicting test, extended to the maximum hierarchy depth.
func TestWithInheritance_CrossScope_FourLevelMapMerge(t *testing.T) {
	t.Parallel()

	builder := config.NewBuilder()

	builder = builder.AddCollector(collectors.NewMap(map[string]any{
		"replication": map[string]any{"failover": "election"}, // global.
		"groups": map[string]any{
			"storages": map[string]any{
				"replication": map[string]any{"timeout": 30}, // group.
				"replicasets": map[string]any{
					"s-001": map[string]any{
						"replication": map[string]any{"bootstrap_strategy": "auto"}, // replicaset.
						"instances": map[string]any{
							"s-001-a": map[string]any{
								"replication": map[string]any{"connect_timeout": 5}, // instance.
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

	cfg, errs := builder.Build(t.Context())
	require.Empty(t, errs)

	eff, err := cfg.Effective(
		config.NewKeyPath("groups/storages/replicasets/s-001/instances/s-001-a"))
	require.NoError(t, err)

	var failover, bootstrap string

	var timeout, connect int

	_, err = eff.Get(config.NewKeyPath("replication/failover"), &failover)
	require.NoError(t, err)
	assert.Equal(t, "election", failover, "global contribution must survive")

	_, err = eff.Get(config.NewKeyPath("replication/timeout"), &timeout)
	require.NoError(t, err)
	assert.Equal(t, 30, timeout, "group contribution must survive")

	_, err = eff.Get(config.NewKeyPath("replication/bootstrap_strategy"), &bootstrap)
	require.NoError(t, err)
	assert.Equal(t, "auto", bootstrap, "replicaset contribution must survive")

	_, err = eff.Get(config.NewKeyPath("replication/connect_timeout"), &connect)
	require.NoError(t, err)
	assert.Equal(t, 5, connect, "instance contribution must survive")
}

// TestWithInheritance_CrossScope_MultiLevelConflictPriority verifies that
// when multiple scopes set the same leaf path, the highest-priority scope
// that sets it wins — not just the leaf scope (the instance may not
// override it). With a global=1, group=2, replicaset=3, instance-unset
// chain, the effective value must be 3.
func TestWithInheritance_CrossScope_MultiLevelConflictPriority(t *testing.T) {
	t.Parallel()

	builder := config.NewBuilder()

	builder = builder.AddCollector(collectors.NewMap(map[string]any{
		"replication": map[string]any{"timeout": 1}, // global.
		"groups": map[string]any{
			"storages": map[string]any{
				"replication": map[string]any{"timeout": 2}, // group.
				"replicasets": map[string]any{
					"s-001": map[string]any{
						"replication": map[string]any{"timeout": 3}, // replicaset.
						"instances": map[string]any{
							"s-001-a": map[string]any{}, // instance has no replication.
						},
					},
				},
			},
		},
	}).WithName("test").WithSourceType(config.FileSource))

	builder = builder.WithInheritance(
		config.Levels(config.Global, "groups", "replicasets", "instances"),
	)

	cfg, errs := builder.Build(t.Context())
	require.Empty(t, errs)

	eff, err := cfg.Effective(
		config.NewKeyPath("groups/storages/replicasets/s-001/instances/s-001-a"))
	require.NoError(t, err)

	var timeout int

	_, err = eff.Get(config.NewKeyPath("replication/timeout"), &timeout)
	require.NoError(t, err)
	assert.Equal(t, 3, timeout, "highest-priority defined scope must win")
}

// TestWithInheritance_CrossScope_DefaultMatchesExplicitMergeDeep is a
// sanity check that the new default strategy is wired exactly to
// MergeDeep. Two builders with the same input — one relying on the default,
// one declaring WithInheritMerge("iproto", MergeDeep) — must produce
// identical effective views.
func TestWithInheritance_CrossScope_DefaultMatchesExplicitMergeDeep(t *testing.T) {
	t.Parallel()

	data := map[string]any{
		"iproto": map[string]any{
			"advertise": map[string]any{"peer": map[string]any{"login": "replicator"}},
			"listen":    map[string]any{"params": map[string]any{"transport": "plain"}},
		},
		"groups": map[string]any{
			"storages": map[string]any{
				"replicasets": map[string]any{
					"s-001": map[string]any{
						"instances": map[string]any{
							"s-001-a": map[string]any{
								"iproto": map[string]any{
									"listen": map[string]any{"uri": "127.0.0.1:3301"},
								},
							},
						},
					},
				},
			},
		},
	}

	build := func(explicit bool) config.Config {
		t.Helper()

		builder := config.NewBuilder()

		builder = builder.AddCollector(
			collectors.NewMap(data).WithName("test").WithSourceType(config.FileSource),
		)
		if explicit {
			builder = builder.WithInheritance(
				config.Levels(config.Global, "groups", "replicasets", "instances"),
				config.WithInheritMerge("iproto", config.MergeDeep),
			)
		} else {
			builder = builder.WithInheritance(
				config.Levels(config.Global, "groups", "replicasets", "instances"),
			)
		}

		cfg, errs := builder.Build(t.Context())
		require.Empty(t, errs)

		return cfg
	}

	for _, label := range []string{"default", "explicit"} {
		cfg := build(label == "explicit")
		eff, err := cfg.Effective(
			config.NewKeyPath("groups/storages/replicasets/s-001/instances/s-001-a"))
		require.NoError(t, err, label)

		var login string

		_, err = eff.Get(config.NewKeyPath("iproto/advertise/peer/login"), &login)
		require.NoError(t, err, label)
		assert.Equal(t, "replicator", login, label)

		var transport string

		_, err = eff.Get(config.NewKeyPath("iproto/listen/params/transport"), &transport)
		require.NoError(t, err, label)
		assert.Equal(t, "plain", transport, label)

		var uri string

		_, err = eff.Get(config.NewKeyPath("iproto/listen/uri"), &uri)
		require.NoError(t, err, label)
		assert.Equal(t, "127.0.0.1:3301", uri, label)
	}
}

// TestWithInheritance_NoInheritFrom_DeepPathPruning extends the prefix-
// matching test with several deeper-path variants now that exclusions are
// enforced at every depth (not just top-level iteration keys). Each
// sub-case configures a different excluded prefix and verifies that the
// pruned path is gone while siblings survive.
func TestWithInheritance_NoInheritFrom_DeepPathPruning(t *testing.T) {
	t.Parallel()

	mkCfg := func(t *testing.T, excluded string) config.Config {
		t.Helper()

		builder := config.NewBuilder()

		builder = builder.AddCollector(collectors.NewMap(map[string]any{
			"credentials": map[string]any{
				"users": map[string]any{
					"admin": map[string]any{
						"password": "global-admin-pass",
						"roles":    []any{"super"},
					},
					"monitor": map[string]any{
						"password": "global-monitor-pass",
					},
				},
				"settings": map[string]any{
					"timeout": 30,
				},
			},
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
		}).WithName("test").WithSourceType(config.FileSource))
		builder = builder.WithInheritance(
			config.Levels(config.Global, "groups", "replicasets", "instances"),
			config.WithNoInheritFrom(config.Global, excluded),
		)

		cfg, errs := builder.Build(t.Context())
		require.Empty(t, errs)

		return cfg
	}

	t.Run("ExcludeSpecificUser", func(t *testing.T) {
		t.Parallel()

		cfg := mkCfg(t, "credentials/users/admin")

		eff, err := cfg.Effective(
			config.NewKeyPath("groups/storages/replicasets/s-001/instances/s-001-a"))
		require.NoError(t, err)

		// admin pruned away.
		_, err = eff.Get(config.NewKeyPath("credentials/users/admin/password"), new(string))
		require.Error(t, err, "excluded sub-tree must be pruned")

		// monitor (sibling under credentials/users) and settings (sibling
		// under credentials) survive.
		var monitorPass string

		_, err = eff.Get(config.NewKeyPath("credentials/users/monitor/password"), &monitorPass)
		require.NoError(t, err)
		assert.Equal(t, "global-monitor-pass", monitorPass)

		var timeout int

		_, err = eff.Get(config.NewKeyPath("credentials/settings/timeout"), &timeout)
		require.NoError(t, err)
		assert.Equal(t, 30, timeout)
	})

	t.Run("ExcludeLeafFieldOnly", func(t *testing.T) {
		t.Parallel()

		cfg := mkCfg(t, "credentials/users/admin/roles")

		eff, err := cfg.Effective(
			config.NewKeyPath("groups/storages/replicasets/s-001/instances/s-001-a"))
		require.NoError(t, err)

		// roles pruned, password survives (same admin map).
		_, err = eff.Get(config.NewKeyPath("credentials/users/admin/roles"), new([]string))
		require.Error(t, err)

		var pass string

		_, err = eff.Get(config.NewKeyPath("credentials/users/admin/password"), &pass)
		require.NoError(t, err)
		assert.Equal(t, "global-admin-pass", pass)
	})

	t.Run("ExcludeWholeTopLevelKey", func(t *testing.T) {
		t.Parallel()

		cfg := mkCfg(t, "credentials")

		eff, err := cfg.Effective(
			config.NewKeyPath("groups/storages/replicasets/s-001/instances/s-001-a"))
		require.NoError(t, err)

		// Entire credentials subtree from global is pruned.
		_, err = eff.Get(config.NewKeyPath("credentials/users/admin/password"), new(string))
		require.Error(t, err)

		_, err = eff.Get(config.NewKeyPath("credentials/settings/timeout"), new(int))
		require.Error(t, err)
	})
}

// TestWithInheritance_CrossScope_DeepNoInheritStillFires verifies that
// WithNoInherit (universal exclusion) also applies at every depth via the
// new pruning path — not just on top-level layer iteration. The excluded
// sub-tree must vanish from every non-leaf scope while the leaf (instance)
// can still set the same path for itself.
func TestWithInheritance_CrossScope_DeepNoInheritStillFires(t *testing.T) {
	t.Parallel()

	builder := config.NewBuilder()

	builder = builder.AddCollector(collectors.NewMap(map[string]any{
		"iproto": map[string]any{
			"advertise": map[string]any{
				"peer": map[string]any{"login": "global-login"},
			},
		},
		"groups": map[string]any{
			"storages": map[string]any{
				"iproto": map[string]any{
					"advertise": map[string]any{
						"peer": map[string]any{"login": "group-login"},
					},
				},
				"replicasets": map[string]any{
					"s-001": map[string]any{
						"instances": map[string]any{
							"s-001-a": map[string]any{
								"iproto": map[string]any{
									"advertise": map[string]any{
										"peer": map[string]any{"login": "instance-login"},
									},
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
		config.WithNoInherit("iproto/advertise/peer"),
	)

	cfg, errs := builder.Build(t.Context())
	require.Empty(t, errs)

	eff, err := cfg.Effective(
		config.NewKeyPath("groups/storages/replicasets/s-001/instances/s-001-a"))
	require.NoError(t, err)

	// Instance's own value (leaf scope) is preserved per the leaf-scope
	// carve-out.
	var login string

	_, err = eff.Get(config.NewKeyPath("iproto/advertise/peer/login"), &login)
	require.NoError(t, err)
	assert.Equal(t, "instance-login", login)
}

// TestWithInheritance_CrossScope_NonConflictingSubkeysCoexist is the
// scope-chain analogue of TestLayered_CrossLoader_NonConflictingSubkeysCoexist:
// when a single loader sets disjoint sub-keys of the same top-level map at
// different scopes (e.g. instance sets iproto/listen while global sets
// iproto/advertise/peer/login), both must survive in the effective view. The
// higher-priority scope's bare presence of "iproto" must not wipe out the
// lower-priority scope's sibling sub-keys.
func TestWithInheritance_CrossScope_NonConflictingSubkeysCoexist(t *testing.T) {
	t.Parallel()

	builder := config.NewBuilder()

	builder = builder.AddCollector(collectors.NewMap(map[string]any{
		"iproto": map[string]any{
			"advertise": map[string]any{
				"peer": map[string]any{"login": "replicator"},
			},
		},
		"groups": map[string]any{
			"storages": map[string]any{
				"replicasets": map[string]any{
					"s-001": map[string]any{
						"instances": map[string]any{
							"s-001-a": map[string]any{
								"iproto": map[string]any{
									"listen": []any{
										map[string]any{"uri": "127.0.0.1:3301"},
									},
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

	cfg, errs := builder.Build(t.Context())
	require.Empty(t, errs)

	instanceCfg, err := cfg.Effective(
		config.NewKeyPath("groups/storages/replicasets/s-001/instances/s-001-a"))
	require.NoError(t, err)

	var login string

	_, err = instanceCfg.Get(config.NewKeyPath("iproto/advertise/peer/login"), &login)
	require.NoError(t, err, "lower-priority scope sub-key must survive")
	assert.Equal(t, "replicator", login)

	var listen []map[string]string

	_, err = instanceCfg.Get(config.NewKeyPath("iproto/listen"), &listen)
	require.NoError(t, err, "higher-priority scope sub-key must be present")
	require.Len(t, listen, 1)
	assert.Equal(t, "127.0.0.1:3301", listen[0]["uri"])
}

// TestWithInheritance_CrossScope_ConflictingLeafHigherPriorityWins is the
// scope-chain analogue of
// TestLayered_CrossLoader_ConflictingSubkeyHigherPriorityWins: when both a
// parent and a child scope set the same leaf, the child scope wins, while
// non-conflicting sibling sub-keys from the parent scope still coexist in
// the effective view.
func TestWithInheritance_CrossScope_ConflictingLeafHigherPriorityWins(t *testing.T) {
	t.Parallel()

	builder := config.NewBuilder()

	builder = builder.AddCollector(collectors.NewMap(map[string]any{
		"iproto": map[string]any{
			"advertise": map[string]any{
				"peer":     map[string]any{"login": "replicator"},
				"sharding": map[string]any{"login": "storage"},
			},
		},
		"groups": map[string]any{
			"storages": map[string]any{
				"replicasets": map[string]any{
					"s-001": map[string]any{
						"instances": map[string]any{
							"s-001-a": map[string]any{
								"iproto": map[string]any{
									"advertise": map[string]any{
										"peer": map[string]any{"login": "replicator-instance"},
									},
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

	cfg, errs := builder.Build(t.Context())
	require.Empty(t, errs)

	instanceCfg, err := cfg.Effective(
		config.NewKeyPath("groups/storages/replicasets/s-001/instances/s-001-a"))
	require.NoError(t, err)

	var peerLogin string

	_, err = instanceCfg.Get(config.NewKeyPath("iproto/advertise/peer/login"), &peerLogin)
	require.NoError(t, err)
	assert.Equal(t, "replicator-instance", peerLogin, "instance scope wins on conflict")

	var shardingLogin string

	_, err = instanceCfg.Get(config.NewKeyPath("iproto/advertise/sharding/login"), &shardingLogin)
	require.NoError(t, err, "non-conflicting sibling from global scope must survive")
	assert.Equal(t, "storage", shardingLogin)
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

	cfg, errs := builder.Build(t.Context())
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

	cfg, errs := builder.Build(t.Context())
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

	cfg, errs := builder.Build(t.Context())
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

	cfg, errs := builder.Build(t.Context())
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

	cfg, errs := builder.Build(t.Context())
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

	cfg, errs := builder.Build(t.Context())
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

	cfg, errs := builder.Build(t.Context())
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

	cfg, errs := builder.Build(t.Context())
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

	cfg, errs := builder.Build(t.Context())
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

	cfg, errs := builder.Build(t.Context())
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

	cfg, errs := builder.Build(t.Context())
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

	cfg, errs := builder.Build(t.Context())
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

	cfg, errs := builder.Build(t.Context())
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

	cfg, errs := builder.Build(t.Context())
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

func TestWithInheritance_EffectiveAll_EmptyMappingLeaf(t *testing.T) {
	t.Parallel()

	builder := config.NewBuilder()

	builder = builder.AddCollector(collectors.NewMap(map[string]any{
		"replication": map[string]any{"failover": "manual"},
		"groups": map[string]any{
			"storages": map[string]any{
				"replicasets": map[string]any{
					"r-001": map[string]any{
						"instances": map[string]any{
							"inst1": map[string]any{},
						},
					},
				},
			},
		},
	}).WithName("test").WithSourceType(config.FileSource))

	builder = builder.WithInheritance(
		config.Levels(config.Global, "groups", "replicasets", "instances"),
	)

	cfg, errs := builder.Build(t.Context())
	require.Empty(t, errs)
	require.NotNil(t, cfg)

	all, err := cfg.EffectiveAll()
	require.NoError(t, err)
	assert.Len(t, all, 1)

	instanceCfg, ok := all["groups/storages/replicasets/r-001/instances/inst1"]
	assert.True(t, ok)

	var failover string

	_, err = instanceCfg.Get(config.NewKeyPath("replication/failover"), &failover)
	require.NoError(t, err)
	assert.Equal(t, "manual", failover)
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

	// Exclude credentials/users at global level (prefix matching). Keys are
	// "/"-separated; the previous "credentials.users" form silently matched
	// nothing and only "worked" because the default merge strategy used to
	// wholesale-replace credentials at the group scope.
	builder = builder.WithInheritance(
		config.Levels(config.Global, "groups", "replicasets", "instances"),
		config.WithNoInheritFrom(config.Global, "credentials/users"),
	)

	cfg, errs := builder.Build(t.Context())
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

	cfg, errs := builder.Build(t.Context())
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

	cfg, errs := builder.Build(t.Context())
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

	cfg, errs := builder.Build(t.Context())
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

	cfg, errs := builder.Build(t.Context())
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

	cfg, errs := builder.Build(t.Context())
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

	cfg, errs := builder.Build(t.Context())
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

	cfg, errs := builder.Build(t.Context())
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

// ---------------------------------------------------------------------------
// Layered Effective / EffectiveAll tests
// ---------------------------------------------------------------------------.

// TestLayered_LoaderPriorityBeatsScope verifies that a value set at the global
// scope in a higher-priority loader overrides a value set at the instance scope
// in a lower-priority loader (IV1).  This is the "audit_log/extract_key" class
// of bug: instance-scope in loader-A must lose to global-scope in loader-B when
// loader-B has higher priority.
// The test also checks that Stat reports the source of the winning value (PC1).
func TestLayered_LoaderPriorityBeatsScope(t *testing.T) {
	t.Parallel()

	// Loader 1 (lower priority): sets audit_log/extract_key at instance scope.
	loader1 := collectors.NewMap(map[string]any{
		"groups": map[string]any{
			"storages": map[string]any{
				"replicasets": map[string]any{
					"s-001": map[string]any{
						"instances": map[string]any{
							"s-001-a": map[string]any{
								"audit_log": map[string]any{
									"extract_key": "instance-value",
								},
							},
						},
					},
				},
			},
		},
	}).WithName("loader1").WithSourceType(config.FileSource)

	// Loader 2 (higher priority): sets audit_log/extract_key at global scope.
	loader2 := collectors.NewMap(map[string]any{
		"audit_log": map[string]any{
			"extract_key": "global-override",
		},
	}).WithName("loader2").WithSourceType(config.EnvSource)

	builder := config.NewBuilder()

	builder = builder.AddCollector(loader1) // lower priority.
	builder = builder.AddCollector(loader2) // higher priority.
	builder = builder.WithInheritance(
		config.Levels(config.Global, "groups", "replicasets", "instances"),
	)

	cfg, errs := builder.Build(t.Context())
	require.Empty(t, errs)

	leafPath := config.NewKeyPath("groups/storages/replicasets/s-001/instances/s-001-a")
	eff, err := cfg.Effective(leafPath)
	require.NoError(t, err)

	var val string

	_, err = eff.Get(config.NewKeyPath("audit_log/extract_key"), &val)
	require.NoError(t, err)
	// Loader2 (global scope, higher priority) must win over loader1 (instance scope).
	assert.Equal(t, "global-override", val,
		"higher-priority loader global value must beat lower-priority loader instance value")

	// PC1: Stat must report the originating source (loader2).
	// Note: SourceType is not stored in tree nodes; only the collector name is.
	meta, ok := eff.Stat(config.NewKeyPath("audit_log/extract_key"))
	require.True(t, ok)
	assert.Equal(t, "loader2", meta.Source.Name,
		"Stat must report the source name of the winning value")
}

// TestLayered_MergeAppendAcrossLoaders verifies that MergeAppend composes
// across loader boundaries (values from different loaders are appended).
func TestLayered_MergeAppendAcrossLoaders(t *testing.T) {
	t.Parallel()

	// Loader 1: roles at global scope.
	loader1 := collectors.NewMap(map[string]any{
		"roles": []any{"storage"},
	}).WithName("loader1").WithSourceType(config.FileSource)

	// Loader 2 (higher priority): roles at global scope — should append.
	loader2 := collectors.NewMap(map[string]any{
		"roles": []any{"metrics"},
	}).WithName("loader2").WithSourceType(config.EnvSource)

	builder := config.NewBuilder()

	builder = builder.AddCollector(loader1)
	builder = builder.AddCollector(loader2)
	builder = builder.WithInheritance(
		config.Levels(config.Global, "groups", "replicasets", "instances"),
		config.WithInheritMerge("roles", config.MergeAppend),
	)

	cfg, errs := builder.Build(t.Context())
	require.Empty(t, errs)

	// Any leaf path works; there's no explicit instances, so use nonexistent
	// (matchHierarchy returns ok=true, all nodes nil except global).
	eff, err := cfg.Effective(
		config.NewKeyPath("groups/storages/replicasets/s-001/instances/s-001-a"))
	require.NoError(t, err)

	var roles []string

	_, err = eff.Get(config.NewKeyPath("roles"), &roles)
	require.NoError(t, err)
	// Loader1 provides ["storage"], loader2 provides ["metrics"].
	// With MergeAppend both should appear, loader1 first (lower priority), loader2 second.
	assert.Equal(t, []string{"storage", "metrics"}, roles)
}

// TestLayered_MergeDeepAcrossLoaders verifies that MergeDeep composes across
// loader boundaries (users from both loaders are present in effective config).
func TestLayered_MergeDeepAcrossLoaders(t *testing.T) {
	t.Parallel()

	// Loader 1: user "admin" at global scope.
	loader1 := collectors.NewMap(map[string]any{
		"credentials": map[string]any{
			"users": map[string]any{
				"admin": map[string]any{"password": "secret"},
			},
		},
	}).WithName("loader1").WithSourceType(config.FileSource)

	// Loader 2 (higher priority): user "monitor" at global scope.
	loader2 := collectors.NewMap(map[string]any{
		"credentials": map[string]any{
			"users": map[string]any{
				"monitor": map[string]any{"password": "mon"},
			},
		},
	}).WithName("loader2").WithSourceType(config.EnvSource)

	builder := config.NewBuilder()

	builder = builder.AddCollector(loader1)
	builder = builder.AddCollector(loader2)
	builder = builder.WithInheritance(
		config.Levels(config.Global, "groups", "replicasets", "instances"),
		config.WithInheritMerge("credentials", config.MergeDeep),
	)

	cfg, errs := builder.Build(t.Context())
	require.Empty(t, errs)

	eff, err := cfg.Effective(
		config.NewKeyPath("groups/storages/replicasets/s-001/instances/s-001-a"))
	require.NoError(t, err)

	var users map[string]any

	_, err = eff.Get(config.NewKeyPath("credentials/users"), &users)
	require.NoError(t, err)
	assert.Contains(t, users, "admin", "admin user from loader1 must be present")
	assert.Contains(t, users, "monitor", "monitor user from loader2 must be present")
}

// TestLayered_SingleCollector_ScopeDepthUnchanged ensures that when only one
// collector is used, scope-depth precedence is preserved (AS3): the more
// specific (leaf) scope wins over the global scope within that collector.
func TestLayered_SingleCollector_ScopeDepthUnchanged(t *testing.T) {
	t.Parallel()

	builder := config.NewBuilder()

	builder = builder.AddCollector(collectors.NewMap(map[string]any{
		"replication": map[string]any{"failover": "manual"},
		"groups": map[string]any{
			"storages": map[string]any{
				"replication": map[string]any{"failover": "election"},
				"replicasets": map[string]any{
					"s-001": map[string]any{
						"instances": map[string]any{
							"s-001-a": map[string]any{},
						},
					},
				},
			},
		},
	}).WithName("single").WithSourceType(config.FileSource))
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
	// Group level (more specific) must override global level within the same loader.
	assert.Equal(t, "election", failover,
		"scope-depth precedence must be preserved with a single collector")
}

// TestLayered_EffectiveAll_LoaderPriorityBeatsScope verifies that EffectiveAll
// applies the same loader-priority-over-scope resolution for every leaf.
func TestLayered_EffectiveAll_LoaderPriorityBeatsScope(t *testing.T) {
	t.Parallel()

	loader1 := collectors.NewMap(map[string]any{
		"groups": map[string]any{
			"storages": map[string]any{
				"replicasets": map[string]any{
					"s-001": map[string]any{
						"instances": map[string]any{
							"s-001-a": map[string]any{
								"mode": "instance-value",
							},
							"s-001-b": map[string]any{
								"mode": "instance-value",
							},
						},
					},
				},
			},
		},
	}).WithName("loader1").WithSourceType(config.FileSource)

	loader2 := collectors.NewMap(map[string]any{
		"mode": "global-override",
	}).WithName("loader2").WithSourceType(config.EnvSource)

	builder := config.NewBuilder()

	builder = builder.AddCollector(loader1)
	builder = builder.AddCollector(loader2)
	builder = builder.WithInheritance(
		config.Levels(config.Global, "groups", "replicasets", "instances"),
	)

	cfg, errs := builder.Build(t.Context())
	require.Empty(t, errs)

	all, err := cfg.EffectiveAll()
	require.NoError(t, err)
	require.Len(t, all, 2)

	for path, eff := range all {
		var mode string

		_, err := eff.Get(config.NewKeyPath("mode"), &mode)
		require.NoError(t, err, "leaf %s", path)
		assert.Equal(t, "global-override", mode,
			"higher-priority loader global value must beat instance-scope value for leaf %s", path)
	}
}

func TestWithInheritance_YamlArrayPreserved(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	yamlData := `groups:
  storages:
    roles:
      - global-role
    replicasets:
      s-001:
        instances:
          s-001-a:
            roles:
              - instance-role
`
	err := os.WriteFile(cfgPath, []byte(yamlData), 0o600)
	require.NoError(t, err)

	collector, err := collectors.NewSource(
		t.Context(), collectors.NewFile(cfgPath), collectors.NewYamlFormat())
	require.NoError(t, err)

	builder := config.NewBuilder()

	builder = builder.AddCollector(collector)
	builder = builder.WithInheritance(
		config.Levels(config.Global, "groups", "replicasets", "instances"),
		config.WithInheritMerge("roles", config.MergeAppend),
	)

	cfg, errs := builder.Build(t.Context())
	require.Empty(t, errs)
	require.NotNil(t, cfg)

	instanceCfg, err := cfg.Effective(
		config.NewKeyPath("groups/storages/replicasets/s-001/instances/s-001-a"))
	require.NoError(t, err)

	var roles []string

	_, err = instanceCfg.Get(config.NewKeyPath("roles"), &roles)
	require.NoError(t, err)
	assert.Equal(t, []string{"global-role", "instance-role"}, roles)
}
