package config

import (
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
	// Inheritance hierarchies for configuration inheritance.
	inheritances []inheritanceConfig
	// Merger defines how values are merged into the configuration tree.
	merger Merger
}

// NewBuilder creates a new instance of Builder.
func NewBuilder() Builder {
	return Builder{collectors: nil, validator: nil, inheritances: nil, merger: nil}
}

// AddCollector adds a new data source to the build pipeline.
// The order of adding collectors is critical: each subsequent
// collector has higher priority than the previous one. Its values
// will override values from earlier collectors when keys match.
func (b *Builder) AddCollector(collector Collector) Builder {
	b.collectors = append(b.collectors, collector)
	return *b
}

// WithValidator sets a custom validator for configuration validation.
func (b *Builder) WithValidator(validator validator.Validator) Builder {
	b.validator = validator
	return *b
}

// WithJSONSchema creates a JSON Schema validator and sets it.
// Returns error if schema parsing fails.
func (b *Builder) WithJSONSchema(schema io.Reader) (Builder, error) {
	validator, err := jsonschema.NewFromReader(schema)
	if err != nil {
		return *b, fmt.Errorf("failed to create JSON schema validator: %w", err)
	}

	b.validator = validator

	return *b, nil
}

// MustWithJSONSchema is like WithJSONSchema but panics on error.
// Useful for static schema definitions.
func (b *Builder) MustWithJSONSchema(schema io.Reader) Builder {
	result, err := b.WithJSONSchema(schema)
	if err != nil {
		panic(err)
	}

	return result
}

// WithMerger sets a custom merger for the configuration assembly.
// If not set, the default merging logic is used.
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
func (b *Builder) WithInheritance(levels []string, opts ...InheritanceOption) Builder {
	inheritanceCfg := inheritanceConfig{
		levels:          levels,
		defaults:        nil,
		noInherit:       nil,
		noInheritFrom:   nil,
		mergeStrategies: nil,
	}
	for _, opt := range opts {
		opt(&inheritanceCfg)
	}

	b.inheritances = append(b.inheritances, inheritanceCfg)

	return *b
}

// Build starts the configuration assembly process.
// It performs reading data from all collectors, merging them,
// validation against the schema, and returns a ready Config object or an error.
func (b *Builder) Build() (Config, []error) {
	root := tree.New()

	var errs []error

	merger := b.merger
	if merger == nil {
		merger = Default
	}

	for _, col := range b.collectors {
		err := MergeCollectorWithMerger(root, col, merger)
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return Config{root: nil, inheritances: nil}, errs
	}

	if b.validator != nil {
		validationErrs := b.validator.Validate(root)
		for i := range validationErrs {
			errs = append(errs, &validationErrs[i])
		}
	}

	if len(errs) > 0 {
		return Config{root: nil, inheritances: nil}, errs
	}

	return newConfig(root, b.inheritances), nil
}

// BuildMutable starts the configuration assembly process but returns
// a mutable MutableConfig object that allows changes after creation.
func (b *Builder) BuildMutable() (MutableConfig, []error) {
	cfg, errs := b.Build()
	if len(errs) > 0 {
		return MutableConfig{Config: Config{root: nil, inheritances: nil}, mu: sync.RWMutex{}, validator: nil}, errs
	}

	return MutableConfig{Config: cfg, mu: sync.RWMutex{}, validator: b.validator}, nil
}
