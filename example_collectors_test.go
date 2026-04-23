package config_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tarantool/go-config"
	"github.com/tarantool/go-config/collectors"
)

// Example_envCollector demonstrates the Env collector which reads
// configuration from environment variables. It supports prefix filtering,
// custom delimiters, and custom key transformation functions.
func Example_envCollector() {
	// Set environment variables with a unique prefix.
	_ = os.Setenv("EXAMPLEAPP_DB_HOST", "localhost")
	_ = os.Setenv("EXAMPLEAPP_DB_PORT", "5432")

	defer func() { _ = os.Unsetenv("EXAMPLEAPP_DB_HOST") }()
	defer func() { _ = os.Unsetenv("EXAMPLEAPP_DB_PORT") }()

	// Basic usage: prefix strips "EXAMPLEAPP_", underscore splits into hierarchy.
	envCollector := collectors.NewEnv().
		WithPrefix("EXAMPLEAPP_").
		WithDelimiter("_").
		WithName("env")

	builder := config.NewBuilder()

	builder = builder.AddCollector(envCollector)

	cfg, errs := builder.Build(context.Background())
	if len(errs) > 0 {
		fmt.Printf("Build errors: %v\n", errs)
		return
	}

	var host string

	_, err := cfg.Get(config.NewKeyPath("db/host"), &host)
	if err != nil {
		fmt.Printf("Get error: %v\n", err)
		return
	}

	fmt.Printf("DB host: %s\n", host)

	var port string

	_, err = cfg.Get(config.NewKeyPath("db/port"), &port)
	if err != nil {
		fmt.Printf("Get error: %v\n", err)
		return
	}

	fmt.Printf("DB port: %s\n", port)

	// Custom transform: convert env var names to a custom key path.
	_ = os.Setenv("EXAMPLEAPP_SERVER__HOST", "0.0.0.0")

	defer func() { _ = os.Unsetenv("EXAMPLEAPP_SERVER__HOST") }()

	transformCollector := collectors.NewEnv().
		WithPrefix("EXAMPLEAPP_SERVER__").
		WithTransform(func(key string) config.KeyPath {
			// Use double underscore as separator, preserve case.
			parts := strings.Split(strings.ToLower(key), "__")
			return config.NewKeyPathFromSegments(parts)
		}).
		WithName("env-transform")

	builder = config.NewBuilder()

	builder = builder.AddCollector(transformCollector)

	cfg, errs = builder.Build(context.Background())
	if len(errs) > 0 {
		fmt.Printf("Build errors: %v\n", errs)
		return
	}

	var serverHost string

	_, err = cfg.Get(config.NewKeyPath("host"), &serverHost)
	if err != nil {
		fmt.Printf("Get error: %v\n", err)
		return
	}

	fmt.Printf("Server host: %s\n", serverHost)

	// Output:
	// DB host: localhost
	// DB port: 5432
	// Server host: 0.0.0.0
}

