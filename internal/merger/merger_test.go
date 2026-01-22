package merger_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tarantool/go-config"
	"github.com/tarantool/go-config/collectors"
	"github.com/tarantool/go-config/tree"
)

func checkByPath(t *testing.T, root *tree.Node, keyPath string, expectedValue any) {
	t.Helper()

	node := root.Get(config.NewKeyPath(keyPath))
	require.NotNil(t, node)
	assert.Equal(t, expectedValue, node.Value)
}

func TestMergeCollector_PrimitiveSet(t *testing.T) {
	t.Parallel()

	root := tree.New()

	// Collector sets a primitive value.
	col := collectors.NewMap(map[string]any{
		"port": 8080,
	}).WithName("first").WithKeepOrder(false)

	require.NoError(t, config.MergeCollector(root, col))

	// Verify value.
	node := root.Get(config.NewKeyPath("port"))
	require.NotNil(t, node)
	assert.Equal(t, 8080, node.Value)
	assert.Equal(t, "first", node.Source)
}

func TestMergeCollector_PrimitiveOverride(t *testing.T) {
	t.Parallel()

	root := tree.New()

	// First collector sets a value.
	col1 := collectors.NewMap(map[string]any{
		"port": 8080,
	}).WithName("first").WithKeepOrder(false)

	require.NoError(t, config.MergeCollector(root, col1))

	// Second collector (higher priority) overrides.
	col2 := collectors.NewMap(map[string]any{
		"port": 9090,
	}).WithName("second").WithKeepOrder(false)

	require.NoError(t, config.MergeCollector(root, col2))

	// Verify overridden value.
	node := root.Get(config.NewKeyPath("port"))
	require.NotNil(t, node)
	assert.Equal(t, 9090, node.Value)
	assert.Equal(t, "second", node.Source)
}

func TestMergeCollector_SliceReplacement(t *testing.T) {
	t.Parallel()

	root := tree.New()

	col1 := collectors.NewMap(map[string]any{
		"items": []any{"a", "b", "c"},
	}).WithName("first")

	require.NoError(t, config.MergeCollector(root, col1))

	node := root.Get(config.NewKeyPath("items"))
	require.NotNil(t, node)

	val, ok := node.Value.([]any)
	require.True(t, ok)
	assert.Equal(t, []any{"a", "b", "c"}, val)

	// Higher priority collector replaces slice entirely.
	col2 := collectors.NewMap(map[string]any{
		"items": []any{"x", "y"},
	}).WithName("second")

	require.NoError(t, config.MergeCollector(root, col2))

	node = root.Get(config.NewKeyPath("items"))
	require.NotNil(t, node)

	val2, ok2 := node.Value.([]any)
	require.True(t, ok2)
	assert.Equal(t, []any{"x", "y"}, val2)
	assert.Equal(t, "second", node.Source)
}

func TestMergeCollector_MapRecursiveMerge(t *testing.T) {
	t.Parallel()

	root := tree.New()

	// First collector sets nested map.
	col1 := collectors.NewMap(map[string]any{
		"server": map[string]any{
			"port": 8080,
			"host": "localhost",
		},
	}).WithName("first")

	require.NoError(t, config.MergeCollector(root, col1))

	// Verify initial values.
	portNode := root.Get(config.NewKeyPath("server/port"))
	require.NotNil(t, portNode)
	assert.Equal(t, 8080, portNode.Value)

	hostNode := root.Get(config.NewKeyPath("server/host"))
	require.NotNil(t, hostNode)
	assert.Equal(t, "localhost", hostNode.Value)

	// Second collector overrides only one key, adds another.
	col2 := collectors.NewMap(map[string]any{
		"server": map[string]any{
			"port": 9090,
			"ssl":  true,
		},
	}).WithName("second")

	require.NoError(t, config.MergeCollector(root, col2))

	// Port should be overridden.
	portNode = root.Get(config.NewKeyPath("server/port"))
	require.NotNil(t, portNode)
	assert.Equal(t, 9090, portNode.Value)
	assert.Equal(t, "second", portNode.Source)

	// Host should remain (from first collector).
	hostNode = root.Get(config.NewKeyPath("server/host"))
	require.NotNil(t, hostNode)
	assert.Equal(t, "localhost", hostNode.Value)
	assert.Equal(t, "first", hostNode.Source)

	// SSL should be added.
	sslNode := root.Get(config.NewKeyPath("server/ssl"))
	require.NotNil(t, sslNode)
	assert.Equal(t, true, sslNode.Value)
	assert.Equal(t, "second", sslNode.Source)
}

func TestMergeCollector_OrderedCollectorSetsOrder(t *testing.T) {
	t.Parallel()

	root := tree.New()

	// Unordered collector adds keys in some order.
	col1 := collectors.NewMap(map[string]any{
		"a": 1,
		"b": 2,
		"c": 3,
		"d": 4,
	}).WithName("unordered").WithKeepOrder(false)

	require.NoError(t, config.MergeCollector(root, col1))

	parent := root.Get(config.NewKeyPath(""))
	require.NotNil(t, parent)

	initialKeys := parent.ChildrenKeys()
	// Unordered collector should not set order flag.
	assert.False(t, parent.OrderSet())
	// Ensure all keys are present.
	assert.Len(t, initialKeys, 4)
	assert.Subset(t, initialKeys, []string{"a", "b", "c", "d"})

	// Ordered collector provides a subset of keys in a different order.
	// Map collector with KeepOrder true sorts keys alphabetically.
	col2 := collectors.NewMap(map[string]any{
		"c": 30,
		"a": 10,
	}).WithName("ordered").WithKeepOrder(true)

	require.NoError(t, config.MergeCollector(root, col2))

	// Ordered keys are sorted alphabetically: ["a", "c"].
	// They should be moved to front while preserving relative order of other keys.
	// Compute expected order.
	orderedKeys := []string{"a", "c"}

	var expected []string
	// Add ordered keys first.
	expected = append(expected, orderedKeys...)
	// Add remaining keys in their original relative order.
	for _, k := range initialKeys {
		if k != "a" && k != "c" {
			expected = append(expected, k)
		}
	}

	keys := parent.ChildrenKeys()
	assert.Equal(t, expected, keys)

	checkByPath(t, root, "a", 10)
	checkByPath(t, root, "b", 2)
	checkByPath(t, root, "c", 30)
	checkByPath(t, root, "d", 4)
}

