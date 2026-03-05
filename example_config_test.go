package config_test

import (
	"context"
	"fmt"
	"sort"

	"github.com/tarantool/go-config"
	"github.com/tarantool/go-config/collectors"
)

// Example_basicGetAndLookup demonstrates the core Config API methods:
// Get (extracts a typed value), Lookup (returns a Value without error on miss),
// and Stat (returns only metadata without touching the value).
func Example_basicGetAndLookup() {
	data := map[string]any{
		"server": map[string]any{
			"host": "localhost",
			"port": 8080,
		},
	}

	builder := config.NewBuilder()

	builder = builder.AddCollector(
		collectors.NewMap(data).
			WithName("app-config").
			WithSourceType(config.FileSource).
			WithRevision("v1.0"),
	)

	cfg, errs := builder.Build(context.Background())
	if len(errs) > 0 {
		fmt.Printf("Build errors: %v\n", errs)
		return
	}

	// Get extracts a typed value and returns metadata.
	var host string

	meta, err := cfg.Get(config.NewKeyPath("server/host"), &host)
	if err != nil {
		fmt.Printf("Get error: %v\n", err)
		return
	}

	fmt.Printf("Host: %s\n", host)
	fmt.Printf("Source: %s\n", meta.Source.Name)
	fmt.Printf("Revision: %s\n", meta.Revision)

	// Get returns an error for missing keys.
	var missing string

	_, err = cfg.Get(config.NewKeyPath("server/timeout"), &missing)
	fmt.Printf("Missing key error: %v\n", err)

	// Lookup returns (Value, bool) — no error on miss.
	val, ok := cfg.Lookup(config.NewKeyPath("server/port"))
	fmt.Printf("Port found: %v\n", ok)

	if ok {
		var port int

		_ = val.Get(&port)
		fmt.Printf("Port: %d\n", port)
	}

	_, ok = cfg.Lookup(config.NewKeyPath("server/missing"))
	fmt.Printf("Missing found: %v\n", ok)

	// Stat returns metadata without extracting the value.
	statMeta, ok := cfg.Stat(config.NewKeyPath("server/host"))
	fmt.Printf("Stat found: %v\n", ok)
	fmt.Printf("Stat source: %s\n", statMeta.Source.Name)

	// Output:
	// Host: localhost
	// Source: app-config
	// Revision: v1.0
	// Missing key error: key not found: server/timeout
	// Port found: true
	// Port: 8080
	// Missing found: false
	// Stat found: true
	// Stat source: app-config
}

// Example_walkConfig demonstrates Config.Walk() for iterating over all
// leaf values in the configuration tree with optional depth control.
func Example_walkConfig() {
	data := map[string]any{
		"database": map[string]any{
			"host": "localhost",
			"port": 5432,
			"pool": map[string]any{
				"max_size": 10,
			},
		},
	}

	builder := config.NewBuilder()

	builder = builder.AddCollector(collectors.NewMap(data).WithName("config"))

	cfg, errs := builder.Build(context.Background())
	if len(errs) > 0 {
		fmt.Printf("Build errors: %v\n", errs)
		return
	}

	// Walk all leaf values from the root (depth -1 means unlimited).
	ctx := context.Background()

	valueCh, err := cfg.Walk(ctx, config.NewKeyPath(""), -1)
	if err != nil {
		fmt.Printf("Walk error: %v\n", err)
		return
	}

	allKeys := make([]string, 0, 3)
	for val := range valueCh {
		allKeys = append(allKeys, val.Meta().Key.String())
	}

	sort.Strings(allKeys)

	fmt.Printf("All keys: %v\n", allKeys)

	// Walk from a sub-path to iterate only within "database".
	valueCh, err = cfg.Walk(ctx, config.NewKeyPath("database"), -1)
	if err != nil {
		fmt.Printf("Walk error: %v\n", err)
		return
	}

	subKeys := make([]string, 0, 3)
	for val := range valueCh {
		subKeys = append(subKeys, val.Meta().Key.String())
	}

	sort.Strings(subKeys)

	fmt.Printf("Database keys: %v\n", subKeys)

	// Walk with depth=2 limits traversal depth (stops before reaching pool/max_size).
	valueCh, err = cfg.Walk(ctx, config.NewKeyPath("database"), 2)
	if err != nil {
		fmt.Printf("Walk error: %v\n", err)
		return
	}

	shallowKeys := make([]string, 0, 2)
	for val := range valueCh {
		shallowKeys = append(shallowKeys, val.Meta().Key.String())
	}

	sort.Strings(shallowKeys)

	fmt.Printf("Shallow keys (depth=2): %v\n", shallowKeys)

	// Output:
	// All keys: [database/host database/pool/max_size database/port]
	// Database keys: [database/host database/pool/max_size database/port]
	// Shallow keys (depth=2): [database/host database/port]
}

