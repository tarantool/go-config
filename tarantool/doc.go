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
// # Environment variable values
//
// With a schema, an env-var value is read as the type the schema gives
// its path, following the rules of Tarantool's own env source, and
// lands in the config typed, as if a file had set it:
//
//   - An empty value is unset, as in Tarantool: it overrides nothing.
//   - A string field takes the value as is, and the value stays a
//     string: TT_CONSOLE_SOCKET=123 is "123" through validation.
//   - A number or an integer is parsed (decimal, or hex with "0x"); a
//     duration or byte size with a unit ("30s", "1GiB") stays a string,
//     as it does in a file.
//   - A boolean is "true" or "false" in any case, or "1" or "0".
//   - An array is JSON when the value starts with "[", or a comma-separated
//     list when its items are scalars: TT_CONFIG_ETCD_ENDPOINTS=a,b.
//   - A map (an object with free keys, like labels) is JSON when the value
//     starts with "{", or "k=v,k2=v2" when its values are scalars.
//   - A value of no type (app.cfg, roles_cfg) is JSON only.
//   - A union, such as a number-or-string, prefers the variant that turns
//     the value into something other than the plain string.
//
// An array or a map becomes nodes of the tree, as from a file, so an
// env array replaces a file array whole and an env map merges into a
// file map key by key. A value that cannot be read as its type (a record
// such as TT_CONSOLE, JSON that does not parse, "yes" for a boolean) is
// kept as the raw string, and validation reports the type error where
// Tarantool would refuse to start. Without a schema every value stays a
// string.
//
// # Inheritance
//
// The builder registers the Tarantool hierarchy
// (Global → groups → replicasets → instances) with default merge strategies:
//   - credentials — MergeDeep
//
// # Templates
//
// Effective and EffectiveAll substitute {{ instance_name }},
// {{ replicaset_name }} and {{ group_name }} after source priority and
// inheritance. Substitution applies to string leaves and mapping keys, including
// nested YAML mappings and sequences. Go containers stored as whole leaf values
// stay unchanged. ASCII spaces around names are optional; replacement values are
// not expanded again. Unknown placeholders retain their exact spelling.
//
// TemplateVariables supplies the three built-in names from a complete Tarantool
// instance path and returns nil for other path shapes. For a generic builder,
// register it through config.WithTemplateVariables under the Tarantool hierarchy.
// Additional variables can be supplied through WithInheritanceOption and
// config.WithTemplateVariables. Later providers override earlier values; known
// empty values replace their placeholders with an empty string.
//
// config.context declarations remain ordinary configuration data. References
// such as {{ context.secret }} stay unchanged unless a variable provider supplies
// that name. The template engine performs no environment or file reads.
//
// Get, Slice and MarshalYAML on the original config retain source templates.
// MarshalYAML on an effective config writes substituted values and preserves
// unknown placeholders, comments and scalar styles. WithEnvPrefix controls
// TT_* config overrides independently of template substitution.
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
