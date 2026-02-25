package config

import (
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/tarantool/go-config/keypath"
	"github.com/tarantool/go-config/tree"
)

const segmentsPerLevel = 2

// InheritMergeStrategy defines how a specific key is merged during inheritance
// resolution (NOT during collector merging — that is handled by Merger).
type InheritMergeStrategy int

const (
	// MergeReplace replaces the value from the parent with the child's value.
	// This is the default strategy for all keys.
	MergeReplace InheritMergeStrategy = iota
	// MergeAppend appends child slice elements to the parent's slice.
	// If the value is not a slice, behaves like MergeReplace.
	MergeAppend
	// MergeDeep recursively merges maps from parent and child.
	// Child keys override parent keys; parent-only keys are preserved.
	// If the value is not a map, behaves like MergeReplace.
	MergeDeep
)

// Global is a sentinel value for Levels() indicating the root (global) level.
const Global = ""

// Levels defines the hierarchy of structural keys.
// The first argument must be Global (empty string) to represent the root level.
// Subsequent arguments are the structural keys in order from top to bottom.
//
// Example:
//
//	config.Levels(config.Global, "groups", "replicasets", "instances")
//
// This defines 4 levels:
//   - Level 0 (Global):     root node, config keys live here
//   - Level 1 (Group):      under root/groups/<name>
//   - Level 2 (Replicaset): under root/groups/<name>/replicasets/<name>
//   - Level 3 (Instance):   under root/groups/<name>/replicasets/<name>/instances/<name>
func Levels(levels ...string) []string {
	if len(levels) == 0 {
		panic("Levels requires at least one argument (Global)")
	}

	if levels[0] != Global {
		panic("first argument to Levels must be config.Global")
	}

	return levels
}

// InheritanceOption configures inheritance behavior.
// Created via With* functions.
type InheritanceOption func(*inheritanceConfig)

// inheritanceConfig holds the full inheritance configuration for one hierarchy.
type inheritanceConfig struct {
	// levels defines the structural keys that separate hierarchy levels.
	// The first element is a sentinel for the global level (empty string).
	// Example: ["", "groups", "replicasets", "instances"].
	levels []string

	// defaults are applied at the leaf level with lowest priority.
	defaults DefaultsType

	// noInherit contains config key prefixes that are never inherited down.
	// Example: ["leader", "iproto.listen"].
	noInherit []keypath.KeyPath

	// noInheritFrom maps a level index to config key prefixes that should not
	// be inherited FROM that specific level.
	// Example: {0: ["snapshot.dir"]} means snapshot.dir set at global level
	// is not inherited by groups/replicasets/instances.
	noInheritFrom map[int][]keypath.KeyPath

	// mergeStrategies maps config key prefixes to their merge strategy
	// during inheritance resolution.
	mergeStrategies map[string]InheritMergeStrategy
}

// WithDefaults sets default values applied to every resolved leaf entity.
// Defaults have the lowest priority — any value from any level overrides them.
func WithDefaults(defaults DefaultsType) InheritanceOption {
	return func(ic *inheritanceConfig) {
		ic.defaults = defaults
	}
}

// WithNoInherit marks config key prefixes that are never propagated down
// the hierarchy from any level. The key is excluded from inheritance
// entirely — it only applies at the level where it is explicitly set.
//
// Example: WithNoInherit("leader") means "leader" set at the replicaset
// level is NOT copied into instance configs during inheritance resolution.
// It remains accessible at its original path in the raw tree.
func WithNoInherit(keys ...string) InheritanceOption {
	return func(ic *inheritanceConfig) {
		for _, k := range keys {
			ic.noInherit = append(ic.noInherit, keypath.NewKeyPath(k))
		}
	}
}

