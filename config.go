package config

import (
	"context"
	"fmt"
	"sync"

	"github.com/tarantool/go-config/keypath"
	"github.com/tarantool/go-config/tree"
	"github.com/tarantool/go-config/validator"
)

// MergerContext holds state for merging a single collector's values.
// Implementations can use this to track ordering or other state across
// multiple MergeValue calls within a single collector.
//
// Custom merger implementations must handle ordering properly when the collector's
// KeepOrder method returns true. This typically involves:
//  1. Allocating a map to track parent-child relationships in CreateContext
//  2. Calling RecordOrdering for each value during MergeValue
//  3. Implementing ApplyOrdering to set the order on tree nodes
//
// For collectors that do not preserve order (KeepOrder returns false),
// the ordering methods can be no-ops.
type MergerContext interface {
	// Collector returns the collector being processed.
	Collector() Collector

	// RecordOrdering tracks a child key under its parent for ordering.
	// This should be called for each value when the collector's KeepOrder
	// returns true and ordering needs to be preserved.
	//
	// The parent parameter is the path to the parent node (may be nil for root).
	// The child parameter is the key of the child node to record.
	//
	// Implementations should store this information and apply it in ApplyOrdering.
	RecordOrdering(parent keypath.KeyPath, child string)

	// ApplyOrdering applies recorded ordering to the tree.
	// Called after all values from the collector have been processed.
	//
	// Implementations should iterate through recorded parent-child relationships
	// and call SetOrder on the corresponding tree nodes to preserve insertion order.
	//
	// Returns an error if ordering cannot be applied.
	ApplyOrdering(root *tree.Node) error
}

// Merger defines how values from collectors are merged into the configuration tree.
// This interface allows customization of the merging process, enabling use cases such as:
//   - Validation: reject invalid values before merging
//   - Transformation: modify values based on their path or source
//   - Selective merging: skip certain paths or sources
//   - Auditing: log or track all merge operations
//   - Custom conflict resolution: define how to handle duplicate keys
//
// The default merging logic is provided by DefaultMerger, which implements
// standard last-write-wins semantics with type-aware merging for maps and arrays.
//
// Custom implementations should:
//  1. Create a context in CreateContext that tracks state for the collector
//  2. Implement MergeValue to handle each value from the collector
//  3. Handle ordering properly if the collector's KeepOrder returns true
//  4. Return meaningful errors when merging fails
//
// Example custom merger that counts merge operations is located in "merger_custom_test.go".
//
// Use Builder.WithMerger to configure a custom merger:
//
//	cfg, errs := config.NewBuilder().
//	    WithMerger(&countingMerger{}).
//	    AddCollector(myCollector).
//	    Build()
type Merger interface {
	// CreateContext creates a new context for processing a collector.
	// Called once per collector before any MergeValue calls.
	//
	// The context should store any state needed for merging values from this collector,
	// such as ordering information, validation state, or statistics.
	//
	// If the collector's KeepOrder returns true, the context should allocate
	// data structures for tracking ordering (typically a map[string][]string).
	CreateContext(collector Collector) MergerContext

	// MergeValue merges a single value into the tree.
	// The method is called for each value produced by the collector.
	//
	// Parameters:
	//   - ctx: the context created by CreateContext for this collector
	//   - root: the root of the configuration tree to merge into
	//   - path: the key path where the value should be merged
	//   - value: the raw value to merge (primitive, slice, or map[string]any)
	//
	// Implementations should:
	//   - Navigate to the appropriate node in the tree using path
	//   - Merge the value according to custom logic or delegate to DefaultMerger
	//   - Call ctx.RecordOrdering if the collector preserves order
	//   - Return an error if merging fails (validation, type mismatch, etc.)
	//
	// The tree is modified in place. Multiple MergeValue calls may update the same
	// nodes if paths overlap (e.g., "a.b" and "a.c" both create children under "a").
	MergeValue(ctx MergerContext, root *tree.Node, path keypath.KeyPath, value any) error
}

// Config provides access to the final configuration data.
type Config struct {
	// root is the internal tree representation of the configuration.
	root *tree.Node
	// inheritances holds inheritance configurations for lazy resolution.
	inheritances []inheritanceConfig
}

// newConfig creates a Config from a tree node.
func newConfig(root *tree.Node, inheritances []inheritanceConfig) Config {
	return Config{root: root, inheritances: inheritances}
}

// Get is the primary, most convenient method for retrieving a value.
// It finds the value at the specified path and extracts it into the variable `dest`.
// Returns metadata and an error if the key is not found or the type cannot be converted.
func (c *Config) Get(path KeyPath, dest any) (MetaInfo, error) {
	val, ok := c.Lookup(path)
	if !ok {
		return MetaInfo{}, fmt.Errorf("%w: %s", ErrKeyNotFound, path)
	}

	err := val.Get(dest)
	if err != nil {
		return val.Meta(), err
	}

	return val.Meta(), nil
}

