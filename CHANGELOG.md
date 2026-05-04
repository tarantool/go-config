# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/)
and this project adheres to [Semantic
Versioning](http://semver.org/spec/v2.0.0.html) except to the first release.

## [Unreleased]

### Added

* Thread-safe mutable configuration API on `MutableConfig`: `Set`, `Merge`,
  `Update`, `Delete` validate the resulting tree and roll back to the previous
  state on failure, and read methods (`Get`, `Lookup`, `Stat`, `Walk`, `Slice`,
  `Effective`, `EffectiveAll`) take a read lock. Modified keys get a bumped
  revision and `"modified"` source.
* `MutableConfig.Snapshot()` returns a deep-copied read-only `Config` decoupled
  from the live tree, so a long-lived reader can keep a stable view while other
  goroutines continue mutating the configuration.
* `Builder.WithoutValidation()` skips the Build-time validation pass while
  retaining the configured validator: `BuildMutable` still attaches it to the
  resulting `MutableConfig` so runtime mutations remain validated.
* `Config.Validate()` runs the validator carried over from the `Builder` on
  the current tree, so callers who used `Builder.WithoutValidation()` can
  validate the assembled config later (e.g. after enriching it from another
  source). `MutableConfig.Validate()` is the read-lock-protected counterpart.
  Sub-configs from `Slice`/`Effective` do not carry the validator (the
  schema describes the full root, not subtrees).
* `tarantool.Builder.WithoutValidation()` loads the schema for env-path
  resolution but skips JSON-Schema validation at Build time, enabling
  schema-aware env-var routing for intentionally-partial configs.

### Changed

### Fixed

* Bump `google.golang.org/grpc` to v1.79.3 to fix GO-2026-4762
  (authorization bypass via missing leading slash in `:path`).
* Bump `go.opentelemetry.io/otel/sdk` to v1.40.0 to fix GO-2026-4394
  (arbitrary code execution via PATH hijacking).

## [v1.1.0] - 2026-04-29

This release ships offline JSON Schema validation by default with embedded
schemas for Tarantool 3.3.0 – 3.7.0, makes the Storage collector strict about
parse errors, and includes fixes for env-var resolution, tree merging, and
nil-input handling on the builder. `collectors.NewSource` and
`tarantool.New()` defaults change in backward-incompatible ways — see below.

### Added

* Offline JSON Schema validation with embedded schemas for Tarantool
  3.3.0 – 3.7.0, plus opt-in HTTP fetching via `WithSchemaURLDefault`,
  `WithSchemaURL`, `WithHTTPClient`, and `DefaultSchemaURL` (#27).
* `collectors.Storage.WithSkipInvalid(bool)` to silently skip documents that
  failed to parse, restoring the pre-1.1 lenient behavior (#29).
* `tarantool.Builder.WithEnvIgnore(patterns ...string)` accepts shell-glob
  patterns for env-var names to drop before the env transform runs (#30).

### Changed

* `collectors.NewSource` now takes `ctx context.Context` as its first argument.
  Previously the function created `context.Background()` internally; callers
  must now supply a context, which is forwarded to `DataSource.FetchStream`.
  Migrate `NewSource(src, fmt)` to `NewSource(ctx, src, fmt)`. This is a
  breaking change (#27).
* `tarantool.New()` now uses the newest embedded JSON Schema by default
  instead of fetching
  `https://download.tarantool.org/tarantool/schema/config.schema.json` at
  `Build()` time, and schema selectors on `tarantool.Builder` are now mutually
  exclusive. This is a breaking change in default behavior (#27).
* `collectors.Storage` is now strict by default: a document whose value fails
  to parse causes `Collectors` to return an error wrapping `ErrFormatParse`
  that identifies the offending storage key, instead of being silently
  dropped. Use `WithSkipInvalid(true)` to restore the previous lenient
  behavior (#26).
* Remove redundant `roles` merge strategy from `tarantool.Builder` defaults
  since `MergeReplace` is already the default inheritance behavior (#34).
* Remove hardcoded `leader` exclusion from `tarantool.Builder` default
  inheritance options so `leader` is now inherited down the hierarchy like
  other keys. Users who need the old behavior can opt out via
  `WithInheritanceOption(config.WithNoInherit("leader"))` (#36).

### Fixed

* Fix empty YAML mappings (`{}`) being silently dropped during parsing,
  which caused `EffectiveAll()` to miss leaf entities with empty configs (#32).
* Preserve `isArray` flag when merging numeric children into the config tree,
  so YAML sequences are correctly represented as arrays after inheritance
  resolution (#34).
* Env vars with compound schema keys (e.g. `TT_AUDIT_LOG_NONBLOCK`,
  `TT_WAL_QUEUE_MAX_SIZE`, `TT_REPLICATION_FAILOVER`) now resolve to the
  correct config path when a JSON schema is supplied (#31).
* `Builder.AddCollector`, `Builder.Build`, `Builder.WithValidator`,
  `Builder.WithJSONSchema`, `Builder.MustWithJSONSchema`, `Builder.WithMerger`,
  and `Builder.WithInheritance` no longer panic on nil inputs (#39).

## [v1.0.0] - 2026-03-10

Initial release of go-config library.

### Added

* Configuration tree API with hierarchical data storage and safe value
  retrieval.
* Builder pattern for assembling configuration from multiple sources with
  priority-based merging.
* Collectors for reading configuration from: in-memory maps, YAML/JSON files,
  directories, environment variables, and key-value storage (etcd via go-storage).
* Hierarchical inheritance system: define multi-level hierarchies (e.g.,
  global → group → replicaset → instance) with configurable merge strategies
  (replace, append, deep).
* Flexible merging with customizable Merger interface and default
  last-write-wins semantics.
* JSON Schema validation support for configuration integrity checking.
* Order preservation for deterministic key ordering when serializing
  configuration.
* Tarantool-specific builder with predefined defaults for Tarantool cluster
  configuration (env prefix, inheritance rules, schema validation).
* Storage integration with integrity verification via hash validation and
  signature checking.
* Watcher interface for reactive change notifications from storage backends.
* KeyPath type for configuration key manipulation with wildcard pattern support.
* MutableConfig for runtime configuration modifications with validation.