// WithNoInheritFrom marks config key prefixes that should not be inherited
// FROM a specific level. The level is identified by its structural key
// (or Global for the root level).
//
// Example:
//
//	WithNoInheritFrom(config.Global, "snapshot.dir")
//
// means snapshot.dir set at the global level does not flow into
// group/replicaset/instance levels. But snapshot.dir set at the group
// level DOES flow into replicaset/instance levels.
func WithNoInheritFrom(level string, keys ...string) InheritanceOption {
	return func(inheritanceCfg *inheritanceConfig) {
		// Find level index.
		levelIdx := -1

		for i, l := range inheritanceCfg.levels {
			if l == level {
				levelIdx = i
				break
			}
		}

		if levelIdx == -1 {
			panic(fmt.Sprintf("level %q not found in hierarchy", level))
		}

		if inheritanceCfg.noInheritFrom == nil {
			inheritanceCfg.noInheritFrom = make(map[int][]keypath.KeyPath)
		}

		for _, k := range keys {
			kp := keypath.NewKeyPath(k)

			inheritanceCfg.noInheritFrom[levelIdx] = append(inheritanceCfg.noInheritFrom[levelIdx], kp)
		}
	}
}

// WithInheritMerge sets the merge strategy for a specific config key prefix
// during inheritance resolution. This controls how parent and child values
// are combined when both define the same key.
//
// Example:
//
//	WithInheritMerge("roles", config.MergeAppend)
//
// means when group has roles=[storage] and instance has roles=[metrics],
// the effective roles for that instance will be [storage, metrics].
func WithInheritMerge(key string, strategy InheritMergeStrategy) InheritanceOption {
	return func(ic *inheritanceConfig) {
		if ic.mergeStrategies == nil {
			ic.mergeStrategies = make(map[string]InheritMergeStrategy)
		}

		ic.mergeStrategies[key] = strategy
	}
}

// cloneNode creates a deep copy of a tree node and all its descendants.
func cloneNode(node *tree.Node) *tree.Node {
	if node == nil {
		return nil
	}

	clone := tree.New()

	clone.Value = node.Value
	clone.Source = node.Source
	clone.Revision = node.Revision

	for _, key := range node.ChildrenKeys() {
		clone.SetChild(key, cloneNode(node.Child(key)))
	}

	return clone
}

// keyMatchesPrefix checks if a key path matches a prefix path.
// Prefix matching: prefix must be exact prefix segments.
func keyMatchesPrefix(key, prefix keypath.KeyPath) bool {
	if len(prefix) > len(key) {
		return false
	}

	for i := range prefix {
		if prefix[i] != key[i] {
			return false
		}
	}

	return true
}

// isStructuralKey checks if a key is a structural key in the hierarchy.
func isStructuralKey(inheritanceCfg *inheritanceConfig, key string) bool {
	return slices.Contains(inheritanceCfg.levels, key)
}

// matchHierarchy checks if a path matches a registered hierarchy and returns
// the level nodes (global, group, replicaset, instance) if it does.
//
// Path: "groups/storages/replicasets/s-001/instances/s-001-a"
// Levels: ["", "groups", "replicasets", "instances"]
//
// Expected path structure: <level1_key>/<name>/<level2_key>/<name>/...
// Level 0 (Global) is the root node itself.
//
// Returns: [rootNode, groupNode, replicasetNode, instanceNode], true
//
//	or: nil, false
func matchHierarchy(root *tree.Node, inheritanceCfg *inheritanceConfig, keyPath keypath.KeyPath) ([]*tree.Node, bool) {
	numLevels := len(inheritanceCfg.levels)

	// Path must have exactly (numLevels-1)*segmentsPerLevel segments:
	//   groups/storages/replicasets/s-001/instances/s-001-a = 6 segments for 4 levels.
	expectedLen := (numLevels - 1) * segmentsPerLevel
	if len(keyPath) != expectedLen {
		return nil, false
	}

	layers := make([]*tree.Node, numLevels)

	layers[0] = root

	// Traverse path pairwise: (structural_key, name), (structural_key, name), ...
	currentNode := root

	for i := 1; i < numLevels; i++ {
		pathIdx := (i - 1) * segmentsPerLevel
		structKey := keyPath[pathIdx]
		name := keyPath[pathIdx+1]

		// Verify structural key matches.
		if structKey != inheritanceCfg.levels[i] {
			return nil, false
		}

		// If currentNode is nil, we cannot find children; set layer to nil and continue.
		if currentNode == nil {
			layers[i] = nil
			// currentNode stays nil for next levels.
			continue
		}

		structNode := currentNode.Child(structKey)
		if structNode == nil {
			layers[i] = nil
			currentNode = nil

			continue
		}

		namedNode := structNode.Child(name)
		if namedNode == nil {
			layers[i] = nil
			currentNode = nil

			continue
		}

		layers[i] = namedNode
		currentNode = namedNode
	}

	// Return true even if some layers are nil, as long as the path pattern matches.
	return layers, true
}

