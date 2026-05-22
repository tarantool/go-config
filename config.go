package config

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	"github.com/tarantool/go-config/keypath"
	"github.com/tarantool/go-config/meta"
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
	// validator is the validator carried over from the Builder. It is
	// invoked on demand by [Config.Validate] (and, for MutableConfig,
	// by Set/Merge/Update via validateOrRestore).
	validator validator.Validator

	// layers holds the per-loader layer trees produced by Builder.Build, one
	// per top-level collector (or MultiCollector) in ascending-priority order.
	// Nil for configs not produced by Builder (slices, Walk/effective sub-configs).
	layers []*tree.Node

	// modified holds runtime mutations applied by MutableConfig; nil until the
	// first mutation.
	modified *tree.Node

	// tombstones records key paths deleted via MutableConfig.Delete.
	tombstones []keypath.KeyPath
}

// entityTombstoned reports whether entityPath, or one of its ancestor scopes,
// was deleted via MutableConfig.Delete (any tombstone that prefixes entityPath).
func entityTombstoned(tombstones []keypath.KeyPath, entityPath keypath.KeyPath) bool {
	for _, tomb := range tombstones {
		if len(tomb) > len(entityPath) {
			continue
		}

		match := true

		for i, seg := range tomb {
			if entityPath[i] != seg {
				match = false
				break
			}
		}

		if match {
			return true
		}
	}

	return false
}

// newConfig creates a Config from a tree node.
func newConfig(root *tree.Node, inheritances []inheritanceConfig, val validator.Validator) Config {
	return Config{
		root:         root,
		inheritances: inheritances,
		validator:    val,
		layers:       nil,
		modified:     nil,
		tombstones:   nil,
	}
}

