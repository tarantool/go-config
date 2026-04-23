package tarantool_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tarantool/go-config"
	"github.com/tarantool/go-config/tarantool"
)

func ExampleBuilder_WithConfigFile() {
	cfgPath, cleanup := writeExampleFile(`
log:
  level: info

replication:
  failover: election

groups:
  storages:
    replicasets:
      s-001:
        instances:
          s-001-a:
            iproto:
              listen: 3301
`)
	defer cleanup()

	cfg, err := tarantool.New().
		WithConfigFile(cfgPath).
		WithEnvPrefix("TT_EXAMPLE_").
		WithoutSchema().
		Build(context.Background())
	if err != nil {
		fmt.Printf("Build error: %v\n", err)
		return
	}

	instanceCfg, err := cfg.Effective(
		config.NewKeyPath("groups/storages/replicasets/s-001/instances/s-001-a"))
	if err != nil {
		fmt.Printf("Effective error: %v\n", err)
		return
	}

	var listen string

	_, err = instanceCfg.Get(config.NewKeyPath("iproto/listen"), &listen)
	if err != nil {
		fmt.Printf("Get listen error: %v\n", err)
		return
	}

	var failover string

	_, err = instanceCfg.Get(config.NewKeyPath("replication/failover"), &failover)
	if err != nil {
		fmt.Printf("Get failover error: %v\n", err)
		return
	}

	fmt.Printf("Listen: %s\n", listen)
	fmt.Printf("Failover: %s\n", failover)

	// Output:
	// Listen: 3301
	// Failover: election
}

func ExampleBuilder_WithSchemaVersion() {
	err := tarantool.RegisterSchema("99.30.0", []byte(`{
		"$schema":"https://json-schema.org/draft/2020-12/schema",
		"type":"object",
		"properties":{
			"app":{
				"type":"object",
				"properties":{
					"name":{"type":"string"}
				},
				"required":["name"],
				"additionalProperties":false
			}
		},
		"required":["app"],
		"additionalProperties":false
	}`))
	if err != nil {
		fmt.Printf("RegisterSchema error: %v\n", err)
		return
	}

	cfgPath, cleanup := writeExampleFile("app:\n  name: router\n")
	defer cleanup()

	cfg, err := tarantool.New().
		WithConfigFile(cfgPath).
		WithSchemaVersion("99.30.0").
		WithEnvPrefix("TT_EXAMPLE_").
		Build(context.Background())
	if err != nil {
		fmt.Printf("Build error: %v\n", err)
		return
	}

	var name string

	_, err = cfg.Get(config.NewKeyPath("app/name"), &name)
	if err != nil {
		fmt.Printf("Get error: %v\n", err)
		return
	}

	fmt.Printf("App: %s\n", name)

	// Output:
	// App: router
}

func ExampleBuilder_WithoutSchema() {
	cfgPath, cleanup := writeExampleFile(`
custom:
  feature_flag: true
`)
	defer cleanup()

	cfg, err := tarantool.New().
		WithConfigFile(cfgPath).
		WithoutSchema().
		WithEnvPrefix("TT_EXAMPLE_").
		Build(context.Background())
	if err != nil {
		fmt.Printf("Build error: %v\n", err)
		return
	}

	var flag string

	_, err = cfg.Get(config.NewKeyPath("custom/feature_flag"), &flag)
	if err != nil {
		fmt.Printf("Get error: %v\n", err)
		return
	}

	fmt.Printf("Feature flag: %s\n", flag)

	// Output:
	// Feature flag: true
}

func writeExampleFile(contents string) (string, func()) {
	dir, err := os.MkdirTemp("", "go-config-tarantool-example-*")
	if err != nil {
		panic(err)
	}

	cfgPath := filepath.Join(dir, "config.yaml")

	err = os.WriteFile(cfgPath, []byte(contents), 0o600)
	if err != nil {
		_ = os.RemoveAll(dir)

		panic(err)
	}

	return cfgPath, func() { _ = os.RemoveAll(dir) }
}
