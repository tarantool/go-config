package config_test

import (
	"fmt"

	"github.com/tarantool/go-config"
	"github.com/tarantool/go-config/collectors"
)

// Example_inheritanceBasic demonstrates hierarchical configuration inheritance.
// It shows how to define a hierarchy (global → groups → replicasets → instances)
// and resolve effective configuration for a leaf entity.
func Example_inheritanceBasic() {
	builder := config.NewBuilder()

	// Build configuration with hierarchy.
	builder = builder.AddCollector(collectors.NewMap(map[string]any{
		"credentials": map[string]any{
			"users": map[string]any{
				"replicator": map[string]any{
					"password": "secret",
					"roles":    []any{"replication"},
				},
			},
		},
		"replication": map[string]any{"failover": "manual"},
		"groups": map[string]any{
			"storages": map[string]any{
				"sharding": map[string]any{"roles": []any{"storage"}},
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
	}).WithName("config"))

	// Register inheritance hierarchy.
	builder = builder.WithInheritance(
		config.Levels(config.Global, "groups", "replicasets", "instances"),
	)

	cfg, errs := builder.Build()
	if len(errs) > 0 {
		fmt.Printf("Build errors: %v\n", errs)
		return
	}

	// Get effective config for a specific instance.
	instanceCfg, err := cfg.Effective(config.NewKeyPath("groups/storages/replicasets/s-001/instances/s-001-a"))
	if err != nil {
		fmt.Printf("Effective error: %v\n", err)
		return
	}

	// Retrieve inherited values.
	var failover string

	_, err = instanceCfg.Get(config.NewKeyPath("replication/failover"), &failover)
	if err != nil {
		fmt.Printf("Get failover error: %v\n", err)
	} else {
		fmt.Printf("Failover: %s\n", failover)
	}

	var roles []string

	_, err = instanceCfg.Get(config.NewKeyPath("sharding/roles"), &roles)
	if err != nil {
		fmt.Printf("Get roles error: %v\n", err)
	} else {
		fmt.Printf("Roles: %v\n", roles)
	}

	var leader string

	_, err = instanceCfg.Get(config.NewKeyPath("leader"), &leader)
	if err != nil {
		fmt.Printf("Get leader error: %v\n", err)
	} else {
		fmt.Printf("Leader: %s\n", leader)
	}

	// Output:
	// Failover: manual
	// Roles: [storage]
	// Leader: s-001-a
}

// Example_inheritanceMergeStrategies demonstrates different merge strategies
// during inheritance: MergeReplace (default), MergeAppend, and MergeDeep.
func Example_inheritanceMergeStrategies() {
	builder := config.NewBuilder()

	// Configuration with values at group, replicaset, and instance levels.
	builder = builder.AddCollector(collectors.NewMap(map[string]any{
		"groups": map[string]any{
			"storages": map[string]any{
				"roles": []any{"storage"}, // Slice for append.
				"credentials": map[string]any{ // Map for deep merge.
					"users": map[string]any{
						"admin": map[string]any{"password": "admin123"},
					},
				},
				"replicasets": map[string]any{
					"s-001": map[string]any{
						"roles": []any{"metrics"}, // Append to parent slice.
						"credentials": map[string]any{ // Deep merge with parent map.
							"users": map[string]any{
								"monitor": map[string]any{"password": "monitor123"},
							},
						},
						"leader": "s-001-a", // Replace (default).
						"instances": map[string]any{
							"s-001-a": map[string]any{
								"roles": []any{"cache"}, // Further append.
								"credentials": map[string]any{ // Deep merge.
									"users": map[string]any{
										"operator": map[string]any{"password": "op123"},
									},
								},
								"iproto": map[string]any{
									"listen": []any{map[string]any{"uri": "127.0.0.1:3301"}},
								},
							},
						},
					},
				},
			},
		},
	}).WithName("config"))

	// Register hierarchy with custom merge strategies.
	builder = builder.WithInheritance(
		config.Levels(config.Global, "groups", "replicasets", "instances"),
		config.WithInheritMerge("roles", config.MergeAppend),
		config.WithInheritMerge("credentials", config.MergeDeep),
	)

	cfg, errs := builder.Build()
	if len(errs) > 0 {
		fmt.Printf("Build errors: %v\n", errs)
		return
	}

	// Get effective config for instance (leaf entity).
	instanceCfg, err := cfg.Effective(config.NewKeyPath("groups/storages/replicasets/s-001/instances/s-001-a"))
	if err != nil {
		fmt.Printf("Effective error: %v\n", err)
		return
	}

	var roles []string

	_, err = instanceCfg.Get(config.NewKeyPath("roles"), &roles)
	if err != nil {
		fmt.Printf("Get roles error: %v\n", err)
	} else {
		fmt.Printf("Roles (appended across 3 levels): %v\n", roles)
	}

	// Check merged credentials from all three levels.
	var adminPass, monitorPass, operatorPass string

	_, err = instanceCfg.Get(config.NewKeyPath("credentials/users/admin/password"), &adminPass)
	if err != nil {
		fmt.Printf("Get admin password error: %v\n", err)
	} else {
		fmt.Printf("Admin password (from group): %s\n", adminPass)
	}

	_, err = instanceCfg.Get(config.NewKeyPath("credentials/users/monitor/password"), &monitorPass)
	if err != nil {
		fmt.Printf("Get monitor password error: %v\n", err)
	} else {
		fmt.Printf("Monitor password (from replicaset): %s\n", monitorPass)
	}

	_, err = instanceCfg.Get(config.NewKeyPath("credentials/users/operator/password"), &operatorPass)
	if err != nil {
		fmt.Printf("Get operator password error: %v\n", err)
	} else {
		fmt.Printf("Operator password (from instance): %s\n", operatorPass)
	}

	var leader string

	_, err = instanceCfg.Get(config.NewKeyPath("leader"), &leader)
	if err != nil {
		fmt.Printf("Get leader error: %v\n", err)
	} else {
		fmt.Printf("Leader (replaced from replicaset): %s\n", leader)
	}

	// Output:
	// Roles (appended across 3 levels): [storage metrics cache]
	// Admin password (from group): admin123
	// Monitor password (from replicaset): monitor123
	// Operator password (from instance): op123
	// Leader (replaced from replicaset): s-001-a
}

// Example_inheritanceExclusions demonstrates how to exclude certain keys
// from inheritance using WithNoInherit and WithNoInheritFrom.
func Example_inheritanceExclusions() {
	builder := config.NewBuilder()

	// Configuration with global, group, replicaset, and instance values.
	builder = builder.AddCollector(collectors.NewMap(map[string]any{
		"snapshot": map[string]any{"dir": "/global/snapshots"},
		"groups": map[string]any{
			"storages": map[string]any{
				"snapshot": map[string]any{"dir": "/group/snapshots"},
				"leader":   "group-leader", // Excluded from inheritance.
				"replicasets": map[string]any{
					"s-001": map[string]any{
						"leader": "replicaset-leader",
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
	}).WithName("config"))

	// Register hierarchy with exclusions.
	builder = builder.WithInheritance(
		config.Levels(config.Global, "groups", "replicasets", "instances"),
		config.WithNoInherit("leader"),                          // Leader never inherited.
		config.WithNoInheritFrom(config.Global, "snapshot.dir"), // Global snapshot.dir not inherited.
	)

	cfg, errs := builder.Build()
	if len(errs) > 0 {
		fmt.Printf("Build errors: %v\n", errs)
		return
	}

	// Get effective config for instance (leaf entity).
	instanceCfg, err := cfg.Effective(config.NewKeyPath("groups/storages/replicasets/s-001/instances/s-001-a"))
	if err != nil {
		fmt.Printf("Effective error: %v\n", err)
		return
	}

	var snapshotDir string

	_, err = instanceCfg.Get(config.NewKeyPath("snapshot/dir"), &snapshotDir)
	if err != nil {
		fmt.Printf("Get snapshot.dir error: %v\n", err)
	} else {
		fmt.Printf("Snapshot dir (global excluded, group inherited): %s\n", snapshotDir)
	}

	var leader string

	_, err = instanceCfg.Get(config.NewKeyPath("leader"), &leader)
	if err != nil {
		fmt.Printf("Get leader error: %v\n", err)
	} else {
		fmt.Printf("Leader (not inherited from group, replicaset value): %s\n", leader)
	}

	// Output:
	// Snapshot dir (global excluded, group inherited): /group/snapshots
	// Get leader error: key not found: leader
}

// Example_inheritanceDefaults demonstrates how to set default values
// that apply to every leaf entity unless overridden.
func Example_inheritanceDefaults() {
	builder := config.NewBuilder()

	// Minimal configuration with replicaset and an instance.
	builder = builder.AddCollector(collectors.NewMap(map[string]any{
		"groups": map[string]any{
			"storages": map[string]any{
				"replicasets": map[string]any{
					"s-001": map[string]any{
						"leader": "s-001-a",
						"instances": map[string]any{
							"s-001-a": map[string]any{},
						},
					},
				},
			},
		},
	}).WithName("config"))

	// Register hierarchy with defaults.
	builder = builder.WithInheritance(
		config.Levels(config.Global, "groups", "replicasets", "instances"),
		config.WithDefaults(map[string]any{
			"replication": map[string]any{"failover": "manual"},
			"snapshot":    map[string]any{"dir": "/default/snapshots"},
		}),
	)

	cfg, errs := builder.Build()
	if len(errs) > 0 {
		fmt.Printf("Build errors: %v\n", errs)
		return
	}

	// Get effective config for instance (leaf entity).
	instanceCfg, err := cfg.Effective(config.NewKeyPath("groups/storages/replicasets/s-001/instances/s-001-a"))
	if err != nil {
		fmt.Printf("Effective error: %v\n", err)
		return
	}

	var failover string

	_, err = instanceCfg.Get(config.NewKeyPath("replication/failover"), &failover)
	if err != nil {
		fmt.Printf("Get failover error: %v\n", err)
	} else {
		fmt.Printf("Failover (default): %s\n", failover)
	}

	var snapshotDir string

	_, err = instanceCfg.Get(config.NewKeyPath("snapshot/dir"), &snapshotDir)
	if err != nil {
		fmt.Printf("Get snapshot.dir error: %v\n", err)
	} else {
		fmt.Printf("Snapshot dir (default): %s\n", snapshotDir)
	}

	var leader string

	_, err = instanceCfg.Get(config.NewKeyPath("leader"), &leader)
	if err != nil {
		fmt.Printf("Get leader error: %v\n", err)
	} else {
		fmt.Printf("Leader (inherited from replicaset): %s\n", leader)
	}

	// Output:
	// Failover (default): manual
	// Snapshot dir (default): /default/snapshots
	// Leader (inherited from replicaset): s-001-a
}
