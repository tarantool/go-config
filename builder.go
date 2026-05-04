package config

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/tarantool/go-config/tree"
	"github.com/tarantool/go-config/validator"
	"github.com/tarantool/go-config/validators/jsonschema"
)

// DefaultsType is a wrapper for default values in inheritance zones.
type DefaultsType map[string]any

// Builder is a builder for stepwise creation of a Config object.
type Builder struct {
	// Ordered list of collectors from which the configuration will be assembled.
	collectors []Collector
	// Validator for validation of the final configuration.
	validator validator.Validator
	// skipValidation, when true, bypasses the Build-time Validate pass.
	// The validator is still carried into MutableConfig.
	skipValidation bool
	// Inheritance hierarchies for configuration inheritance.
	inheritances []inheritanceConfig
	// Merger defines how values are merged into the configuration tree.
	merger Merger
}

// NewBuilder creates a new instance of Builder.
func NewBuilder() Builder {
	return Builder{
		collectors:     nil,
		validator:      nil,
		skipValidation: false,
		inheritances:   nil,
		merger:         nil,
	}
}

// AddCollector adds a new data source to the build pipeline.
// The order of adding collectors is critical: each subsequent
// collector has higher priority than the previous one. Its values
// will override values from earlier collectors when keys match.
//
// A nil collector is accepted here but causes Build to return an
// ErrNilCollector error (annotated with the collector's index)
// instead of panicking. Other collectors are still processed.
func (b *Builder) AddCollector(collector Collector) Builder {
	b.collectors = append(b.collectors, collector)
	return *b
}

// WithValidator sets a custom validator for configuration validation.
// Passing nil clears any previously configured validator; Build
// then skips the validation step entirely.
func (b *Builder) WithValidator(validator validator.Validator) Builder {
	b.validator = validator
	return *b
}

// WithJSONSchema creates a JSON Schema validator and sets it.
// Returns ErrNilSchemaReader if schema is nil, or a wrapped error
// if schema parsing fails. The Builder is returned unchanged on error.
func (b *Builder) WithJSONSchema(schema io.Reader) (Builder, error) {
	if schema == nil {
		return *b, fmt.Errorf("failed to create JSON schema validator: %w", ErrNilSchemaReader)
	}

	validator, err := jsonschema.NewFromReader(schema)
	if err != nil {
		return *b, fmt.Errorf("failed to create JSON schema validator: %w", err)
	}

	b.validator = validator

	return *b, nil
}

// MustWithJSONSchema is like WithJSONSchema but panics on error.
// Useful for static schema definitions. Panics with ErrNilSchemaReader
// if schema is nil.
func (b *Builder) MustWithJSONSchema(schema io.Reader) Builder {
	result, err := b.WithJSONSchema(schema)
	if err != nil {
		panic(err)
	}

	return result
}

// WithoutValidation skips the Build-time validation pass. Any validator
// configured via [Builder.WithValidator] or [Builder.WithJSONSchema] is
// retained: [Builder.BuildMutable] still attaches it to the resulting
// [MutableConfig], so runtime mutations (Set/Merge/Update) remain validated.
//
// Useful when initial sources are intentionally partial (e.g. completed
// later by env vars or a remote storage layer) but mutation-time
// validation is still desired.
func (b *Builder) WithoutValidation() Builder {
	b.skipValidation = true
	return *b
}

// WithMerger sets a custom merger for the configuration assembly.
// If not set, or if nil is passed, the default merging logic
// (Default) is used at Build time.
func (b *Builder) WithMerger(merger Merger) Builder {
	b.merger = merger
	return *b
}

// WithInheritance registers a hierarchy for inheritance resolution.
// Multiple hierarchies can be registered (e.g., groups and buckets).
//
// The levels parameter defines the structural keys (use Levels() to create).
// Options configure exclusions, defaults, and merge strategies.
//
// Inheritance is resolved during Build(), after collector merging
// but before validation. This ensures the validator sees the effective
// (fully resolved) config for each leaf entity.
//
// A nil levels slice is accepted and registers an empty hierarchy.
// Any nil entries in opts are skipped silently.
func (b *Builder) WithInheritance(levels []string, opts ...InheritanceOption) Builder {
	inheritanceCfg := inheritanceConfig{
		levels:          levels,
		defaults:        nil,
		noInherit:       nil,
		noInheritFrom:   nil,
		mergeStrategies: nil,
	}

	for _, opt := range opts {
		if opt == nil {
			continue
		}

		opt(&inheritanceCfg)
	}

	b.inheritances = append(b.inheritances, inheritanceCfg)

	return *b
}

// Build starts the configuration assembly process.
// It performs reading data from all collectors, merging them,
// validation against the schema, and returns a ready Config object or an error.
//
// Nil collectors (top-level or returned from a MultiCollector) are not
// dereferenced: each contributes an ErrNilCollector entry to the returned
// error slice and is skipped. The remaining collectors are still processed.
func (b *Builder) Build(ctx context.Context) (Config, []error) {
	root := tree.New()

	var errs []error

	merger := b.merger
	if merger == nil {
		merger = Default
	}

	for i, col := range b.collectors {
		if col == nil {
			errs = append(errs, fmt.Errorf("%w at index %d", ErrNilCollector, i))

			continue
		}

		multiCol, isMulti := col.(MultiCollector)
		if !isMulti {
			err := MergeCollectorWithMerger(ctx, root, col, merger)
			if err != nil {
				errs = append(errs, err)
			}

			continue
		}

		subs, err := multiCol.Collectors(ctx)
		if err != nil {
			errs = append(errs, NewCollectorError(col.Name(), err))

			continue
		}

		for j, sub := range subs {
			if sub == nil {
				errs = append(errs, NewCollectorError(col.Name(),
					fmt.Errorf("%w at sub-index %d", ErrNilCollector, j)))

				continue
			}

			err := MergeCollectorWithMerger(ctx, root, sub, merger)
			if err != nil {
				errs = append(errs, err)
			}
		}
	}

	if len(errs) > 0 {
		return Config{root: nil, inheritances: nil, validator: nil}, errs
	}

	if b.validator != nil && !b.skipValidation {
		validationErrs := b.validator.Validate(root)
		for i := range validationErrs {
			errs = append(errs, &validationErrs[i])
		}
	}

	if len(errs) > 0 {
		return Config{root: nil, inheritances: nil, validator: nil}, errs
	}

	return newConfig(root, b.inheritances, b.validator), nil
}

// BuildMutable starts the configuration assembly process but returns
// a mutable MutableConfig object that allows changes after creation.
func (b *Builder) BuildMutable(ctx context.Context) (MutableConfig, []error) {
	cfg, errs := b.Build(ctx)
	if len(errs) > 0 {
		return MutableConfig{Config: Config{root: nil, inheritances: nil, validator: nil}, mu: sync.RWMutex{}}, errs
	}

	return MutableConfig{Config: cfg, mu: sync.RWMutex{}}, nil
}