// Example_sliceConfig demonstrates Config.Slice() for extracting
// a sub-configuration as a separate Config object.
func Example_sliceConfig() {
	data := map[string]any{
		"server": map[string]any{
			"http": map[string]any{
				"port": 8080,
				"host": "0.0.0.0",
			},
			"grpc": map[string]any{
				"port": 9090,
			},
		},
	}

	builder := config.NewBuilder()

	builder = builder.AddCollector(collectors.NewMap(data).WithName("config"))

	cfg, errs := builder.Build(context.Background())
	if len(errs) > 0 {
		fmt.Printf("Build errors: %v\n", errs)
		return
	}

	// Slice extracts "server/http" as a standalone Config.
	httpCfg, err := cfg.Slice(config.NewKeyPath("server/http"))
	if err != nil {
		fmt.Printf("Slice error: %v\n", err)
		return
	}

	// Access values relative to the sliced root.
	var port int

	_, err = httpCfg.Get(config.NewKeyPath("port"), &port)
	if err != nil {
		fmt.Printf("Get error: %v\n", err)
		return
	}

	fmt.Printf("HTTP port: %d\n", port)

	var host string

	_, err = httpCfg.Get(config.NewKeyPath("host"), &host)
	if err != nil {
		fmt.Printf("Get error: %v\n", err)
		return
	}

	fmt.Printf("HTTP host: %s\n", host)

	// Slice returns an error for non-existent paths.
	_, err = cfg.Slice(config.NewKeyPath("nonexistent"))
	fmt.Printf("Nonexistent slice error: %v\n", err)

	// Output:
	// HTTP port: 8080
	// HTTP host: 0.0.0.0
	// Nonexistent slice error: path not found: nonexistent
}

// Example_effectiveAll demonstrates Config.EffectiveAll() which resolves
// effective configurations for ALL leaf entities in the hierarchy at once.
func Example_effectiveAll() {
	data := map[string]any{
		"replication": map[string]any{"failover": "manual"},
		"groups": map[string]any{
			"storages": map[string]any{
				"sharding": map[string]any{"roles": []any{"storage"}},
				"replicasets": map[string]any{
					"s-001": map[string]any{
						"leader": "s-001-a",
						"instances": map[string]any{
							"s-001-a": map[string]any{
								"iproto": map[string]any{"listen": "127.0.0.1:3301"},
							},
							"s-001-b": map[string]any{
								"iproto": map[string]any{"listen": "127.0.0.1:3302"},
							},
						},
					},
				},
			},
		},
	}

	builder := config.NewBuilder()

	builder = builder.AddCollector(collectors.NewMap(data).WithName("config"))

	builder = builder.WithInheritance(
		config.Levels(config.Global, "groups", "replicasets", "instances"),
	)

	cfg, errs := builder.Build(context.Background())
	if len(errs) > 0 {
		fmt.Printf("Build errors: %v\n", errs)
		return
	}

	// EffectiveAll resolves all leaf entities at once.
	allConfigs, err := cfg.EffectiveAll()
	if err != nil {
		fmt.Printf("EffectiveAll error: %v\n", err)
		return
	}

	// Sort keys for stable output.
	keys := make([]string, 0, len(allConfigs))
	for k := range allConfigs {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	for _, key := range keys {
		instanceCfg := allConfigs[key]

		var listen string

		_, err := instanceCfg.Get(config.NewKeyPath("iproto/listen"), &listen)
		if err != nil {
			fmt.Printf("Get error: %v\n", err)
			continue
		}

		var failover string

		_, err = instanceCfg.Get(config.NewKeyPath("replication/failover"), &failover)
		if err != nil {
			fmt.Printf("Get error: %v\n", err)
			continue
		}

		fmt.Printf("%s: listen=%s failover=%s\n", key, listen, failover)
	}

	// Output:
	// groups/storages/replicasets/s-001/instances/s-001-a: listen=127.0.0.1:3301 failover=manual
	// groups/storages/replicasets/s-001/instances/s-001-b: listen=127.0.0.1:3302 failover=manual
}
