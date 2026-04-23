package config_test

import (
	"context"
	"fmt"
	"io"

	"github.com/tarantool/go-config"
	"github.com/tarantool/go-config/collectors"
	"github.com/tarantool/go-config/internal/testutil"
)

// Example_storageCollector demonstrates reading multiple configuration
// documents from a key-value storage under a common prefix using the
// Storage collector.
func Example_storageCollector() {
	// Set up in-memory storage with configuration data.
	mock := testutil.NewMockStorage()
	testutil.PutIntegrity(mock, "/config/", "app", []byte("port: 8080\nhost: localhost"))

	// Create an integrity-typed wrapper and the Storage collector.
	typed := testutil.NewRawTyped(mock, "/config/")
	collector := collectors.NewStorage(typed, "/config/", collectors.NewYamlFormat())

	// Build configuration.
	builder := config.NewBuilder()

	builder = builder.AddCollector(collector)

	cfg, errs := builder.Build(context.Background())
	if len(errs) > 0 {
		fmt.Printf("Build errors: %v\n", errs)
		return
	}

	var port int

	_, err := cfg.Get(config.NewKeyPath("port"), &port)
	if err != nil {
		fmt.Printf("Get port error: %v\n", err)
		return
	}

	var host string

	_, err = cfg.Get(config.NewKeyPath("host"), &host)
	if err != nil {
		fmt.Printf("Get host error: %v\n", err)
		return
	}

	fmt.Printf("Host: %s\n", host)
	fmt.Printf("Port: %d\n", port)

	// Output:
	// Host: localhost
	// Port: 8080
}

// Example_storageCollectorMultipleKeys demonstrates reading and merging
// multiple keys from storage into a unified configuration tree. Key names
// are used only for distinguishing documents; the YAML content determines
// the tree structure.
func Example_storageCollectorMultipleKeys() {
	mock := testutil.NewMockStorage()
	testutil.PutIntegrity(mock, "/config/", "cfg-servers",
		[]byte("server:\n  port: 8080\n  host: localhost"))
	testutil.PutIntegrity(mock, "/config/", "cfg-database",
		[]byte("database:\n  driver: postgres\n  port: 5432"))

	typed := testutil.NewRawTyped(mock, "/config/")
	collector := collectors.NewStorage(typed, "/config/", collectors.NewYamlFormat())

	builder := config.NewBuilder()

	builder = builder.AddCollector(collector)

	cfg, errs := builder.Build(context.Background())
	if len(errs) > 0 {
		fmt.Printf("Build errors: %v\n", errs)
		return
	}

	var host string

	_, err := cfg.Get(config.NewKeyPath("server/host"), &host)
	if err != nil {
		fmt.Printf("Get error: %v\n", err)
		return
	}

	fmt.Printf("server host: %s\n", host)

	var driver string

	_, err = cfg.Get(config.NewKeyPath("database/driver"), &driver)
	if err != nil {
		fmt.Printf("Get error: %v\n", err)
		return
	}

	fmt.Printf("database driver: %s\n", driver)

	// Output:
	// server host: localhost
	// database driver: postgres
}

// Example_storageCollectorWithMapOverride demonstrates combining a Storage
// collector with a Map collector, where later collectors override earlier ones.
func Example_storageCollectorWithMapOverride() {
	mock := testutil.NewMockStorage()
	testutil.PutIntegrity(mock, "/config/", "db",
		[]byte("db:\n  host: storage-host\n  port: 5432"))

	typed := testutil.NewRawTyped(mock, "/config/")
	storageCollector := collectors.NewStorage(typed, "/config/", collectors.NewYamlFormat())

	// Map collector provides defaults; storage collector overrides the host.
	mapCollector := collectors.NewMap(map[string]any{
		"db/host": "override-host",
	})

	builder := config.NewBuilder()

	builder = builder.AddCollector(mapCollector)
	builder = builder.AddCollector(storageCollector)

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

	fmt.Printf("Host: %s\n", host)

	// Output:
	// Host: storage-host
}

// Example_storageSource demonstrates using StorageSource as a DataSource
// to read a single configuration document from storage with integrity
// verification.
func Example_storageSource() {
	mock := testutil.NewMockStorage()
	testutil.PutIntegrity(mock, "/config/", "app",
		[]byte("server:\n  port: 8080\n  host: localhost"))

	// Create a StorageSource for a single key.
	source := collectors.NewStorageSource(mock, "/config/", "app", nil, nil)

	// Use it with a format to build a collector.
	collector, err := collectors.NewSource(context.Background(), source, collectors.NewYamlFormat())
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

	var port int

	_, err = cfg.Get(config.NewKeyPath("server/port"), &port)
	if err != nil {
		fmt.Printf("Get error: %v\n", err)
		return
	}

	fmt.Printf("Server port: %d\n", port)

	// Output:
	// Server port: 8080
}

// Example_storageSourceFetchStream demonstrates using StorageSource.FetchStream
// to read raw configuration bytes from storage.
func Example_storageSourceFetchStream() {
	mock := testutil.NewMockStorage()
	testutil.PutIntegrity(mock, "/config/", "app", []byte("key: value"))

	source := collectors.NewStorageSource(mock, "/config/", "app", nil, nil)

	ctx := context.Background()

	reader, err := source.FetchStream(ctx)
	if err != nil {
		fmt.Printf("FetchStream error: %v\n", err)
		return
	}

	defer func() { _ = reader.Close() }()

	data, err := io.ReadAll(reader)
	if err != nil {
		fmt.Printf("ReadAll error: %v\n", err)
		return
	}

	fmt.Printf("Data: %s\n", string(data))
	fmt.Printf("Revision: %s\n", source.Revision())

	// Output:
	// Data: key: value
	// Revision: 1
}
