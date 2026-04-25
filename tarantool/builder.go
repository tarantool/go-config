package tarantool

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	config "github.com/tarantool/go-config"
	"github.com/tarantool/go-config/collectors"
	"github.com/tarantool/go-config/tarantool/internal/envpath"
	"github.com/tarantool/go-storage/integrity"
)

const defaultEnvPrefix = "TT_"
const defaultEnvSuffix = "_DEFAULT"

// DefaultStorageKey is the middle path segment that Tarantool uses between the
// storage base prefix and the configuration keys. The full storage path is
// "<prefix>/<storageKey>/<key>", matching the layout used by the tt CLI tool.
// See https://github.com/tarantool/tt/blob/master/lib/cluster/storages.go.
const DefaultStorageKey = "config"

// Builder assembles a Tarantool-compatible [config.Config] from standard
// sources. Use [New] to create a Builder with Tarantool defaults, chain
// With* methods, and call [Builder.Build].
type Builder struct {
	configFile string
	configDir  string
	envPrefix  string

	storage    *integrity.Typed[[]byte]
	storageKey string

	schema        []byte
	schemaFile    string
	schemaVersion string
	schemaURL     string
	schemaURLSet  bool
	schemaHTTP    bool
	skipSchema    bool
	httpClient    *http.Client

	inheritanceOpts []config.InheritanceOption

	merger config.Merger
}

// New creates a Builder with Tarantool defaults:
//   - env prefix: "TT_"
//   - inheritance: Global → groups → replicasets → instances
//   - default inheritance options: credentials(MergeDeep), roles(MergeAppend), leader(NoInherit)
//   - schema: the newest embedded Tarantool version is used offline by default
func New() *Builder {
	return &Builder{ //nolint:exhaustruct
		envPrefix:  defaultEnvPrefix,
		storageKey: DefaultStorageKey,
	}
}

// WithConfigFile sets the path to a single YAML config file.
// Mutually exclusive with [Builder.WithConfigDir].
func (b *Builder) WithConfigFile(path string) *Builder {
	b.configFile = path
	return b
}

// WithConfigDir sets the path to a directory of *.yaml config files.
// Mutually exclusive with [Builder.WithConfigFile].
func (b *Builder) WithConfigDir(path string) *Builder {
	b.configDir = path
	return b
}

// WithStorage sets the centralized storage backend.
// The [integrity.Typed] instance must already be configured with the
// correct prefix (use [ConfigPrefix] to build it). The caller is
// responsible for building the instance with appropriate
// hashers/verifiers via [integrity.NewTypedBuilder].
func (b *Builder) WithStorage(typed *integrity.Typed[[]byte]) *Builder {
	b.storage = typed
	return b
}

// WithStorageKey overrides the middle path segment used for the storage
// collector's source name (default [DefaultStorageKey] = "config").
func (b *Builder) WithStorageKey(key string) *Builder {
	b.storageKey = key
	return b
}

// WithEnvPrefix sets the environment variable prefix (default "TT_").
func (b *Builder) WithEnvPrefix(prefix string) *Builder {
	b.envPrefix = prefix
	return b
}

// WithSchema sets a JSON Schema from raw bytes.
// Mutually exclusive with [Builder.WithSchemaFile], [Builder.WithSchemaVersion],
// [Builder.WithSchemaURLDefault], [Builder.WithSchemaURL], and [Builder.WithoutSchema];
// setting more than one returns
// [ErrConflictingSchemaOptions] at [Builder.Build] time.
func (b *Builder) WithSchema(schema []byte) *Builder {
	b.schema = schema

	return b
}

// WithSchemaFile sets a JSON Schema from a local file path.
// Mutually exclusive with [Builder.WithSchema], [Builder.WithSchemaVersion],
// [Builder.WithSchemaURLDefault], [Builder.WithSchemaURL], and [Builder.WithoutSchema];
// setting more than one returns
// [ErrConflictingSchemaOptions] at [Builder.Build] time.
func (b *Builder) WithSchemaFile(path string) *Builder {
	b.schemaFile = path

	return b
}

// WithSchemaVersion resolves the JSON Schema from the embedded schema registry
// by the given version string (e.g. "3.7.0"). Use [RegisterSchema] to add
// custom versions. Returns [ErrUnknownSchemaVersion] at [Builder.Build] time if
// the version is not found.
// Mutually exclusive with [Builder.WithSchema], [Builder.WithSchemaFile],
// [Builder.WithSchemaURLDefault], [Builder.WithSchemaURL], and [Builder.WithoutSchema];
// setting more than one returns
// [ErrConflictingSchemaOptions] at [Builder.Build] time.
func (b *Builder) WithSchemaVersion(version string) *Builder {
	b.schemaVersion = version

	return b
}