// Example_directoryCollector demonstrates the Directory collector which reads
// all configuration files with a given extension from a directory and merges
// them into a unified configuration tree.
func Example_directoryCollector() {
	// Create a temporary directory with configuration files.
	dir, err := os.MkdirTemp("", "go-config-example-*")
	if err != nil {
		fmt.Printf("MkdirTemp error: %v\n", err)
		return
	}

	defer func() { _ = os.RemoveAll(dir) }()

	// Write YAML configuration files.
	err = os.WriteFile(filepath.Join(dir, "app.yaml"),
		[]byte("app:\n  name: myservice\n  port: 8080\n"), 0o600)
	if err != nil {
		fmt.Printf("WriteFile error: %v\n", err)
		return
	}

	err = os.WriteFile(filepath.Join(dir, "db.yaml"),
		[]byte("database:\n  host: postgres\n  port: 5432\n"), 0o600)
	if err != nil {
		fmt.Printf("WriteFile error: %v\n", err)
		return
	}

	// Create a subdirectory with another config file.
	subdir := filepath.Join(dir, "extra")

	err = os.MkdirAll(subdir, 0o750)
	if err != nil {
		fmt.Printf("MkdirAll error: %v\n", err)
		return
	}

	err = os.WriteFile(filepath.Join(subdir, "cache.yaml"),
		[]byte("cache:\n  ttl: 300\n"), 0o600)
	if err != nil {
		fmt.Printf("WriteFile error: %v\n", err)
		return
	}

	// Read only top-level files (non-recursive).
	collector := collectors.NewDirectory(dir, ".yaml", collectors.NewYamlFormat()).
		WithName("config")

	builder := config.NewBuilder()

	builder = builder.AddCollector(collector)

	cfg, errs := builder.Build(context.Background())
	if len(errs) > 0 {
		fmt.Printf("Build errors: %v\n", errs)
		return
	}

	var appName string

	_, err = cfg.Get(config.NewKeyPath("app/name"), &appName)
	if err != nil {
		fmt.Printf("Get error: %v\n", err)
		return
	}

	fmt.Printf("App: %s\n", appName)

	var dbHost string

	_, err = cfg.Get(config.NewKeyPath("database/host"), &dbHost)
	if err != nil {
		fmt.Printf("Get error: %v\n", err)
		return
	}

	fmt.Printf("DB host: %s\n", dbHost)

	// Subdirectory files are not read without recursive mode.
	_, ok := cfg.Lookup(config.NewKeyPath("cache/ttl"))
	fmt.Printf("Cache TTL found (non-recursive): %v\n", ok)

	// Enable recursive scanning to include subdirectories.
	recursiveCollector := collectors.NewDirectory(dir, ".yaml", collectors.NewYamlFormat()).
		WithRecursive(true)

	builder = config.NewBuilder()

	builder = builder.AddCollector(recursiveCollector)

	cfg, errs = builder.Build(context.Background())
	if len(errs) > 0 {
		fmt.Printf("Build errors: %v\n", errs)
		return
	}

	var ttl int

	_, err = cfg.Get(config.NewKeyPath("cache/ttl"), &ttl)
	if err != nil {
		fmt.Printf("Get error: %v\n", err)
		return
	}

	fmt.Printf("Cache TTL (recursive): %d\n", ttl)

	// Output:
	// App: myservice
	// DB host: postgres
	// Cache TTL found (non-recursive): false
	// Cache TTL (recursive): 300
}

// Example_fileSource demonstrates reading configuration from a single YAML
// file using collectors.NewFile() as a DataSource with collectors.NewSource().
func Example_fileSource() {
	// Create a temporary YAML file.
	dir, err := os.MkdirTemp("", "go-config-file-example-*")
	if err != nil {
		fmt.Printf("MkdirTemp error: %v\n", err)
		return
	}

	defer func() { _ = os.RemoveAll(dir) }()

	filePath := filepath.Join(dir, "config.yaml")

	err = os.WriteFile(filePath,
		[]byte("server:\n  host: localhost\n  port: 8080\nlog_level: info\n"), 0o600)
	if err != nil {
		fmt.Printf("WriteFile error: %v\n", err)
		return
	}

	// Create a File data source and wrap it with a YAML format.
	file := collectors.NewFile(filePath)

	collector, err := collectors.NewSource(context.Background(), file, collectors.NewYamlFormat())
	if err != nil {
		fmt.Printf("NewSource error: %v\n", err)
		return
	}

	builder := config.NewBuilder()

	builder = builder.AddCollector(collector)

	cfg, errs := builder.Build(context.Background())
	if len(errs) > 0 {
		fmt.Printf("Build errors: %v\n", errs)
		return
	}

	var host string

	_, err = cfg.Get(config.NewKeyPath("server/host"), &host)
	if err != nil {
		fmt.Printf("Get error: %v\n", err)
		return
	}

	fmt.Printf("Host: %s\n", host)

	var port int

	_, err = cfg.Get(config.NewKeyPath("server/port"), &port)
	if err != nil {
		fmt.Printf("Get error: %v\n", err)
		return
	}

	fmt.Printf("Port: %d\n", port)

	var logLevel string

	_, err = cfg.Get(config.NewKeyPath("log_level"), &logLevel)
	if err != nil {
		fmt.Printf("Get error: %v\n", err)
		return
	}

	fmt.Printf("Log level: %s\n", logLevel)

	// Output:
	// Host: localhost
	// Port: 8080
	// Log level: info
}
