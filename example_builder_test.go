package config_test

import (
	"context"
	"fmt"
	"strings"

	"github.com/tarantool/go-config"
	"github.com/tarantool/go-config/collectors"
)

// Example_multipleCollectorPriority demonstrates priority-based merging
// across multiple collectors, where later collectors override earlier ones.
func Example_multipleCollectorPriority() {
	// First collector: defaults (lowest priority).
	defaults := collectors.NewMap(map[string]any{
		"server": map[string]any{
			"host":    "0.0.0.0",
			"port":    8080,
			"timeout": 30,
		},
		"log_level": "info",
	}).WithName("defaults")

	// Second collector: environment-specific overrides.
	envOverrides := collectors.NewMap(map[string]any{
		"server": map[string]any{
			"host": "prod.example.com",
			"port": 443,
		},
		"log_level": "warn",
	}).WithName("production")

	// Third collector: local overrides (highest priority).
	localOverrides := collectors.NewMap(map[string]any{
		"log_level": "debug",
	}).WithName("local")

	// Build with collectors in priority order (first = lowest, last = highest).
	builder := config.NewBuilder()

	builder = builder.AddCollector(defaults)
	builder = builder.AddCollector(envOverrides)
	builder = builder.AddCollector(localOverrides)

	cfg, errs := builder.Build(context.Background())
	if len(errs) > 0 {
		fmt.Printf("Build errors: %v\n", errs)
		return
	}

	// "host" and "port" overridden by production, "timeout" from defaults.
	var host string

	_, _ = cfg.Get(config.NewKeyPath("server/host"), &host)
	fmt.Printf("Host: %s\n", host)

	var port int

	_, _ = cfg.Get(config.NewKeyPath("server/port"), &port)
	fmt.Printf("Port: %d\n", port)

	var timeout int

	_, _ = cfg.Get(config.NewKeyPath("server/timeout"), &timeout)
	fmt.Printf("Timeout: %d\n", timeout)

	// "log_level" overridden by local (highest priority).
	var logLevel string

	meta, _ := cfg.Get(config.NewKeyPath("log_level"), &logLevel)
	fmt.Printf("Log level: %s (from %s)\n", logLevel, meta.Source.Name)

	// Output:
	// Host: prod.example.com
	// Port: 443
	// Timeout: 30
	// Log level: debug (from local)
}

// Example_withJSONSchema demonstrates using Builder.WithJSONSchema(reader)
// and Builder.MustWithJSONSchema(reader) convenience methods for schema
// validation, as an alternative to manually creating a validator.
func Example_withJSONSchema() {
	schema := `{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"type": "object",
		"properties": {
			"server": {
				"type": "object",
				"properties": {
					"port": {
						"type": "integer",
						"minimum": 1024,
						"maximum": 65535
					},
					"host": {
						"type": "string"
					}
				},
				"required": ["port", "host"]
			}
		},
		"additionalProperties": false
	}`

	// Using WithJSONSchema with a reader.
	validData := map[string]any{
		"server": map[string]any{
			"port": 8080,
			"host": "localhost",
		},
	}

	builder := config.NewBuilder()

	builder, err := builder.WithJSONSchema(strings.NewReader(schema))
	if err != nil {
		fmt.Printf("Schema error: %v\n", err)
		return
	}

	builder = builder.AddCollector(collectors.NewMap(validData).WithName("valid"))

	cfg, errs := builder.Build(context.Background())
	if len(errs) > 0 {
		fmt.Printf("Validation errors: %v\n", errs)
		return
	}

	var port int

	_, _ = cfg.Get(config.NewKeyPath("server/port"), &port)
	fmt.Printf("Valid config port: %d\n", port)

	// Using MustWithJSONSchema (panics on invalid schema).
	invalidData := map[string]any{
		"server": map[string]any{
			"port": 80, // Below minimum of 1024.
			"host": "localhost",
		},
	}

	builder = config.NewBuilder()
	builder = builder.MustWithJSONSchema(strings.NewReader(schema))
	builder = builder.AddCollector(collectors.NewMap(invalidData).WithName("invalid"))

	_, errs = builder.Build(context.Background())
	fmt.Printf("Validation failed: %v\n", len(errs) > 0)

	// Output:
	// Valid config port: 8080
	// Validation failed: true
}