func TestMergeCollector_OrderSetFlagPreventsReordering(t *testing.T) {
	t.Parallel()

	root := tree.New()

	// First ordered collector sets order.
	col1 := collectors.NewMap(map[string]any{
		"x": 1,
		"y": 2,
		"z": 3,
	}).WithName("ordered1").WithKeepOrder(true)

	require.NoError(t, config.MergeCollector(root, col1))

	parent := root.Get(config.NewKeyPath(""))
	require.NotNil(t, parent)
	assert.Equal(t, []string{"x", "y", "z"}, parent.ChildrenKeys())
	assert.True(t, parent.OrderSet())

	// Second ordered collector (higher priority) attempts to reorder,
	// but orderSet flag should prevent reordering.
	col2 := collectors.NewMap(map[string]any{
		"z": 30,
		"x": 10,
	}).WithName("ordered2").WithKeepOrder(true)

	require.NoError(t, config.MergeCollector(root, col2))

	// Order should remain unchanged (x,y,z).
	keys := parent.ChildrenKeys()
	assert.Equal(t, []string{"x", "y", "z"}, keys)

	checkByPath(t, root, "x", 10)
	checkByPath(t, root, "y", 2)
	checkByPath(t, root, "z", 30)
}

func TestMergeCollector_UnorderedCollectorDoesNotAffectOrder(t *testing.T) {
	t.Parallel()

	root := tree.New()

	// Ordered collector sets initial order.
	col1 := collectors.NewMap(map[string]any{
		"first":  1,
		"second": 2,
		"third":  3,
	}).WithName("ordered").WithKeepOrder(true)

	require.NoError(t, config.MergeCollector(root, col1))

	parent := root.Get(config.NewKeyPath(""))
	keys := parent.ChildrenKeys()
	assert.Equal(t, []string{"first", "second", "third"}, keys)

	// Unordered collector adds a new key; it should be appended.
	col2 := collectors.NewMap(map[string]any{
		"fourth": 4,
	}).WithName("unordered").WithKeepOrder(false)

	require.NoError(t, config.MergeCollector(root, col2))

	keys = parent.ChildrenKeys()
	assert.Equal(t, []string{"first", "second", "third", "fourth"}, keys)

	// Unordered collector updates existing value, order unchanged.
	col3 := collectors.NewMap(map[string]any{
		"second": 200,
	}).WithName("unordered2").WithKeepOrder(false)

	require.NoError(t, config.MergeCollector(root, col3))

	keys = parent.ChildrenKeys()
	assert.Equal(t, []string{"first", "second", "third", "fourth"}, keys)
	assert.Equal(t, 200, root.Get(config.NewKeyPath("second")).Value)
}

func TestMergeCollector_LeafToMapConversion(t *testing.T) {
	t.Parallel()

	root := tree.New()

	// Set a primitive value.
	col1 := collectors.NewMap(map[string]any{
		"foo": "bar",
	}).WithName("primitive")

	require.NoError(t, config.MergeCollector(root, col1))

	node := root.Get(config.NewKeyPath("foo"))
	require.NotNil(t, node)
	assert.Equal(t, "bar", node.Value)
	assert.True(t, node.IsLeaf())

	// Replace with a map (higher priority).
	col2 := collectors.NewMap(map[string]any{
		"foo": map[string]any{
			"nested": "value",
		},
	}).WithName("map")

	require.NoError(t, config.MergeCollector(root, col2))

	node = root.Get(config.NewKeyPath("foo"))
	require.NotNil(t, node)
	assert.False(t, node.IsLeaf())
	// Original value should be cleared.
	assert.Nil(t, node.Value)

	child := node.Get(config.NewKeyPath("nested"))
	require.NotNil(t, child)
	assert.Equal(t, "value", child.Value)
}

func TestMergeCollector_MapToLeafConversion(t *testing.T) {
	t.Parallel()

	root := tree.New()

	// Set a map.
	col1 := collectors.NewMap(map[string]any{
		"foo": map[string]any{
			"nested": "value",
		},
	}).WithName("map")

	require.NoError(t, config.MergeCollector(root, col1))

	node := root.Get(config.NewKeyPath("foo"))
	require.NotNil(t, node)
	assert.False(t, node.IsLeaf())

	// Replace with a primitive (higher priority).
	col2 := collectors.NewMap(map[string]any{
		"foo": 42,
	}).WithName("primitive")

	require.NoError(t, config.MergeCollector(root, col2))

	node = root.Get(config.NewKeyPath("foo"))
	require.NotNil(t, node)
	assert.True(t, node.IsLeaf())
	assert.Equal(t, 42, node.Value)
	// Children should be cleared.
	assert.Nil(t, node.Children())
}
