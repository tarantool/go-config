// Package collectors provides standard implementations of the
// [config.Collector] interface for loading configuration from various sources.
//
// # Collector Types
//
//   - [Map] — reads configuration from an in-memory map[string]any.
//   - [Env] — reads configuration from environment variables, with
//     configurable prefix, delimiter, and key transformation.
//   - [Storage] — reads multiple configuration documents from a key-value
//     storage (e.g., etcd) under a common prefix, with integrity verification.
//   - [Directory] — reads multiple configuration files from a filesystem
//     directory, with optional recursive scanning.
//   - [Source] / [DataSource] — abstraction over a single data stream
//     (file, storage key, etc.) used by the generic collector.
//
// # Format and Watching
//
//   - [Format] — interface for parsing raw data (e.g., YAML) into a tree.Node.
//   - [YamlFormat] — YAML implementation of [Format].
//   - [Watcher] — interface for reactive change notifications from storage
//     backends.
//
// # Builder Pattern
//
// All collectors use a fluent builder pattern with With* methods for optional
// configuration (WithName, WithSourceType, WithRevision, WithKeepOrder).
//
// # Example
//
//	m := collectors.NewMap(map[string]any{
//	    "listen": "127.0.0.1:3301",
//	    "log":    map[string]any{"level": "info"},
//	})
//
//	cfg, err := config.NewBuilder().
//	    AddCollector(m.WithName("defaults")).
//	    Build()
//	if err != nil {
//	    log.Fatal(err)
//	}
package collectors
