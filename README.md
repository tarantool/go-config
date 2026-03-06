[![Go Reference][godoc-badge]][godoc-url]
[![Actions Status][actions-badge]][actions-url]
[![Code Coverage][coverage-badge]][coverage-url]
[![Telegram EN][telegram-badge]][telegram-en-url]
[![Telegram RU][telegram-badge]][telegram-ru-url]

# go-config: library to manage hierarchical configurations

### About

<a href="http://tarantool.org">
    <img align="right" src="assets/logo.png" width="250" alt="go-config logo">
</a>

**go-config** is a Go library that provides a uniform way to handle
configurations with hierarchical inheritance, validation, and flexible merging
strategies. It supports multiple data sources, key ordering preservation, and
runtime modifications.

### Overview

The library is designed for distributed systems where configuration often
follows a hierarchical structure (e.g., global → group → replicaset →
instance). It assembles configuration from multiple sources with priority-based
merging and resolves effective configuration for any entity in the hierarchy.

### Features

- Hierarchical Configuration Inheritance: define multi-level hierarchies
  and resolve effective configuration for any leaf entity
- Flexible Merge Strategies: choose how values are inherited — replace
  (default), append (for slices), or deep merge (for maps)
- Fine-grained Exclusions: exclude specific keys from inheritance, either
  globally or from certain levels
- Defaults: set default values that apply to every leaf entity unless
  overridden
- JSON Schema Validation: validate configuration against JSON Schema or
  custom validators
- Multiple Sources: load configuration from maps, files, directories,
  environment variables, or centralized key-value storages (etcd, TCS)
- Order Preservation: maintain insertion order of keys when needed
- Reactive Watch: monitor storage changes via the Watcher interface
- Custom Mergers: full control over how collector values are merged into
  the configuration tree

### Installation

```bash
go get github.com/tarantool/go-config
```

### Quick Start

#### Basic Configuration from a Map

```go
package main

import (
    "fmt"
    "log"

    "github.com/tarantool/go-config"
    "github.com/tarantool/go-config/collectors"
)

func main() {
    builder := config.NewBuilder()

    builder = builder.AddCollector(collectors.NewMap(map[string]any{
        "server": map[string]any{
            "host": "localhost",
            "port": 8080,
        },
        "database": map[string]any{
            "driver": "postgres",
            "port":   5432,
        },
    }).WithName("defaults"))

    cfg, errs := builder.Build()
    if len(errs) > 0 {
        log.Fatal(errs)
    }

    var host string
    _, _ = cfg.Get(config.NewKeyPath("server/host"), &host)
    fmt.Printf("Host: %s\n", host) // "localhost"

    var port int
    _, _ = cfg.Get(config.NewKeyPath("server/port"), &port)
    fmt.Printf("Port: %d\n", port) // 8080
}
```

#### Hierarchical Inheritance

```go
package main

import (
    "fmt"
    "log"

    "github.com/tarantool/go-config"
    "github.com/tarantool/go-config/collectors"
)

func main() {
    builder := config.NewBuilder()

    builder = builder.AddCollector(collectors.NewMap(map[string]any{
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
        log.Fatal(errs)
    }

    // Resolve effective config for a specific instance.
    instanceCfg, err := cfg.Effective(
        config.NewKeyPath("groups/storages/replicasets/s-001/instances/s-001-a"),
    )
    if err != nil {
        log.Fatal(err)
    }

    var failover string
    _, _ = instanceCfg.Get(config.NewKeyPath("replication/failover"), &failover)
    fmt.Printf("Failover: %s\n", failover) // "manual" (inherited from global)

    var roles []string
    _, _ = instanceCfg.Get(config.NewKeyPath("sharding/roles"), &roles)
    fmt.Printf("Roles: %v\n", roles) // [storage] (inherited from group)
}
```

#### Tarantool Builder

The `tarantool` package provides a high-level builder with Tarantool defaults
(env prefix, inheritance rules, schema validation):

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/tarantool/go-config/tarantool"
)

