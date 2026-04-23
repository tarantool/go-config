// Package tarantool provides a high-level Builder that assembles a
// Tarantool-compatible configuration from standard sources.
//
// It wires together the go-config collectors (Env, File, Directory, Storage),
// JSON Schema validation, and hierarchical inheritance with Tarantool-specific
// defaults so that callers can build a ready-to-use [config.Config] with
// minimal boilerplate.
//
// # Collector Priority (lowest to highest)
//
//  1. Default environment variables — TT_*_DEFAULT prefix.
//  2. Configuration file or directory (mutually exclusive).
//  3. Centralized storage (etcd / tarantool-storage) under <prefix>/config/*.
//  4. Environment variables — TT_* prefix.
//
// # Ignoring environment variables
//
// [Builder.WithEnvIgnore] takes shell-glob patterns ([path.Match]
// syntax) that drop matching env vars before the env transform runs.
// Patterns are matched against the full env-var name, so use the same
// string you'd see in `env | grep TT_` — for example,
// WithEnvIgnore("TT_CLI_*") to skip the variables the tt CLI exports
// into developer shells. Invalid patterns surface as
// [ErrBadEnvIgnorePattern] from [Builder.Build].
//
// # Environment variable path resolution
//
// When a JSON schema is supplied (via [Builder.WithSchema] or
// [Builder.WithSchemaFile], or fetched at build time), env-var names
// are resolved to config key paths by walking the schema with greedy
// longest-prefix matching. This lets compound keys like
// TT_AUDIT_LOG_NONBLOCK reach audit_log.nonblock and
// TT_WAL_QUEUE_MAX_SIZE reach wal_queue_max_size, instead of being
// naively split into "audit/log/nonblock" or "wal/queue/max/size".
//
// Vars that don't match any schema path are silently dropped. With
// [Builder.WithoutSchema] the legacy underscore-split is used.
//
// Wildcard segments (e.g. group / replicaset / instance names under
// additionalProperties) consume exactly one underscore-separated token
// each; user-defined names containing '_' (e.g. "my_group") are not
// supported and will fail to resolve.
//
// Schema features the resolver does not understand — allOf / oneOf /
// anyOf composition, $ref pointing anywhere other than #/$defs/<name>
// or #/definitions/<name> — are treated as opaque and any paths
// buried beneath them won't be reachable via env vars.
//
// # Inheritance
//
// The builder registers the Tarantool hierarchy
// (Global → groups → replicasets → instances) with default merge strategies:
//   - credentials — MergeDeep
//
// # Schema Validation
//
// By default the builder validates the assembled configuration against the
// newest Tarantool JSON Schema available in the embedded offline registry —
// no network access is required. The active schema can be overridden with one
// (and only one) of six mutually exclusive setters:
//   - [Builder.WithSchema] — raw schema bytes supplied by the caller.
//   - [Builder.WithSchemaFile] — path to a local JSON Schema file.
//   - [Builder.WithSchemaVersion] — a specific version from the registry
//     (see [SchemaVersions], [Schema], [RegisterSchema]).
//   - [Builder.WithSchemaURLDefault] — fetch from [DefaultSchemaURL].
//   - [Builder.WithSchemaURL] — fetch from a caller-supplied URL.
//   - [Builder.WithoutSchema] — disables validation entirely.
//
// Setting more than one of these on the same builder returns
// [ErrConflictingSchemaOptions] at [Builder.Build] time.
//
// # Example
//
//	cfg, err := tarantool.New().
//	    WithConfigFile("/etc/tarantool/config.yaml").
//	    Build(context.Background())
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	inst, _ := cfg.Effective(config.NewKeyPath(
//	    "groups/storages/replicasets/s-001/instances/s-001-a"))
package tarantool