// newLayeredConfig creates a Config that carries per-loader layer trees in
// addition to the merged root. Used by Builder.Build.
func newLayeredConfig(
	root *tree.Node,
	layers []*tree.Node,
	inheritances []inheritanceConfig,
	val validator.Validator,
) Config {
	return Config{
		root:         root,
		inheritances: inheritances,
		validator:    val,
		layers:       layers,
		modified:     nil,
		tombstones:   nil,
	}
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

// Validate runs the validator carried over from the [Builder] on the current
// configuration tree. It is intended for callers who used
// [Builder.WithoutValidation] and want to validate the assembled config later
// (e.g. after merging additional sources or restoring from a snapshot).
//
// Returns nil if no validator is attached or the tree is empty. On failure,
// returns the validation errors as a slice of *[validator.ValidationError]
// (matching the shape of [Builder.Build]). The tree itself is not modified.
func (c *Config) Validate() []error {
	if c.validator == nil || c.root == nil {
		return nil
	}

	validationErrs := c.validator.Validate(c.root)
	if len(validationErrs) == 0 {
		return nil
	}

	errs := make([]error, len(validationErrs))
	for i := range validationErrs {
		errs[i] = &validationErrs[i]
	}

	return errs
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
//
// The returned sub-Config does not carry the validator: the configured schema
// describes the full root, not arbitrary subtrees, so [Config.Validate] would
// be meaningless on a slice.
func (c *Config) Slice(path KeyPath) (Config, error) {
	if c.root == nil {
		if len(path) == 0 {
			return newConfig(nil, c.inheritances, nil), nil
		}

		return Config{}, fmt.Errorf("%w: %s", ErrPathNotFound, path)
	}

	if len(path) == 0 {
		return newConfig(c.root, c.inheritances, nil), nil
	}

	root := c.root.Get(path)
	if root == nil {
		return Config{}, fmt.Errorf("%w: %s", ErrPathNotFound, path)
	}

	return newConfig(root, c.inheritances, nil), nil
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

		resolved, matched, tombstoned := c.resolveEntityConfig(inheritanceCfg, path)
		if tombstoned {
			return Config{}, fmt.Errorf("%w: %s", ErrPathNotFound, path)
		}

		if matched {
			return resolved, nil
		}
	}

	// No hierarchy matched — fall back to raw subtree.
	node := c.root.Get(path)
	if node == nil {
		return Config{}, fmt.Errorf("%w: %s", ErrPathNotFound, path)
	}

	return newConfig(cloneNode(node), c.inheritances, nil), nil
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

// deepClone returns an independent copy: root, layers, modified and tombstones
// are copied; inheritances and validator are shared (read-only after Build).
func (c *Config) deepClone() Config {
	clonedLayers := make([]*tree.Node, len(c.layers))
	for i, layer := range c.layers {
		clonedLayers[i] = cloneNode(layer)
	}

	clonedTombstones := make([]keypath.KeyPath, len(c.tombstones))
	for i, kp := range c.tombstones {
		clonedTombstones[i] = append(keypath.KeyPath{}, kp...)
	}

	return Config{
		root:         cloneNode(c.root),
		inheritances: c.inheritances,
		validator:    c.validator,
		layers:       clonedLayers,
		modified:     cloneNode(c.modified),
		tombstones:   clonedTombstones,
	}
}

// resolveEntityConfig resolves the effective Config for entityPath under
// inheritanceCfg. It returns the resolved config (meaningful only when matched),
// whether entityPath fits the hierarchy, and whether it (or an ancestor scope)
// was deleted via MutableConfig.Delete (in which case matched is false).
func (c *Config) resolveEntityConfig(
	inheritanceCfg *inheritanceConfig,
	entityPath keypath.KeyPath,
) (Config, bool, bool) {
	if len(c.layers) == 0 {
		// Not produced by a Builder: single merged-tree resolution.
		layers, ok := matchHierarchy(c.root, inheritanceCfg, entityPath)
		if !ok {
			return newConfig(nil, nil, nil), false, false
		}

		return newConfig(resolveEffective(layers, inheritanceCfg), c.inheritances, nil), true, false
	}

	if _, ok := matchHierarchy(c.root, inheritanceCfg, entityPath); !ok {
		return newConfig(nil, nil, nil), false, false
	}

	if entityTombstoned(c.tombstones, entityPath) {
		return newConfig(nil, nil, nil), false, true
	}

	return newConfig(resolveEffectiveLayered(c, inheritanceCfg, entityPath), c.inheritances, nil), true, false
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

			resolved, matched, _ := c.resolveEntityConfig(inheritanceCfg, entityPath)
			if !matched {
				continue
			}

			result[entityPath.String()] = resolved
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
//
// The validator (if any) lives on the embedded [Config]. Set/Merge/Update use
// it to validate every mutation; [MutableConfig.Validate] re-runs it on the
// current tree on demand.
type MutableConfig struct {
	Config // Embeds the read-only interface.

	// mu provides synchronization for thread-safe configuration changes.
	mu sync.RWMutex
}

// nextRevision increments a revision string. Non-numeric or empty revisions start from "1".
func nextRevision(cur string) string {
	n, err := strconv.ParseUint(cur, 10, 64)
	if err != nil {
		n = 0
	}

	return strconv.FormatUint(n+1, 10)
}

// markModified updates a node's Source and Revision to reflect a runtime modification.
func markModified(node *tree.Node) {
	if node == nil {
		return
	}

	node.Source = meta.ModifiedSourceName
	node.Revision = nextRevision(node.Revision)
}

// setMutableValue replaces composite mutation values with an equivalent node
// subtree so maps and slices are not left as opaque leaves.
func setMutableValue(root *tree.Node, path keypath.KeyPath, value any) *tree.Node {
	switch value.(type) {
	case map[string]any, []any:
		replacement := mutableValueNode(value)
		if len(path) == 0 {
			return replacement
		}

		parentPath := path.Parent()
		if len(parentPath) > 0 {
			root.Set(parentPath, nil)
		}

		parent := root.Get(parentPath)
		parent.SetChild(path.Leaf(), replacement)
	default:
		root.Set(path, value)
	}

	return root
}

// mutableValueNode builds the tree representation used by runtime composite
// mutations. Empty composites keep their raw value so leaf reads still retain
// the empty map or slice type.
func mutableValueNode(value any) *tree.Node {
	node := tree.New()

	switch typedValue := value.(type) {
	case map[string]any:
		if len(typedValue) == 0 {
			node.Value = typedValue
			return node
		}

		for key, childValue := range typedValue {
			node.SetChild(key, mutableValueNode(childValue))
		}
	case []any:
		node.MarkArray()

		if len(typedValue) == 0 {
			node.Value = typedValue
			return node
		}

		for i, childValue := range typedValue {
			node.SetChild(strconv.Itoa(i), mutableValueNode(childValue))
		}
	default:
		node.Value = value
	}

	return node
}

// Get retrieves a value at the specified path with read-lock protection.
func (mc *MutableConfig) Get(path KeyPath, dest any) (MetaInfo, error) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	return mc.Config.Get(path, dest)
}

// Lookup searches for a value by key with read-lock protection.
func (mc *MutableConfig) Lookup(path KeyPath) (Value, bool) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	return mc.Config.Lookup(path)
}

// Stat returns metadata for a key with read-lock protection.
func (mc *MutableConfig) Stat(path KeyPath) (MetaInfo, bool) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	return mc.Config.Stat(path)
}