// WithSchemaURLDefault resolves the JSON Schema over HTTP from [DefaultSchemaURL].
// Mutually exclusive with [Builder.WithSchema], [Builder.WithSchemaFile],
// [Builder.WithSchemaVersion], [Builder.WithSchemaURL], and
// [Builder.WithoutSchema]; setting more than one returns
// [ErrConflictingSchemaOptions] at [Builder.Build] time.
func (b *Builder) WithSchemaURLDefault() *Builder {
	b.schemaHTTP = true

	return b
}

// WithSchemaURL resolves the JSON Schema over HTTP from the provided URL.
// Mutually exclusive with [Builder.WithSchema], [Builder.WithSchemaFile],
// [Builder.WithSchemaVersion], [Builder.WithSchemaURLDefault], and
// [Builder.WithoutSchema]; setting more than one returns
// [ErrConflictingSchemaOptions] at [Builder.Build] time.
func (b *Builder) WithSchemaURL(url string) *Builder {
	b.schemaURL = url
	b.schemaURLSet = true

	return b
}

// WithHTTPClient injects the HTTP client used by [Builder.WithSchemaURLDefault] and
// [Builder.WithSchemaURL]. If unset, a package-private default client with a
// 30-second timeout is used.
func (b *Builder) WithHTTPClient(client *http.Client) *Builder {
	b.httpClient = client

	return b
}

// WithoutSchema disables JSON Schema validation entirely.
// Mutually exclusive with [Builder.WithSchema], [Builder.WithSchemaFile],
// [Builder.WithSchemaVersion], [Builder.WithSchemaURLDefault], [Builder.WithSchemaURL],
// and [Builder.WithoutSchema]; setting more than one returns
// [ErrConflictingSchemaOptions] at [Builder.Build] time.
func (b *Builder) WithoutSchema() *Builder {
	b.skipSchema = true

	return b
}

// WithInheritanceOption adds extra inheritance options on top of the
// Tarantool defaults (credentials=MergeDeep, roles=MergeAppend,
// leader=NoInherit).
func (b *Builder) WithInheritanceOption(opts ...config.InheritanceOption) *Builder {
	b.inheritanceOpts = append(b.inheritanceOpts, opts...)
	return b
}

// WithMerger sets a custom merger for the configuration assembly.
func (b *Builder) WithMerger(m config.Merger) *Builder {
	b.merger = m
	return b
}

// Build assembles all configured collectors in priority order, applies
// inheritance and validation, and returns an immutable [config.Config].
// The context is forwarded to collector reads.
func (b *Builder) Build(ctx context.Context) (config.Config, error) {
	inner, err := b.buildInner(ctx)
	if err != nil {
		return config.Config{}, err
	}

	cfg, errs := inner.Build(ctx)
	if len(errs) > 0 {
		return cfg, errors.Join(errs...)
	}

	return cfg, nil
}

// BuildMutable is like [Builder.Build] but returns a [config.MutableConfig]
// that allows runtime modifications.
// Note: this method is not implemented yet and is under active development.
func (b *Builder) BuildMutable(ctx context.Context) (*config.MutableConfig, error) {
	inner, err := b.buildInner(ctx)
	if err != nil {
		return nil, err
	}

	cfg, errs := inner.BuildMutable(ctx)
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	return &cfg, nil
}

// validate checks that the builder configuration is consistent.
func (b *Builder) validate() error {
	if b.configFile != "" && b.configDir != "" {
		return ErrMutuallyExclusive
	}

	schemaCount := 0
	if b.schema != nil {
		schemaCount++
	}

	if b.schemaFile != "" {
		schemaCount++
	}

	if b.schemaVersion != "" {
		schemaCount++
	}

	if b.skipSchema {
		schemaCount++
	}

	if b.schemaURLSet {
		schemaCount++
	}

	if b.schemaHTTP {
		schemaCount++
	}

	if schemaCount > 1 {
		return ErrConflictingSchemaOptions
	}

	return nil
}

// ConfigPrefix builds the full storage config prefix by combining the base
// prefix with [DefaultStorageKey]: "<base>/config/".
// This matches the layout used by the tt CLI tool's getConfigPrefix.
// Use this when constructing the prefix for [integrity.NewTypedBuilder].
func ConfigPrefix(base string) string {
	return strings.TrimRight(base, "/") + "/" + DefaultStorageKey + "/"
}

// tarantoolInheritanceOpts returns the default Tarantool inheritance options.
func tarantoolInheritanceOpts() []config.InheritanceOption {
	return []config.InheritanceOption{
		config.WithInheritMerge("credentials", config.MergeDeep),
		config.WithInheritMerge("roles", config.MergeAppend),
		config.WithNoInherit("leader"),
	}
}

