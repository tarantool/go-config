package tarantool

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/tarantool/go-config"
	"github.com/tarantool/go-config/collectors"
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

	schema     []byte
	schemaFile string
	skipSchema bool

	inheritanceOpts []config.InheritanceOption

	merger config.Merger
}

// New creates a Builder with Tarantool defaults:
//   - env prefix: "TT_"
//   - inheritance: Global → groups → replicasets → instances
//   - default inheritance options: credentials(MergeDeep), roles(MergeAppend), leader(NoInherit)
//   - schema: fetched from download.tarantool.org at Build time
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

// WithSchema sets a JSON Schema from raw bytes, disabling the default
// HTTP fetch from download.tarantool.org.
func (b *Builder) WithSchema(schema []byte) *Builder {
	b.schema = schema
	b.schemaFile = ""
	b.skipSchema = false

	return b
}

// WithSchemaFile sets a JSON Schema from a local file path, disabling
// the default HTTP fetch.
func (b *Builder) WithSchemaFile(path string) *Builder {
	b.schemaFile = path
	b.schema = nil
	b.skipSchema = false

	return b
}

// WithoutSchema disables JSON Schema validation entirely.
func (b *Builder) WithoutSchema() *Builder {
	b.skipSchema = true
	b.schema = nil
	b.schemaFile = ""

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
// The context is used for schema fetching (HTTP) and collector reads.
func (b *Builder) Build(ctx context.Context) (config.Config, error) {
	inner, err := b.buildInner(ctx)
	if err != nil {
		return config.Config{}, err
	}

	cfg, errs := inner.Build() //nolint:contextcheck // config.Builder.Build does not accept context.
	if len(errs) > 0 {
		return cfg, errors.Join(errs...)
	}

	return cfg, nil
}

// BuildMutable is like [Builder.Build] but returns a [config.MutableConfig]
// that allows runtime modifications.
func (b *Builder) BuildMutable(ctx context.Context) (*config.MutableConfig, error) {
	inner, err := b.buildInner(ctx)
	if err != nil {
		return nil, err
	}

	cfg, errs := inner.BuildMutable() //nolint:contextcheck // config.Builder.BuildMutable does not accept context.
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

// regularEnvTransform returns a transform function for the regular env
// collector. It skips variables ending with "_DEFAULT" (those belong to the
// default-env collector), lowercases the remainder, and splits by "_".
func regularEnvTransform() func(string) config.KeyPath {
	suffix := defaultEnvSuffix

	return func(key string) config.KeyPath {
		if strings.HasSuffix(key, suffix) {
			return nil
		}

		return keyPathFromdKey(key)
	}
}

// defaultEnvTransform returns a transform function for the default-env
// collector. It filters for variables ending with "_DEFAULT", strips that
// suffix, lowercases the remainder, and splits by "_" into a key path.
func defaultEnvTransform(_ string) func(string) config.KeyPath {
	suffix := defaultEnvSuffix

	return func(key string) config.KeyPath {
		if !strings.HasSuffix(key, suffix) {
			return nil
		}

		key = strings.TrimSuffix(key, suffix)
		if key == "" {
			return nil
		}

		return keyPathFromdKey(key)
	}
}

// buildInner assembles the inner [config.Builder] with all collectors,
// schema validation, inheritance, and merger configured.
func (b *Builder) buildInner(ctx context.Context) (config.Builder, error) {
	err := b.validate()
	if err != nil {
		return config.Builder{}, err
	}

	inner := config.NewBuilder()

	// 1. Default env vars (lowest priority).
	inner = inner.AddCollector(
		collectors.NewEnv().
			WithPrefix(b.envPrefix).
			WithTransform(defaultEnvTransform(b.envPrefix)).
			WithName("env-default").
			WithSourceType(config.EnvDefaultSource),
	)

	// 2. Config file or directory.
	if b.configFile != "" {
		source, sourceErr := collectors.NewSource( //nolint:contextcheck
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
			WithTransform(regularEnvTransform()).
			WithName("env").
			WithSourceType(config.EnvSource),
	)

	// 5. Schema validation.
	if !b.skipSchema {
		schema, schemaErr := b.resolveSchema(ctx)
		if schemaErr != nil {
			return config.Builder{}, schemaErr
		}

		inner, err = inner.WithJSONSchema(bytes.NewReader(schema))
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