// shouldInherit determines whether a key at a given level should be inherited.
func (inheritanceCfg *inheritanceConfig) shouldInherit(levelIdx int, key keypath.KeyPath) bool {
	// Check global exclusions (WithNoInherit).
	for _, excluded := range inheritanceCfg.noInherit {
		if keyMatchesPrefix(key, excluded) {
			return false
		}
	}

	// Check level-specific exclusions (WithNoInheritFrom).
	if levelExclusions, ok := inheritanceCfg.noInheritFrom[levelIdx]; ok {
		for _, excluded := range levelExclusions {
			if keyMatchesPrefix(key, excluded) {
				return false
			}
		}
	}

	return true
}

// strategyFor returns the merge strategy for a key and whether it was
// explicitly registered. When explicit is false, the returned strategy
// is the default (MergeReplace).
func (inheritanceCfg *inheritanceConfig) strategyFor(key string) (InheritMergeStrategy, bool) {
	if inheritanceCfg.mergeStrategies != nil {
		if strategy, ok := inheritanceCfg.mergeStrategies[key]; ok {
			return strategy, true
		}
	}

	return MergeReplace, false
}

// hasSubStrategies checks if any merge strategy is registered for a sub-path
// of the given prefix (e.g., prefix "credentials" matches "credentials/users").
func (inheritanceCfg *inheritanceConfig) hasSubStrategies(prefix string) bool {
	if inheritanceCfg.mergeStrategies == nil {
		return false
	}

	sub := prefix + "/"

	for k := range inheritanceCfg.mergeStrategies {
		if strings.HasPrefix(k, sub) {
			return true
		}
	}

	return false
}

// resolveEffective merges layers from global to leaf with inheritance rules.
func resolveEffective(layers []*tree.Node, inheritanceCfg *inheritanceConfig) *tree.Node {
	result := tree.New()

	// Start with defaults (lowest priority).
	if inheritanceCfg.defaults != nil {
		mergeDefaults(result, inheritanceCfg.defaults)
	}

	// Merge each layer in order (global first, leaf last = highest priority).
	for levelIdx, layer := range layers {
		if layer == nil {
			continue
		}

		for _, key := range layer.ChildrenKeys() {
			keyPath := keypath.NewKeyPath(key)

			// Skip structural keys.
			if isStructuralKey(inheritanceCfg, key) {
				continue
			}

			// Apply exclusion rules.
			if !inheritanceCfg.shouldInherit(levelIdx, keyPath) {
				// Only include if this is the level where it was explicitly set.
				// For WithNoInherit: only at the LEAF level (last layer).
				// For WithNoInheritFrom: skip only from the excluded level.
				if levelIdx < len(layers)-1 {
					continue
				}
			}

			child := layer.Child(key)
			mergeIntoResultWithStrategies(result, key, child, inheritanceCfg)
		}
	}

	return result
}

// mergeDefaults merges default values into the result node.
func mergeDefaults(result *tree.Node, defaults DefaultsType) {
	for k, v := range defaults {
		setDefaultsRecursive(result, keypath.NewKeyPath(k), v)
	}
}

// setDefaultsRecursive recursively walks nested maps and sets leaf values.
//

func setDefaultsRecursive(node *tree.Node, prefix keypath.KeyPath, val any) {
	switch val := val.(type) {
	case map[string]any:
		for childKey, childVal := range val {
			newPrefix := prefix.Append(childKey)
			setDefaultsRecursive(node, newPrefix, childVal)
		}
	default:
		node.Set(prefix, val)
	}
}

// isSliceNode returns true if node is a leaf with a slice value.
func isSliceNode(n *tree.Node) bool {
	if n == nil || !n.IsLeaf() {
		return false
	}

	return reflect.TypeOf(n.Value).Kind() == reflect.Slice
}

// isMapNode returns true if node is a non-leaf node (has children).
func isMapNode(n *tree.Node) bool {
	return n != nil && !n.IsLeaf()
}

