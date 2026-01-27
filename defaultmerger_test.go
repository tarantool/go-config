package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tarantool/go-config"
	"github.com/tarantool/go-config/collectors"
	"github.com/tarantool/go-config/tree"
)

func TestDefaultMerger_CreateContext(t *testing.T) {
	t.Parallel()

	merger := &config.DefaultMerger{}

	// Test with KeepOrder false.
	col1 := collectors.NewMap(map[string]any{"key": "value"}).
		WithName("test").WithKeepOrder(false)
	ctx1 := merger.CreateContext(col1)
	require.NotNil(t, ctx1)
	assert.Equal(t, col1, ctx1.Collector())

	// Test with KeepOrder true.
	col2 := collectors.NewMap(map[string]any{"key": "value"}).
		WithName("test").WithKeepOrder(true)
	ctx2 := merger.CreateContext(col2)
	require.NotNil(t, ctx2)
	assert.Equal(t, col2, ctx2.Collector())
}

func TestDefaultMergerContext_RecordOrdering(t *testing.T) {
	t.Parallel()

	merger := &config.DefaultMerger{}

	// Test with KeepOrder false - RecordOrdering should be no-op.
	col1 := collectors.NewMap(map[string]any{}).WithName("test").WithKeepOrder(false)
	ctx1 := merger.CreateContext(col1)
	require.NotNil(t, ctx1)

	// Call RecordOrdering - should not panic.
	ctx1.RecordOrdering(config.NewKeyPath("parent"), "child")
	ctx1.RecordOrdering(config.NewKeyPath(""), "rootChild")

	// Test with KeepOrder true - should record ordering.
	col2 := collectors.NewMap(map[string]any{}).WithName("test").WithKeepOrder(true)
	ctx2 := merger.CreateContext(col2)
	require.NotNil(t, ctx2)

	// Record a few orderings.
	ctx2.RecordOrdering(config.NewKeyPath("parent"), "child1")
	ctx2.RecordOrdering(config.NewKeyPath("parent"), "child2")
	ctx2.RecordOrdering(config.NewKeyPath(""), "root1")
	ctx2.RecordOrdering(config.NewKeyPath(""), "root2")

	// Apply ordering to verify they were recorded.
	root := tree.New()
	// Need to create parent node for "parent" path.
	parentNode := tree.New()
	root.SetChild("parent", parentNode)
	// Also need child nodes to reorder (they will be created when merging).
	// For now just test ApplyOrdering doesn't error.
	err := ctx2.ApplyOrdering(root)
	require.NoError(t, err)
}

func TestDefaultMergerContext_ApplyOrdering(t *testing.T) {
	t.Parallel()

	merger := &config.DefaultMerger{}

	// Test with KeepOrder false - parentOrders is nil, ApplyOrdering should be no-op.
	col1 := collectors.NewMap(map[string]any{}).WithName("test").WithKeepOrder(false)
	ctx1 := merger.CreateContext(col1)
	require.NotNil(t, ctx1)

	root := tree.New()
	err := ctx1.ApplyOrdering(root)
	require.NoError(t, err)

	// Test with KeepOrder true but empty parentOrders.
	col2 := collectors.NewMap(map[string]any{}).WithName("test").WithKeepOrder(true)
	ctx2 := merger.CreateContext(col2)
	require.NotNil(t, ctx2)

	err = ctx2.ApplyOrdering(root)
	require.NoError(t, err)

	// Test with parentOrders containing entries.
	col3 := collectors.NewMap(map[string]any{}).WithName("test").WithKeepOrder(true)
	ctx3 := merger.CreateContext(col3)
	require.NotNil(t, ctx3)

	// Record ordering for root and nested parent.
	ctx3.RecordOrdering(config.NewKeyPath(""), "a")
	ctx3.RecordOrdering(config.NewKeyPath(""), "c")
	ctx3.RecordOrdering(config.NewKeyPath(""), "b")
	ctx3.RecordOrdering(config.NewKeyPath("parent"), "x")
	ctx3.RecordOrdering(config.NewKeyPath("parent"), "z")
	ctx3.RecordOrdering(config.NewKeyPath("parent"), "y")

	// Create tree with children (must exist for reorder to work).
	root = tree.New()
	// Add root children.
	root.SetChild("a", tree.New())
	root.SetChild("b", tree.New())
	root.SetChild("c", tree.New())
	// Add parent node and its children.
	parentNode := tree.New()
	root.SetChild("parent", parentNode)
	parentNode.SetChild("x", tree.New())
	parentNode.SetChild("y", tree.New())
	parentNode.SetChild("z", tree.New())

	// Ensure OrderSet is false initially.
	assert.False(t, root.OrderSet())
	assert.False(t, parentNode.OrderSet())

	// Apply ordering.
	err = ctx3.ApplyOrdering(root)
	require.NoError(t, err)

	// Verify ordering.
	assert.Equal(t, []string{"a", "c", "b", "parent"}, root.ChildrenKeys())
	assert.Equal(t, []string{"x", "z", "y"}, parentNode.ChildrenKeys())
	assert.True(t, root.OrderSet())
	assert.True(t, parentNode.OrderSet())

	// Test with parentNode nil (should skip).
	col4 := collectors.NewMap(map[string]any{}).WithName("test").WithKeepOrder(true)
	ctx4 := merger.CreateContext(col4)
	require.NotNil(t, ctx4)

	ctx4.RecordOrdering(config.NewKeyPath("nonexistent"), "child")

	root = tree.New()
	err = ctx4.ApplyOrdering(root)
	require.NoError(t, err) // Should skip, not error.

	// Test with OrderSet already true (should skip reordering).
	col5 := collectors.NewMap(map[string]any{}).WithName("test").WithKeepOrder(true)
	ctx5 := merger.CreateContext(col5)
	require.NotNil(t, ctx5)

	ctx5.RecordOrdering(config.NewKeyPath(""), "b")
	ctx5.RecordOrdering(config.NewKeyPath(""), "a")

	root = tree.New()
	root.SetChild("a", tree.New())
	root.SetChild("b", tree.New())
	root.SetOrderSet(true)

	originalOrder := root.ChildrenKeys()

	err = ctx5.ApplyOrdering(root)
	require.NoError(t, err)
	// Order should remain unchanged.
	assert.Equal(t, originalOrder, root.ChildrenKeys())
}

func TestDefaultMerger_DefaultInstance(t *testing.T) {
	t.Parallel()

	// Verify Default is not nil and is of type *DefaultMerger.
	assert.NotNil(t, config.Default)
	assert.IsType(t, &config.DefaultMerger{}, config.Default)

	// Quick sanity check that CreateContext works.
	col := collectors.NewMap(map[string]any{"key": "value"}).WithName("test")
	ctx := config.Default.CreateContext(col)
	require.NotNil(t, ctx)
	assert.Equal(t, col, ctx.Collector())

	// Verify MergeValue works.

	root := tree.New()
	err := config.Default.MergeValue(ctx, root, config.NewKeyPath("key"), "value")
	require.NoError(t, err)

	node := root.Get(config.NewKeyPath("key"))
	require.NotNil(t, node)
	assert.Equal(t, "value", node.Value)
}
