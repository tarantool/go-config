# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/)
and this project adheres to [Semantic
Versioning](http://semver.org/spec/v2.0.0.html) except to the first release.

## [Unreleased]

### Added

* Offline JSON Schema by default, plus opt-in HTTP fetching via
  `WithSchemaURLDefault`, `WithSchemaURL`, `WithHTTPClient`, and
  `DefaultSchemaURL`.
* Embed gzipped minified schemas for Tarantool 3.3.0 – 3.7.0, decompressed at
  package init.
* `collectors.Storage.WithSkipInvalid(bool)` to silently skip documents that
  failed to parse.

### Changed

* `collectors.NewSource` now takes `ctx context.Context` as its first argument.
  Previously the function created `context.Background()` internally; callers
  must now supply a context, which is forwarded to `DataSource.FetchStream`.
  Migrate `NewSource(src, fmt)` to `NewSource(ctx, src, fmt)`. This is a
  breaking change.
* `tarantool.New()` now uses the newest embedded JSON Schema by default instead
  of fetching `https://download.tarantool.org/tarantool/schema/config.schema.json`
  at `Build()` time. This is a breaking change in default behavior.
* Schema selectors on `tarantool.Builder` are now mutually exclusive.
* `collectors.Storage.Collectors` is now strict by default: a document whose
  value fails to parse causes `Collectors` to return an error wrapping
  `ErrFormatParse` that identifies the offending storage key, instead of
  being silently dropped.
* Remove redundant `roles` merge strategy from `tarantool.Builder` defaults
  since `MergeReplace` is already the default inheritance behavior
  ([#34](https://github.com/tarantool/go-config/issues/34)).
* Remove hardcoded `leader` exclusion from `tarantool.Builder` default
  inheritance options so `leader` is now inherited down the hierarchy
  like other keys. Users who need the old behavior can opt out via
  `WithInheritanceOption(config.WithNoInherit("leader"))`
  ([#36](https://github.com/tarantool/go-config/issues/36)).

### Fixed

* `collectors.Storage` no longer silently skips documents with invalid YAML,
  which could mask partially-loaded configurations
  ([#26](https://github.com/tarantool/go-config/issues/26)).
* Fix empty YAML mappings (`{}`) being silently dropped during parsing,
  which caused `EffectiveAll()` to miss leaf entities with empty configs
  ([#32](https://github.com/tarantool/go-config/issues/32)).
* Preserve `isArray` flag when merging numeric children into the config tree,
  so YAML sequences are correctly represented as arrays after inheritance
  resolution
  ([#34](https://github.com/tarantool/go-config/issues/34)).

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