// Validate runs the configured validator on the current tree under the
// read-lock. See [Config.Validate] for semantics.
func (mc *MutableConfig) Validate() []error {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	return mc.Config.Validate()
}

// Walk returns a channel of all key-value pairs with read-lock protection.
// The tree is cloned under the lock so the channel can be consumed safely after unlock.
func (mc *MutableConfig) Walk(ctx context.Context, path KeyPath, depth int) (<-chan Value, error) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	snap := newConfig(cloneNode(mc.root), mc.inheritances, mc.validator)

	return snap.Walk(ctx, path, depth)
}

// Slice returns a sub-configuration at the given path with read-lock protection.
func (mc *MutableConfig) Slice(path KeyPath) (Config, error) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	return mc.Config.Slice(path)
}

// Effective returns the resolved config for a specific leaf entity with read-lock protection.
func (mc *MutableConfig) Effective(path KeyPath) (Config, error) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	return mc.Config.Effective(path)
}

// EffectiveAll returns resolved configs for all leaf entities with read-lock protection.
func (mc *MutableConfig) EffectiveAll() (map[string]Config, error) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	return mc.Config.EffectiveAll()
}

// Snapshot returns a deep copy of the current configuration as a read-only Config.
// The returned value is decoupled from the live MutableConfig, so concurrent mutations
// after Snapshot returns are not observed by the snapshot.
func (mc *MutableConfig) Snapshot() Config {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	return mc.deepClone()
}

// Set sets or overwrites a value at the specified path.
// The key's metadata is updated: Source becomes "modified", and Revision is incremented.
// If validation fails, the tree is restored to its previous state.
func (mc *MutableConfig) Set(path KeyPath, value any) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	oldRoot := cloneNode(mc.root)

	mc.root = setMutableValue(mc.root, path, value)

	restoreErr := mc.validateOrRestore(oldRoot)
	if restoreErr != nil {
		return restoreErr
	}

	markModified(mc.root.Get(path))

	// Record the mutation in the runtime overlay (it outranks every loader).
	if mc.modified == nil {
		mc.modified = tree.New()
	}

	mc.modified = setMutableValue(mc.modified, path, value)
	markModified(mc.modified.Get(path))

	return nil
}

// mergeOp represents a pending merge operation.
type mergeOp struct {
	path       keypath.KeyPath
	value      any
	arrayPaths []keypath.KeyPath
}

// materializeOps walks the other config and collects all leaf values as operations.
func materializeOps(other *Config) ([]mergeOp, error) {
	ctx := context.Background()

	valueChan, err := other.Walk(ctx, nil, -1)
	if err != nil {
		return nil, err
	}

	var ops []mergeOp

	for val := range valueChan {
		path := val.Meta().Key

		var dest any

		err := val.Get(&dest)
		if err != nil {
			return nil, fmt.Errorf("failed to get value at path %s: %w", path, err)
		}

		node := other.root.Get(path)
		if node != nil && node.IsArray() {
			dest = tree.ToAny(node)
		}

		ops = append(ops, mergeOp{
			path:       path,
			value:      dest,
			arrayPaths: arrayPaths(other.root, path),
		})
	}

	return ops, nil
}

// arrayPaths returns array nodes encountered from root through path. Merge
// replays leaves into the target tree, so this preserves sequence metadata
// when a replay creates an array path from scratch.
func arrayPaths(root *tree.Node, path keypath.KeyPath) []keypath.KeyPath {
	if root == nil {
		return nil
	}

	node := root
	current := keypath.KeyPath{}

	var paths []keypath.KeyPath

	if node.IsArray() {
		paths = append(paths, current)
	}

	for _, segment := range path {
		node = node.Child(segment)
		if node == nil {
			break
		}

		current = current.Append(segment)

		if node.IsArray() {
			paths = append(paths, current)
		}
	}

	return paths
}

