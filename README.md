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
- Mutable Configuration: thread-safe runtime modifications with
  validation rollback and stable read snapshots
- Deferred Validation: assemble a configuration without running the
  schema, then validate later once it is complete
- YAML Round-trip: serialize back to YAML preserving key order, scalar
  style, and source comments
- Reactive Watch: monitor storage changes via the Watcher interface
- Custom Mergers: full control over how collector values are merged into
  the configuration tree

### Installation

```bash
go get github.com/tarantool/go-config/v2
```

### Quick Start

#### Basic Configuration from a Map

```go
package main

import (
    "fmt"
    "log"

    "github.com/tarantool/go-config/v2"
    "github.com/tarantool/go-config/v2/collectors"
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

    "github.com/tarantool/go-config/v2"
    "github.com/tarantool/go-config/v2/collectors"
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

    "github.com/tarantool/go-config/v2/tarantool"
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

### Strings and Types

A string is converted to a bool or a number when the destination asks for one:
`Get` into a `bool` parses `"true"`, and a JSON Schema that expects a boolean
accepts it. This is what makes environment variables usable at all, since they
carry nothing but strings.

A string whose type the source fixed is not converted. In YAML that is a
scalar written in quotes, as a block scalar, or with an explicit tag — YAML
resolves `"false"` to a string whatever it says:

```yaml
debug: false     # bool
label: "false"   # string: Get into a bool fails, a boolean schema rejects it
timeout: "5s"    # string, and still a valid time.Duration
```

`Get` then fails with `tree.ErrFixedTypeString`. A value that overrides such a
string brings its own type, so `DEBUG=true` in the environment still sets a
field that the YAML file quoted. Collectors mark these values with
`tree.Node.SetTypeFixed`; a custom collector with typed input can do the same.

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
    config.WithInheritMerge("credentials", config.MergeDeep),
    config.WithNoInherit("leader"),
    config.WithDefaults(map[string]any{
        "replication": map[string]any{"failover": "manual"},
    }),
)
```

### Templates

`WithTemplateVariables` registers a provider of string variables derived from
an entity path. A known variable with an empty value removes its placeholder.
Unknown placeholders are preserved exactly, including their whitespace.

The Tarantool builder registers `instance_name`, `replicaset_name` and
`group_name` automatically. A generic builder can register the same variables:

```go
builder = builder.WithInheritance(
    config.Levels(config.Global, "groups", "replicasets", "instances"),
    config.WithTemplateVariables(tarantool.TemplateVariables),
)
```

`tarantool.TemplateVariables` returns a fresh map for a complete Tarantool instance
path and nil for other path shapes. Additional variables can be registered with
`WithTemplateVariables`. 

Templates in string leaves and mapping keys of the config tree are expanded
once after inheritance, including nested YAML mappings and sequences.
Maps, slices and arrays stored as whole leaf values by Go collectors are left
unchanged. Only ASCII spaces around names are trimmed. Tabs and newlines are part
of the name. Replacement text is never expanded again.
Raw `Get`, `Slice` and `MarshalYAML` retain source templates. `Effective` and
`EffectiveAll` return substituted views, preserving unknown placeholders.
Their YAML output preserves source comments and styles.

### Validation

Configuration can be validated against a JSON Schema or a custom validator
implementing the `validator.Validator` interface.

```go
// JSON Schema validation.
builder, err := builder.WithJSONSchema(schemaReader)

// Custom validator.
builder = builder.WithValidator(myValidator)
```

#### Deferred Validation

By default `Build()` runs the validator before returning. When a configuration
is assembled in stages — for example, a partial bootstrap that gets enriched
from another source later — call `WithoutValidation()` to skip the Build-time
pass while keeping the validator attached, and run it explicitly once the
tree is complete:

```go
cfg, errs := builder.WithoutValidation().Build()
// ... enrich cfg from another source ...
if errs := cfg.Validate(); len(errs) > 0 {
    log.Fatal(errs)
}
```

The same toggle exists on `tarantool.Builder`. Schema-aware env-var routing
still works because the schema is loaded — only the JSON-Schema check on the
merged tree is skipped. `MutableConfig.Validate()` is the equivalent on a
mutable config; runtime mutations via `Set`/`Merge`/`Update`/`Delete` always
validate regardless of this flag.

### Mutable Configuration

`BuildMutable()` returns a `MutableConfig` that allows thread-safe runtime
modifications. Every mutation validates the resulting tree and rolls back to
the previous state on failure, so observers never see a partially-applied
or invalid configuration:

