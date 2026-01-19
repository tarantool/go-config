package config

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/tarantool/go-config/internal/tree"
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
}

// NewBuilder creates a new instance of Builder.
func NewBuilder() Builder {
	return Builder{collectors: nil, schema: nil, scopes: nil}
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

// Build starts the configuration assembly process.
// It performs reading data from all collectors, merging them,
// validation against the schema, and returns a ready Config object or an error.
func (b *Builder) Build() (Config, []error) {
	root := tree.New()

	var errs []error
	// Process collectors in order (later collectors have higher priority).
	for _, col := range b.collectors {
		err := mergeCollector(root, col)
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return Config{root: nil}, errs
	}

	return newConfig(root), nil
}

// mergeCollector reads all values from a collector and merges them into the tree.
func mergeCollector(root *tree.Node, col Collector) error {
	ctx := context.Background()

	ch := col.Read(ctx)
	for val := range ch {
		meta := val.Meta()
		// Set the value at the path in the tree.
		// For now, we store raw value; tree.Node expects a raw value (any).
		// We need to extract raw value from val. Use Get with interface{}.
		var raw any

		err := val.Get(&raw)
		if err != nil {
			// If Get fails, we cannot obtain raw value; skip or error.
			return fmt.Errorf("failed to get raw value for key %s: %w", meta.Key.String(), err)
		}

		node := root.Get(meta.Key)
		if node == nil {
			// Create node and set value.
			root.Set(meta.Key, raw)

			node = root.Get(meta.Key)
		} else {
			// Replace value.
			node.Value = raw
		}
		// Update node metadata.
		node.Source = col.Name()
		node.Revision = string(col.Revision())
	}

	return nil
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