// Lookup searches for a value by key. Unlike Get, it does not
// return an error if the key is not found, but reports it via a boolean flag.
// Returns a special `Value` object and a flag indicating whether the value was found.
// This is useful when you need to distinguish between a missing value and a nil value.
func (c *Config) Lookup(path KeyPath) (Value, bool) {
	if c.root == nil {
		return nil, false
	}

	node := c.root.Get(path)
	if node == nil {
		return nil, false
	}

	return tree.NewValue(node, path), true
}

// Stat returns metadata for a key (source name, revision)
// without touching the actual value. Useful for debugging and introspection tools.
func (c *Config) Stat(path KeyPath) (MetaInfo, bool) {
	if c.root == nil {
		return MetaInfo{Key: nil, Source: SourceInfo{Name: "", Type: UnknownSource}, Revision: ""}, false
	}

	node := c.root.Get(path)
	if node == nil {
		return MetaInfo{Key: nil, Source: SourceInfo{Name: "", Type: UnknownSource}, Revision: ""}, false
	}

	// Create a temporary value to extract metadata.
	val := tree.NewValue(node, path)

	return val.Meta(), true
}

// Walk returns a channel through which you can iterate over all keys and values in the configuration.
// This is useful for traversing all parameters without needing to know their keys in advance.
// path may be empty (or `nil`) to start from the root of the configuration.
// If depth > 0, only the part of the configuration tree limited by the specified depth is traversed.
// If depth <= 0, the entire object is traversed.
func (c *Config) Walk(ctx context.Context, path KeyPath, depth int) (<-chan Value, error) {
	if c.root == nil {
		return nil, fmt.Errorf("%w: %s", ErrPathNotFound, path)
	}

	start := c.root
	if len(path) > 0 {
		start = c.root.Get(path)
		if start == nil {
			return nil, fmt.Errorf("%w: %s", ErrPathNotFound, path)
		}
	}

	valueCh := make(chan Value)

	go func() {
		defer close(valueCh)

		walkNodes(ctx, start, path, depth, valueCh)
	}()

	return valueCh, nil
}

// walkNodes recursively sends values for leaf nodes.
func walkNodes(ctx context.Context, node *tree.Node, prefix KeyPath, depth int, valueCh chan<- Value) {
	select {
	case <-ctx.Done():
		return
	default:
	}

	switch {
	case depth == 0:
		return
	case node.IsLeaf():
		select {
		case <-ctx.Done():
			return
		case valueCh <- tree.NewValue(node, prefix):
		}

		return
	}

	for _, key := range node.ChildrenKeys() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		child := node.Child(key)
		if child == nil {
			continue
		}

		walkNodes(ctx, child, prefix.Append(key), depth-1, valueCh)
	}
}

// Slice returns a slice of the original config that corresponds to the specified keypath.
// Used to obtain a sub-configuration as a separate Config object.
// If the path does not correspond to an object, returns an error.
// If path is empty (or `nil`), returns a copy of the current Config object.
func (c *Config) Slice(path KeyPath) (Config, error) {
	if c.root == nil {
		if len(path) == 0 {
			return newConfig(nil, c.inheritances), nil
		}

		return Config{}, fmt.Errorf("%w: %s", ErrPathNotFound, path)
	}

	if len(path) == 0 {
		return newConfig(c.root, c.inheritances), nil
	}

	root := c.root.Get(path)
	if root == nil {
		return Config{}, fmt.Errorf("%w: %s", ErrPathNotFound, path)
	}

	return newConfig(root, c.inheritances), nil
}

// Effective returns the resolved (post-inheritance) config for a specific
// leaf entity. The path must point to a concrete leaf entity in the hierarchy
// (e.g., "groups/storages/replicasets/s-001/instances/s-001-a").
//
// If no inheritance was configured in the Builder, returns the raw subtree
// at the given path as a Config.
//
// The returned Config contains only config keys (no structural keys like
// "groups", "replicasets", "instances").
func (c *Config) Effective(path KeyPath) (Config, error) {
	if c.root == nil {
		return Config{}, fmt.Errorf("%w: %s", ErrPathNotFound, path)
	}

	// Try each registered hierarchy.
	for i := range c.inheritances {
		inheritanceCfg := &c.inheritances[i]

		layers, ok := matchHierarchy(c.root, inheritanceCfg, path)
		if !ok {
			continue
		}

		result := resolveEffective(layers, inheritanceCfg)

		return newConfig(result, c.inheritances), nil
	}

	// No hierarchy matched — fall back to raw subtree.
	node := c.root.Get(path)
	if node == nil {
		return Config{}, fmt.Errorf("%w: %s", ErrPathNotFound, path)
	}

	return newConfig(cloneNode(node), c.inheritances), nil
}

// EffectiveAll returns resolved configs for ALL leaf entities found in the
// hierarchy. The returned map keys are full paths to each leaf entity.
//
// If no inheritance was configured, returns an error.
func (c *Config) EffectiveAll() (map[string]Config, error) {
	if len(c.inheritances) == 0 {
		return nil, ErrNoInheritance
	}

	result := make(map[string]Config)

	for i := range c.inheritances {
		inheritanceCfg := &c.inheritances[i]
		c.collectLeafEntities(inheritanceCfg, c.root, nil, 0, result)
	}

	return result, nil
}