```go
cfg, errs := builder.BuildMutable()

// Set a single value.
err := cfg.Set(config.NewKeyPath("server/port"), 9090)

// Merge another config.
err = cfg.Merge(&otherConfig)

// Update only existing keys.
err = cfg.Update(&patchConfig)

// Remove a key.
removed := cfg.Delete(config.NewKeyPath("server/tls"))
```

Reads (`Get`, `Lookup`, `Stat`, `Walk`, `Slice`, `Effective`, `EffectiveAll`)
take a read lock and are safe to call concurrently with mutations. For a
long-lived reader that needs a stable view across many calls, use
`Snapshot()` to obtain a deep-copied `Config` decoupled from the live tree:

```go
snap := cfg.Snapshot()
// snap is unaffected by subsequent cfg.Set/Merge/Update/Delete calls.
```

### YAML Output

`Config.MarshalYAML()` and `Config.String()` serialize the configuration
back to YAML, preserving the key ordering, scalar quoting style, and inline
and block comments of the source document. This makes the library safe to
use in tooling that reads, edits, and writes back hand-maintained YAML
without churning unrelated formatting:

```go
out, err := cfg.MarshalYAML()
// out == "# original comment is preserved\nserver:\n  port: 9090\n..."
```

### YAML Editor Operations

The `syntax` builder prepares a JSON Schema and a reusable YAML parser.
Completion and Hover operate on the same tolerant syntax tree:

```go
parser, err := syntax.NewBuilder().WithJSONSchema(schemaJSON).Build(ctx)
if err != nil {
    return err
}
defer parser.Close()

tree, err := parser.Parse(ctx, source)
if err != nil {
    return err
}
defer tree.Close()

items, err := tree.Completion(position)
if err != nil {
    return err
}

info, err := tree.Hover(position)
if err != nil {
    return err
}
```

Import `github.com/tarantool/go-config/v2/syntax`.
`Tree.Completion` returns `[]syntax.CompletionItem`.
Schema compilation, CST navigation, and context resolution are private to the package;
editor operations accept `syntax.Position` and return package-owned result types.
`syntax/internal/cst` handles YAML navigation, recovery, and completion assembly;
`syntax/internal/schema` handles schema compilation, traversal, and metadata assembly.
`syntax.Position`, `syntax.Range`, `syntax.CompletionItem`, and `syntax.Metadata`
are aliases for the corresponding internal types.

`Tree.Hover` returns a `syntax.HoverItem` with `Metadata` and `Range`. A zero item with
`Metadata == nil` means there is no tooltip. It describes the selected key or scalar value, including
`oneOf` and `anyOf` alternatives regardless of the current value. Parent
fields show their own documentation without expanding child properties.
Comments, separators, and whitespace between nodes have no tooltip.

`syntax.Metadata` contains title, description, types, enum, const, default, and
deprecation. `AllOf`, `AnyOf`, and `OneOf` retain nested branch metadata;
`AllOf` also includes references and overlapping property patterns.
Empty `AnyOf` and `OneOf` entries represent permitted alternatives without
exposed metadata; they may still constrain values. A schema with no displayable
metadata anywhere in the result produces no tooltip.
`Default` is nil for both absent and null defaults; false, zero, and empty
strings are preserved. `HasConst` distinguishes an explicit null constant
from a missing constant. Values retain their JSON types and can be modified without
changing the compiled schema. Descriptions are returned verbatim.

Positions and ranges use zero-based lines and UTF-8 byte columns. An LSP
adapter formats `Metadata` as Markdown or plain text and converts `Range`
to the client's position encoding. Results remain valid
when the tree is closed. Calls to `Parser.Parse` and `Parser.Close` are
synchronized internally; operations on an individual tree must be serialized.

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
| `Example_mutableConfig` | Runtime modifications via `BuildMutable`, `Set`, `Merge`, `Update` |

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

[godoc-badge]: https://pkg.go.dev/badge/github.com/tarantool/go-config/v2.svg
[godoc-url]: https://pkg.go.dev/github.com/tarantool/go-config/v2
[actions-badge]: https://github.com/tarantool/go-config/v2/actions/workflows/testing.yml/badge.svg
[actions-url]: https://github.com/tarantool/go-config/v2/actions/workflows/testing.yml
[coverage-badge]: https://coveralls.io/repos/github/tarantool/go-config/badge.svg?branch=master
[coverage-url]: https://coveralls.io/github/tarantool/go-config?branch=master
[telegram-badge]: https://img.shields.io/badge/Telegram-join%20chat-blue.svg
[telegram-en-url]: http://telegram.me/tarantool
[telegram-ru-url]: http://telegram.me/tarantoolru