func main() {
    cfg, err := tarantool.New().
        WithConfigFile("/etc/tarantool/config.yaml").
        WithoutSchema().
        Build(context.Background())
    if err != nil {
        log.Fatal(err)
    }

    var failover string
    instanceCfg, _ := cfg.Effective(
        config.NewKeyPath("groups/storages/replicasets/s-001/instances/s-001-a"),
    )
    _, _ = instanceCfg.Get(config.NewKeyPath("replication/failover"), &failover)
    fmt.Printf("Failover: %s\n", failover)
}
```

### Collectors

Collectors are pluggable data sources. Each implements the `Collector` interface
and streams configuration values via a channel.

#### Map Collector
Reads configuration from an in-memory `map[string]any`. Useful for defaults
and tests.

#### File / Source Collector
Reads configuration from a single file (e.g., YAML) using the `DataSource` and
`Format` interfaces.

#### Directory Collector
Reads all matching files from a directory (e.g., `*.yaml`). Each file is merged
independently as a sub-collector. Supports recursive scanning.

#### Env Collector
Reads configuration from environment variables with a configurable prefix and
key transformation.

#### Storage Collector
Reads multiple configuration documents from a centralized key-value storage
(etcd, TCS) under a common prefix with integrity verification via
[go-storage](https://github.com/tarantool/go-storage).

### Inheritance

Inheritance resolves effective configuration for leaf entities by merging
values from all hierarchy levels (e.g., global → group → replicaset →
instance). It supports:

- **Merge Strategies**: `MergeReplace` (default), `MergeAppend` (for slices),
  `MergeDeep` (for maps)
- **Exclusions**: `WithNoInherit` excludes keys entirely; `WithNoInheritFrom`
  excludes keys from specific levels
- **Defaults**: `WithDefaults` applies default values with the lowest priority

```go
builder = builder.WithInheritance(
    config.Levels(config.Global, "groups", "replicasets", "instances"),
    config.WithInheritMerge("roles", config.MergeAppend),
    config.WithInheritMerge("credentials", config.MergeDeep),
    config.WithNoInherit("leader"),
    config.WithDefaults(map[string]any{
        "replication": map[string]any{"failover": "manual"},
    }),
)
```

### Validation

Configuration can be validated against a JSON Schema or a custom validator
implementing the `validator.Validator` interface.

```go
// JSON Schema validation.
builder, err := builder.WithJSONSchema(schemaReader)

// Custom validator.
builder = builder.WithValidator(myValidator)
```

### Examples

Runnable examples are available in the root package as `Example_*` test
functions. Run them all with `go test -v -run Example ./...`.

#### Config API — [`example_config_test.go`](example_config_test.go)

| Example | Description |
|---------|-------------|
| `Example_basicGetAndLookup` | Core retrieval methods: `Get`, `Lookup`, and `Stat` |
| `Example_walkConfig` | Iterating leaf values with `Walk`, depth control, and sub-paths |
| `Example_sliceConfig` | Extracting a sub-configuration with `Slice` |
| `Example_effectiveAll` | Resolving all leaf entities at once with `EffectiveAll` |
#### Collectors — [`example_collectors_test.go`](example_collectors_test.go)

| Example | Description |
|---------|-------------|
| `Example_envCollector` | Environment variables with prefix, delimiter, and custom transform |
| `Example_directoryCollector` | Reading YAML files from a directory, with recursive scanning |
| `Example_fileSource` | Single-file reading via `NewFile` + `NewSource` |
| `Example_storageCollector` | Key-value storage under a common prefix |
| `Example_storageCollectorMultipleKeys` | Merging multiple storage keys |
| `Example_storageCollectorWithMapOverride` | Combining storage and map collectors |
| `Example_storageSource` | Using `StorageSource` as a `DataSource` |
| `Example_storageSourceFetchStream` | Reading raw bytes from storage |

#### Builder — [`example_builder_test.go`](example_builder_test.go)

| Example | Description |
|---------|-------------|
| `Example_multipleCollectorPriority` | Priority-based merging across multiple collectors |
| `Example_withJSONSchema` | `WithJSONSchema` and `MustWithJSONSchema` convenience APIs |

#### Inheritance — [`example_inheritance_test.go`](example_inheritance_test.go)

| Example | Description |
|---------|-------------|
| `Example_inheritanceBasic` | Hierarchical inheritance (global → group → replicaset → instance) |
| `Example_inheritanceMergeStrategies` | `MergeReplace`, `MergeAppend`, and `MergeDeep` strategies |
| `Example_inheritanceExclusions` | `WithNoInherit` and `WithNoInheritFrom` exclusions |
| `Example_inheritanceDefaults` | Default values via `WithDefaults` |

#### Custom Mergers — [`example_merger_test.go`](example_merger_test.go)

| Example | Description |
|---------|-------------|
| `Example_validatingMerger` | Validating values before merging |
| `Example_transformingMerger` | Transforming values based on path |
| `Example_loggingMerger` | Logging all merge operations |
| `Example_sourceBasedMerger` | Filtering by collector source |

#### Validation — [`example_validation_test.go`](example_validation_test.go)

| Example | Description |
|---------|-------------|
| `Example_validation` | JSON Schema validation |
| `Example_customValidator` | Custom validator enforcing business rules |

#### Storage — [`example_storage_test.go`](example_storage_test.go)

See the Collectors table above for storage-related examples.

### Contributing

Contributions are welcome! Please open an issue to discuss your ideas or submit
a pull request.

### License

This project is licensed under the BSD 2-Clause License – see the
[LICENSE](LICENSE) file for details.

[godoc-badge]: https://pkg.go.dev/badge/github.com/tarantool/go-config.svg
[godoc-url]: https://pkg.go.dev/github.com/tarantool/go-config
[actions-badge]: https://github.com/tarantool/go-config/actions/workflows/testing.yml/badge.svg
[actions-url]: https://github.com/tarantool/go-config/actions/workflows/testing.yml
[coverage-badge]: https://coveralls.io/repos/github/tarantool/go-config/badge.svg?branch=master
[coverage-url]: https://coveralls.io/github/tarantool/go-config?branch=master
[telegram-badge]: https://img.shields.io/badge/Telegram-join%20chat-blue.svg
[telegram-en-url]: http://telegram.me/tarantool
[telegram-ru-url]: http://telegram.me/tarantoolru