// String returns a string with the current representation of the configuration according to the YAML format.
func (c *Config) String() string {
	//nolint:godox
	// TODO: implement YAML serialization.
	return ""
}

// MarshalYAML serializes the Config object into YAML format.
// Thanks to key order preservation, the resulting YAML will have a predictable and stable structure.
func (c *Config) MarshalYAML() ([]byte, error) {
	//nolint:godox
	// TODO: implement YAML marshaling.
	return nil, nil
}

// collectLeafEntities recursively finds all leaf entities in the hierarchy
// and resolves their effective config.
//
// levelIdx: current level in the hierarchy (0 = global).
// currentPath: accumulated path segments so far.
func (c *Config) collectLeafEntities(
	inheritanceCfg *inheritanceConfig,
	node *tree.Node,
	currentPath keypath.KeyPath,
	levelIdx int,
	result map[string]Config,
) {
	// Determine if the next level is the leaf structural level.
	nextLevel := levelIdx + 1
	if nextLevel >= len(inheritanceCfg.levels) {
		// Should not happen because levelIdx starts at 0 and increments.
		return
	}

	structKey := inheritanceCfg.levels[nextLevel]

	structNode := node.Child(structKey)
	if structNode == nil {
		return
	}

	if nextLevel == len(inheritanceCfg.levels)-1 {
		// The next level is the leaf structural level.
		// Its named children are leaf entities.
		for _, name := range structNode.ChildrenKeys() {
			entityPath := currentPath.Append(structKey, name)

			layers, ok := matchHierarchy(c.root, inheritanceCfg, entityPath)
			if !ok {
				continue
			}

			resolved := resolveEffective(layers, inheritanceCfg)

			result[entityPath.String()] = newConfig(resolved, c.inheritances)
		}

		return
	}

	// Not leaf level; recurse into named children.
	for _, name := range structNode.ChildrenKeys() {
		namedNode := structNode.Child(name)
		if namedNode == nil {
			continue
		}

		childPath := currentPath.Append(structKey, name)
		c.collectLeafEntities(inheritanceCfg, namedNode, childPath, nextLevel, result)
	}
}

// MutableConfig is an extension of Config that allows safe runtime modifications.
type MutableConfig struct {
	Config // Embeds the read-only interface.

	// mu provides synchronization for thread-safe configuration changes.
	mu sync.RWMutex
	// validator validates configuration changes (Set/Merge/Update).
	validator validator.Validator
}

// Set sets or overwrites a value at the specified path.
// The key's metadata must be updated: Source becomes 'ModifiedSource', and Revision is incremented.
func (mc *MutableConfig) Set(path KeyPath, value any) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.root.Set(path, value)

	if mc.validator != nil {
		validationErrs := mc.validator.Validate(mc.root)
		if len(validationErrs) > 0 {
			node := mc.root.Get(path)
			if node != nil {
				node.Value = nil
				// TODO: properly revert to previous state (should be implemented in TNTP-5724).
			}

			return &validationErrs[0]
		}
	}

	node := mc.root.Get(path)
	if node != nil {
		node.Source = "modified"
		// TODO: increment revision (should be implemented in TNTP-5724).
	}

	return nil
}

// Merge merges two configurations so that all values from the new configuration
// are added or override similar values in the current one.
// An error may occur if the new values do not conform to the current schema.
func (mc *MutableConfig) Merge(other *Config) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	ctx := context.Background()

	ch, err := other.Walk(ctx, nil, -1)
	if err != nil {
		return err
	}

	for val := range ch {
		path := val.Meta().Key

		var dest any

		err := val.Get(&dest)
		if err != nil {
			return fmt.Errorf("failed to get value at path %s: %w", path, err)
		}

		mc.root.Set(path, dest)
	}

	if mc.validator != nil {
		validationErrs := mc.validator.Validate(mc.root)
		if len(validationErrs) > 0 {
			return &validationErrs[0]
		}
	}

	return nil
}

// Update merges two configurations, but applies only those values that already exist
// in the current config. Everything else is ignored.
// An error may occur if the new values do not match the type of the current value according to the schema.
func (mc *MutableConfig) Update(other *Config) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	ctx := context.Background()

	ch, err := other.Walk(ctx, nil, -1)
	if err != nil {
		return err
	}

	for val := range ch {
		keyPath := val.Meta().Key
		if mc.root.Get(keyPath) == nil {
			continue
		}

		var dest any

		err := val.Get(&dest)
		if err != nil {
			return fmt.Errorf("failed to get value at path %s: %w", keyPath, err)
		}

		mc.root.Set(keyPath, dest)
	}

	if mc.validator != nil {
		validationErrs := mc.validator.Validate(mc.root)
		if len(validationErrs) > 0 {
			return &validationErrs[0]
		}
	}

	return nil
}

// Delete removes a key from the configuration.
func (mc *MutableConfig) Delete(_ KeyPath) bool {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	//nolint:godox
	// TODO: not implemented (should be implemented in TNTP-5724).
	return false
}
