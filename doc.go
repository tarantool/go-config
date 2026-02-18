// Package config provides a uniform way to handle configurations with hierarchical inheritance,
// validation, and flexible merging strategies.
//
// # Key Features
//
//   - Hierarchical Configuration Inheritance: define multi‑level hierarchies
//     (global → group → replicaset → instance) and resolve effective configuration
//     for any leaf entity.
//   - Flexible Merge Strategies: choose how values are inherited: replace (default),
//     append (for slices), or deep merge (for maps).
//   - Fine‑grained Exclusions: exclude specific keys from inheritance, either globally
//     or from certain levels.
//   - Defaults: set default values that apply to every leaf entity unless overridden.
//   - Validation: validate configuration with JSON Schema or custom validators.
//   - Multiple Sources: load configuration from maps, files, environment variables, etc.
//   - Order Preservation: maintain insertion order of keys when needed.
//
// # Quick Example
//
//	b := config.NewBuilder()
//	b = b.AddCollector(collectors.NewMap(map[string]any{
//	    "replication": map[string]any{"failover": "manual"},
//	    "groups": map[string]any{
//	        "storages": map[string]any{
//	            "sharding": map[string]any{"roles": []any{"storage"}},
//	            "replicasets": map[string]any{
//	                "s-001": map[string]any{
//	                    "leader": "s-001-a",
//	                    "instances": map[string]any{
//	                        "s-001-a": map[string]any{
//	                            "iproto": map[string]any{
//	                                "listen": []any{map[string]any{"uri": "127.0.0.1:3301"}},
//	                            },
//	                        },
//	                    },
//	                },
//	            },
//	        },
//	    },
//	}))
//
//	b = b.WithInheritance(
//	    config.Levels(config.Global, "groups", "replicasets", "instances"),
//	)
//
//	cfg, _ := b.Build()
//	instanceCfg, _ := cfg.Effective(config.NewKeyPath("groups/storages/replicasets/s-001/instances/s-001-a"))
//
//	var failover string
//	instanceCfg.Get(config.NewKeyPath("replication/failover"), &failover)
//	fmt.Printf("Failover: %s\n", failover) // "manual" (inherited from global)
//
// For detailed examples see the example_inheritance_test.go, example_merger_test.go, example_validation_test.go files.
//
// The package supports configuration validation through the validator interface.
// See the validators subpackage for JSON Schema validation.
package config