// markArrayPaths reapplies source sequence metadata after a leaf replay.
func markArrayPaths(root *tree.Node, paths []keypath.KeyPath) {
	for _, path := range paths {
		node := root.Get(path)
		if node != nil {
			node.MarkArray()
		}
	}
}

// Merge merges two configurations so that all values from the new configuration
// are added or override similar values in the current one.
// If validation fails, the tree is restored to its previous state.
func (mc *MutableConfig) Merge(other *Config) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	ops, err := materializeOps(other)
	if err != nil {
		return err
	}

	oldRoot := cloneNode(mc.root)

	for _, mergeEntry := range ops {
		mc.root = setMutableValue(mc.root, mergeEntry.path, mergeEntry.value)
		markArrayPaths(mc.root, mergeEntry.arrayPaths)
	}

	restoreErr := mc.validateOrRestore(oldRoot)
	if restoreErr != nil {
		return restoreErr
	}

	// Record the mutations in the runtime overlay (it outranks every loader).
	if mc.modified == nil {
		mc.modified = tree.New()
	}

	for _, mergeEntry := range ops {
		markModified(mc.root.Get(mergeEntry.path))

		mc.modified = setMutableValue(mc.modified, mergeEntry.path, mergeEntry.value)
		markArrayPaths(mc.modified, mergeEntry.arrayPaths)
		markModified(mc.modified.Get(mergeEntry.path))
	}

	return nil
}

// Update merges two configurations, but applies only those values that already exist
// in the current config. Everything else is ignored.
// If validation fails, the tree is restored to its previous state.
func (mc *MutableConfig) Update(other *Config) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	ops, err := materializeOps(other)
	if err != nil {
		return err
	}

	oldRoot := cloneNode(mc.root)

	var applied []mergeOp

	for _, updateEntry := range ops {
		if mc.root.Get(updateEntry.path) == nil {
			continue
		}

		mc.root.Set(updateEntry.path, updateEntry.value)

		applied = append(applied, updateEntry)
	}

	restoreErr := mc.validateOrRestore(oldRoot)
	if restoreErr != nil {
		return restoreErr
	}

	// Record the mutations in the runtime overlay (it outranks every loader).
	if len(applied) > 0 && mc.modified == nil {
		mc.modified = tree.New()
	}

	for _, op := range applied {
		markModified(mc.root.Get(op.path))
		mc.modified.Set(op.path, op.value)
		markModified(mc.modified.Get(op.path))
	}

	return nil
}

// Delete removes a key (and its entire subtree) from the configuration.
// After removal, any ancestor maps that became empty are also removed,
// stopping at the first non-empty ancestor.
// Returns true if the key was found and deleted, false otherwise (idempotent).
// If validation fails after deletion, the tree is restored and false is returned.
func (mc *MutableConfig) Delete(path KeyPath) bool {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if mc.root == nil || len(path) == 0 {
		return false
	}

	// Idempotent: missing path is not an error, just a no-op.
	if mc.root.Get(path) == nil {
		return false
	}

	oldRoot := cloneNode(mc.root)

	// Cascade delete the target subtree and prune empty ancestors.
	pruneTreePath(mc.root, path)

	restoreErr := mc.validateOrRestore(oldRoot)
	if restoreErr != nil {
		return false
	}

	// Keep the overlay consistent with the live tree.
	pruneTreePath(mc.modified, path)

	// Record a tombstone so resolveEffectiveLayered suppresses this path in every layer.
	mc.tombstones = append(mc.tombstones, append(keypath.KeyPath{}, path...))

	return true
}

// validateOrRestore validates the current tree and restores the old root on failure.
func (mc *MutableConfig) validateOrRestore(oldRoot *tree.Node) error {
	if mc.validator == nil {
		return nil
	}

	validationErrs := mc.validator.Validate(mc.root)
	if len(validationErrs) > 0 {
		mc.root = oldRoot

		return &validationErrs[0]
	}

	return nil
}
