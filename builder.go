package config

import (
	"io"
	"sync"

	"github.com/tarantool/go-config/tree"
)

// DefaultsType is a wrapper for default values in inheritance zones.
type DefaultsType map[string]any

// Builder is a builder for stepwise creation of a Config object.
type Builder struct {
	// Ordered list of collectors from which the configuration will be assembled.
	collectors []Collector
	// Schema source for validation of the final configuration.
	schema io.Reader
	// Inheritance zones and their default values.
	scopes map[string]DefaultsType
	// Merger defines how values are merged into the configuration tree.
	merger Merger
}

// NewBuilder creates a new instance of Builder.
func NewBuilder() Builder {
	return Builder{collectors: nil, schema: nil, scopes: nil, merger: nil}
}

// AddCollector adds a new data source to the build pipeline.
// The order of adding collectors is critical: each subsequent
// collector has higher priority than the previous one. Its values
// will override values from earlier collectors when keys match.
func (b *Builder) AddCollector(collector Collector) Builder {
	b.collectors = append(b.collectors, collector)
	return *b
}

// WithJSONSchema sets a schema for validation of the final configuration.
// If no schema is set, validation is not performed.
func (b *Builder) WithJSONSchema(schema io.Reader) Builder {
	b.schema = schema
	return *b
}

// AddScope adds an inheritance zone.
//
// For example, the path "/groups/*/replicasets/*/instances" indicates that for
// elements located in instance, inheritance is allowed, along the chain, of
// their properties from higher "parent" branches of the configuration, up to
// the top (global) level.
//
// Multiple different inheritance branches can be added to the config.
//
// Additionally, for each inheritance zone, default values can be specified,
// which have the lowest priority and are used only if values are not explicitly
// set in the configuration.
//
// Passing `nil` means that default values are not required.
func (b *Builder) AddScope(scopes KeyPath, defaults DefaultsType) Builder {
	if b.scopes == nil {
		b.scopes = make(map[string]DefaultsType)
	}

	b.scopes[scopes.String()] = defaults

	return *b
}

// WithMerger sets a custom merger for the configuration assembly.
// If not set, the default merging logic is used.
func (b *Builder) WithMerger(merger Merger) Builder {
	b.merger = merger
	return *b
}

// Build starts the configuration assembly process.
// It performs reading data from all collectors, merging them,
// validation against the schema, and returns a ready Config object or an error.
func (b *Builder) Build() (Config, []error) {
	root := tree.New()

	var errs []error
	// Determine which merger to use.
	merger := b.merger
	if merger == nil {
		merger = Default
	}

	// Process collectors in order (later collectors have higher priority).
	for _, col := range b.collectors {
		err := MergeCollectorWithMerger(root, col, merger)
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return Config{root: nil}, errs
	}

	return newConfig(root), nil
}

// BuildMutable starts the configuration assembly process but returns
// a mutable MutableConfig object that allows changes after creation.
func (b *Builder) BuildMutable() (MutableConfig, []error) {
	cfg, errs := b.Build()
	if len(errs) > 0 {
		return MutableConfig{Config: Config{root: nil}, mu: sync.RWMutex{}}, errs
	}

	return MutableConfig{Config: cfg, mu: sync.RWMutex{}}, nil
}