// keyPathFromLoweredKey splits a lowercased key by "_" and returns a KeyPath
// with empty segments filtered out.
func keyPathFromdKey(key string) config.KeyPath {
	parts := strings.Split(strings.ToLower(key), "_")

	filtered := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			filtered = append(filtered, p)
		}
	}

	if len(filtered) == 0 {
		return nil
	}

	return config.NewKeyPathFromSegments(filtered)
}

func resolvePath(resolver *envpath.Node, key string) config.KeyPath {
	if resolver != nil {
		return resolver.Resolve(key)
	}

	return keyPathFromdKey(key)
}

// regularEnvTransform returns a transform function for the regular env
// collector. It skips variables ending with "_DEFAULT" (those belong to the
// default-env collector) and resolves the remainder via the schema trie
// when available, otherwise via naive underscore split.
func regularEnvTransform(resolver *envpath.Node) func(string) config.KeyPath {
	suffix := defaultEnvSuffix

	return func(key string) config.KeyPath {
		if strings.HasSuffix(key, suffix) {
			return nil
		}

		return resolvePath(resolver, key)
	}
}

// defaultEnvTransform returns a transform function for the default-env
// collector. It filters for variables ending with "_DEFAULT", strips that
// suffix, and resolves the remainder via the schema trie when available,
// otherwise via naive underscore split.
func defaultEnvTransform(resolver *envpath.Node) func(string) config.KeyPath {
	suffix := defaultEnvSuffix

	return func(key string) config.KeyPath {
		if !strings.HasSuffix(key, suffix) {
			return nil
		}

		key = strings.TrimSuffix(key, suffix)
		if key == "" {
			return nil
		}

		return resolvePath(resolver, key)
	}
}

// buildInner assembles the inner [config.Builder] with all collectors,
// schema validation, inheritance, and merger configured.
func (b *Builder) buildInner(ctx context.Context) (config.Builder, error) {
	err := b.validate()
	if err != nil {
		return config.Builder{}, err
	}

	// Resolve the schema once: env transforms need the resolver, schema
	// validation needs the raw bytes.
	var (
		schemaBytes []byte
		resolver    *envpath.Node
	)

	if !b.skipSchema {
		schemaBytes, err = b.resolveSchema(ctx)
		if err != nil {
			return config.Builder{}, err
		}

		// Malformed schemas surface via WithJSONSchema below; a nil resolver
		// falls back to the naive underscore split.
		resolver, _ = envpath.Build(schemaBytes)
	}

	inner := config.NewBuilder()

	// 1. Default env vars (lowest priority).
	inner = inner.AddCollector(
		collectors.NewEnv().
			WithPrefix(b.envPrefix).
			WithTransform(defaultEnvTransform(resolver)).
			WithName("env-default").
			WithSourceType(config.EnvDefaultSource),
	)

	// 2. Config file or directory.
	if b.configFile != "" {
		source, sourceErr := collectors.NewSource(
			ctx,
			collectors.NewFile(b.configFile),
			collectors.NewYamlFormat(),
		)
		if sourceErr != nil {
			return config.Builder{}, fmt.Errorf("config file: %w", sourceErr)
		}

		inner = inner.AddCollector(source)
	} else if b.configDir != "" {
		inner = inner.AddCollector(
			collectors.NewDirectory(b.configDir, ".yaml", collectors.NewYamlFormat()),
		)
	}

	// 3. Centralized storage.
	if b.storage != nil {
		inner = inner.AddCollector(
			collectors.NewStorage(b.storage, b.storageKey, collectors.NewYamlFormat()),
		)
	}

	// 4. Env vars (highest priority).
	inner = inner.AddCollector(
		collectors.NewEnv().
			WithPrefix(b.envPrefix).
			WithTransform(regularEnvTransform(resolver)).
			WithName("env").
			WithSourceType(config.EnvSource),
	)

	// 5. Schema validation.
	if !b.skipSchema {
		inner, err = inner.WithJSONSchema(bytes.NewReader(schemaBytes))
		if err != nil {
			return config.Builder{}, fmt.Errorf("json schema: %w", err)
		}
	}

	// 6. Inheritance.
	opts := tarantoolInheritanceOpts()

	opts = append(opts, b.inheritanceOpts...)

	inner = inner.WithInheritance(
		config.Levels(config.Global, "groups", "replicasets", "instances"),
		opts...,
	)

	// 7. Custom merger.
	if b.merger != nil {
		inner = inner.WithMerger(b.merger)
	}

	return inner, nil
}