// mergeIntoResult merges a single key's subtree into the result node
// using the specified inheritance merge strategy.
func mergeIntoResult(result *tree.Node, key string, source *tree.Node, strategy InheritMergeStrategy) {
	existing := result.Child(key)

	switch strategy {
	case MergeReplace:
		// Child completely replaces parent. Simple: set/overwrite.
		result.SetChild(key, cloneNode(source))

	case MergeAppend:
		// Append slice elements.
		if existing == nil || !isSliceNode(existing) || !isSliceNode(source) {
			// Fallback to replace if not both slices.
			result.SetChild(key, cloneNode(source))
			return
		}

		existingSlice, ok1 := existing.Value.([]any)

		sourceSlice, ok2 := source.Value.([]any)
		if !ok1 || !ok2 {
			// Can happen when slice is not []any (e.g., []int).
			result.SetChild(key, cloneNode(source))

			return
		}

		merged := make([]any, 0, len(existingSlice)+len(sourceSlice))

		merged = append(merged, existingSlice...)
		merged = append(merged, sourceSlice...)
		existing.Value = merged
		// Note: we modify existing node in place, which is okay because
		// it's already part of the result tree (not the raw tree).

	case MergeDeep:
		// Recursive map merge.
		if existing == nil {
			result.SetChild(key, cloneNode(source))
			return
		}

		// Both must be non-leaf (map) nodes.
		if isMapNode(existing) && isMapNode(source) {
			deepMergeNodes(existing, source)
		} else {
			// Fallback to replace if not both maps.
			result.SetChild(key, cloneNode(source))
		}
	}
}

// deepMergeNodes recursively merges source children into dst.
// Source keys override dst keys for leaves; recurse for nested maps.
func deepMergeNodes(dst, source *tree.Node) {
	for _, key := range source.ChildrenKeys() {
		srcChild := source.Child(key)
		dstChild := dst.Child(key)

		if dstChild == nil {
			dst.SetChild(key, cloneNode(srcChild))
			continue
		}

		// Both non-leaf: recurse.
		if !dstChild.IsLeaf() && !srcChild.IsLeaf() {
			deepMergeNodes(dstChild, srcChild)
			continue
		}

		// Otherwise source wins.
		dst.SetChild(key, cloneNode(srcChild))
	}
}

// mergeIntoResultWithStrategies merges a key's subtree into result, handling
// nested merge strategies for sub-paths (e.g., strategy for "credentials/users"
// when merging the "credentials" key).
func mergeIntoResultWithStrategies(
	result *tree.Node, key string, source *tree.Node, inheritanceCfg *inheritanceConfig,
) {
	strategy, _ := inheritanceCfg.strategyFor(key)

	if !inheritanceCfg.hasSubStrategies(key) {
		// No nested strategies — use direct merge.
		mergeIntoResult(result, key, source, strategy)
		return
	}

	// There are nested strategies under this key.
	existing := result.Child(key)
	if existing == nil || !isMapNode(existing) || !isMapNode(source) {
		// Cannot walk children — fall back to this level's strategy.
		mergeIntoResult(result, key, source, strategy)
		return
	}

	// Recursively merge, applying strategies at the correct depth.
	// The current level's strategy becomes the default for children.
	strategyAwareMerge(existing, source, key, strategy, inheritanceCfg)
}

// strategyAwareMerge recursively merges source into dst, checking for nested
// merge strategies at each level identified by pathPrefix. Children without
// an explicit strategy inherit defaultStrategy from their parent.
func strategyAwareMerge(
	dst, src *tree.Node, pathPrefix string, defaultStrategy InheritMergeStrategy,
	inheritanceCfg *inheritanceConfig,
) {
	for _, childKey := range src.ChildrenKeys() {
		childPath := pathPrefix + "/" + childKey
		srcChild := src.Child(childKey)

		strategy, explicit := inheritanceCfg.strategyFor(childPath)
		if !explicit {
			strategy = defaultStrategy
		}

		if !inheritanceCfg.hasSubStrategies(childPath) {
			// Apply the strategy directly at this level.
			mergeIntoResult(dst, childKey, srcChild, strategy)
			continue
		}

		// Need to recurse deeper for nested strategies.
		dstChild := dst.Child(childKey)
		if dstChild == nil || !isMapNode(dstChild) || !isMapNode(srcChild) {
			mergeIntoResult(dst, childKey, srcChild, strategy)
		} else {
			strategyAwareMerge(dstChild, srcChild, childPath, strategy, inheritanceCfg)
		}
	}
}
