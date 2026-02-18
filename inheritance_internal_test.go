package config

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tarantool/go-config/keypath"
	"github.com/tarantool/go-config/tree"
)

func TestMatchHierarchy(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(NewKeyPath("groups/storages/replicasets/s-001/instances/s-001-a/foo"), "bar")

	cfg := &inheritanceConfig{
		levels:          Levels(Global, "groups", "replicasets", "instances"),
		defaults:        nil,
		noInherit:       nil,
		noInheritFrom:   nil,
		mergeStrategies: nil,
	}

	layers, ok := matchHierarchy(root, cfg, NewKeyPath("groups/storages/replicasets/s-001/instances/s-001-a"))
	require.True(t, ok)
	require.NotNil(t, layers)
	require.Len(t, layers, 4)
	require.Equal(t, root, layers[0])
}

func TestResolveEffectiveSimple(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(NewKeyPath("groups/storages/replicasets/s-001/instances/s-001-a/foo"), "bar")

	cfg := &inheritanceConfig{
		levels:          Levels(Global, "groups", "replicasets", "instances"),
		defaults:        nil,
		noInherit:       nil,
		noInheritFrom:   nil,
		mergeStrategies: nil,
	}

	layers, ok := matchHierarchy(root, cfg, NewKeyPath("groups/storages/replicasets/s-001/instances/s-001-a"))
	require.True(t, ok)

	result := resolveEffective(layers, cfg)
	require.NotNil(t, result)

	val := result.Child("foo")
	require.NotNil(t, val)
	require.Equal(t, "bar", val.Value)
}

func TestKeyMatchesPrefix(t *testing.T) {
	t.Parallel()

	assert.True(t, keyMatchesPrefix(NewKeyPath("a/b/c"), NewKeyPath("a")))
	assert.True(t, keyMatchesPrefix(NewKeyPath("a/b/c"), NewKeyPath("a/b")))
	assert.True(t, keyMatchesPrefix(NewKeyPath("a/b/c"), NewKeyPath("a/b/c")))
	assert.False(t, keyMatchesPrefix(NewKeyPath("a/b/c"), NewKeyPath("a/b/c/d")))
	assert.False(t, keyMatchesPrefix(NewKeyPath("a/b/c"), NewKeyPath("a/x")))

	assert.True(t, keyMatchesPrefix(NewKeyPath("a/b/c"), NewKeyPath("")))
}

func TestIsStructuralKey(t *testing.T) {
	t.Parallel()

	cfg := &inheritanceConfig{
		levels:          Levels(Global, "groups", "replicasets", "instances"),
		defaults:        nil,
		noInherit:       nil,
		noInheritFrom:   nil,
		mergeStrategies: nil,
	}
	assert.True(t, isStructuralKey(cfg, "groups"))
	assert.True(t, isStructuralKey(cfg, "replicasets"))
	assert.True(t, isStructuralKey(cfg, "instances"))
	assert.True(t, isStructuralKey(cfg, Global))
	assert.False(t, isStructuralKey(cfg, "foo"))
	assert.False(t, isStructuralKey(cfg, "credentials"))
}

func TestCloneNode(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(NewKeyPath("a/b"), "value1")
	root.Set(NewKeyPath("a/c"), "value2")
	root.Set(NewKeyPath("d"), "value3")

	clone := cloneNode(root)
	require.NotNil(t, clone)

	node1 := clone.Get(NewKeyPath("a/b"))
	require.NotNil(t, node1)
	assert.Equal(t, "value1", node1.Value)

	node2 := clone.Get(NewKeyPath("a/c"))
	require.NotNil(t, node2)
	assert.Equal(t, "value2", node2.Value)

	node3 := clone.Get(NewKeyPath("d"))
	require.NotNil(t, node3)
	assert.Equal(t, "value3", node3.Value)

	originalNode := root.Get(NewKeyPath("a"))

	cloneNode := clone.Get(NewKeyPath("a"))
	assert.NotSame(t, originalNode, cloneNode, "cloneNode should create a deep copy, same pointer")
}

func TestCloneNode_Nil(t *testing.T) {
	t.Parallel()

	result := cloneNode(nil)
	assert.Nil(t, result)
}

func TestIsSliceNode_EdgeCases(t *testing.T) {
	t.Parallel()

	// Nil node.
	assert.False(t, isSliceNode(nil))
	// Non-leaf node (map).
	mapNode := tree.New()
	child := tree.New()

	child.Value = "value"
	mapNode.SetChild("key", child)
	assert.False(t, isSliceNode(mapNode))
	// Leaf node with slice value.
	sliceNode := tree.New()

	sliceNode.Value = []any{1, 2}
	assert.True(t, isSliceNode(sliceNode))
	// Leaf node with non-slice value.
	leafNode := tree.New()

	leafNode.Value = "string"
	assert.False(t, isSliceNode(leafNode))
}

func TestShouldInherit_LevelExclusionPrefixMatch(t *testing.T) {
	t.Parallel()

	cfg := &inheritanceConfig{
		levels:    Levels(Global, "groups", "replicasets", "instances"),
		defaults:  nil,
		noInherit: nil,
		noInheritFrom: map[int][]keypath.KeyPath{
			1: {NewKeyPath("groups/storages")},
		},
		mergeStrategies: nil,
	}

	assert.False(t, cfg.shouldInherit(1, NewKeyPath("groups/storages")))

	assert.False(t, cfg.shouldInherit(1, NewKeyPath("groups/storages/replicasets")))

	assert.True(t, cfg.shouldInherit(1, NewKeyPath("groups/databases")))

	assert.True(t, cfg.shouldInherit(2, NewKeyPath("groups/storages")))
}

func TestMergeIntoResult_EdgeCases(t *testing.T) {
	t.Parallel()

	// Helper to create leaf node with value.
	newLeaf := func(v any) *tree.Node {
		n := tree.New()

		n.Value = v

		return n
	}

	// Helper to create map node with child.
	newMap := func(key string, child *tree.Node) *tree.Node {
		n := tree.New()
		n.SetChild(key, child)

		return n
	}

	t.Run("MergeDeep_ExistingMapSourceLeaf", func(t *testing.T) {
		t.Parallel()

		result := newMap("key", newLeaf("existing"))
		source := newLeaf("source")
		mergeIntoResult(result, "key", source, MergeDeep)
		// Should replace with source leaf (fallback).
		child := result.Child("key")
		require.NotNil(t, child)
		assert.Equal(t, "source", child.Value)
	})

	t.Run("MergeDeep_ExistingLeafSourceMap", func(t *testing.T) {
		t.Parallel()

		result := newLeaf("existing")
		source := newMap("nested", newLeaf("value"))
		mergeIntoResult(result, "key", source, MergeDeep)
		// Should replace with source map (fallback).
		keyChild := result.Child("key")
		require.NotNil(t, keyChild)

		nestedChild := keyChild.Child("nested")
		require.NotNil(t, nestedChild)
		assert.Equal(t, "value", nestedChild.Value)
	})

	t.Run("MergeDeep_BothLeaf", func(t *testing.T) {
		t.Parallel()

		result := newLeaf("existing")
		source := newLeaf("source")
		mergeIntoResult(result, "key", source, MergeDeep)
		// Should replace with source leaf (fallback).
		child := result.Child("key")
		require.NotNil(t, child)
		assert.Equal(t, "source", child.Value)
	})

	t.Run("MergeDeep_BothMap", func(t *testing.T) {
		t.Parallel()

		result := newMap("nested", newLeaf("existing"))
		source := newMap("nested", newLeaf("source"))
		mergeIntoResult(result, "key", source, MergeDeep)
		// Should deep merge nested maps.

		keyChild := result.Child("key")
		require.NotNil(t, keyChild)

		nestedChild := keyChild.Child("nested")
		require.NotNil(t, nestedChild)
		assert.Equal(t, "source", nestedChild.Value)
	})

	t.Run("MergeAppend_ExistingNil", func(t *testing.T) {
		t.Parallel()

		result := tree.New()
		source := newLeaf([]any{1, 2})
		mergeIntoResult(result, "key", source, MergeAppend)
		// Should replace with source slice (existing nil).
		require.NotNil(t, result.Child("key"))

		slice, ok := result.Child("key").Value.([]any)
		assert.True(t, ok)
		assert.Equal(t, []any{1, 2}, slice)
	})

	t.Run("MergeAppend_ExistingNotSlice", func(t *testing.T) {
		t.Parallel()

		result := newMap("key", newLeaf("existing"))
		source := newLeaf([]any{1, 2})
		mergeIntoResult(result, "key", source, MergeAppend)
		// Should replace with source slice (existing not slice).
		require.NotNil(t, result.Child("key"))

		slice, ok := result.Child("key").Value.([]any)
		assert.True(t, ok)
		assert.Equal(t, []any{1, 2}, slice)
	})

	t.Run("MergeAppend_SourceNotSlice", func(t *testing.T) {
		t.Parallel()

		result := newLeaf([]any{1, 2})
		source := newLeaf("not-slice")
		mergeIntoResult(result, "key", source, MergeAppend)
		// Should replace with source leaf (source not slice).
		child := result.Child("key")
		require.NotNil(t, child)
		assert.Equal(t, "not-slice", child.Value)
	})

	t.Run("MergeAppend_SliceTypeAssertionFail_ExistingConcrete", func(t *testing.T) {
		t.Parallel()

		// Create a slice node with concrete slice type (not []any).
		// isSliceNode returns true but type assertion to []any fails.
		existing := newLeaf([]int{1, 2}) // Concrete slice type.
		result := newMap("key", existing)
		source := newLeaf([]any{3, 4})
		mergeIntoResult(result, "key", source, MergeAppend)
		// Should fall back to replace with source slice.
		child := result.Child("key")
		require.NotNil(t, child)

		slice, ok := child.Value.([]any)
		assert.True(t, ok)
		assert.Equal(t, []any{3, 4}, slice)
	})

	t.Run("MergeAppend_SliceTypeAssertionFail_SourceConcrete", func(t *testing.T) {
		t.Parallel()
		// Source slice is concrete type, existing is []any.
		existing := newLeaf([]any{1, 2})
		result := newMap("key", existing)
		source := newLeaf([]int{3, 4}) // Concrete slice type.
		mergeIntoResult(result, "key", source, MergeAppend)
		// Should fall back to replace with source slice.
		child := result.Child("key")
		require.NotNil(t, child)
		// Source is cloned, value remains concrete type []int.
		slice, ok := child.Value.([]int)
		assert.True(t, ok)
		assert.Equal(t, []int{3, 4}, slice)
	})

	t.Run("DeepMergeNodes_SourceLeafDoesNothing", func(t *testing.T) {
		t.Parallel()

		dst := newMap("key", newLeaf("dst-value"))
		originalValue := dst.Value
		originalChild := dst.Child("key")
		src := newLeaf("src-value")
		deepMergeNodes(dst, src)
		// Source leaf has no children, so nothing should change.
		assert.Equal(t, originalValue, dst.Value)
		assert.Equal(t, originalChild, dst.Child("key"))
	})

	t.Run("DeepMergeNodes_SourceMapDstLeaf", func(t *testing.T) {
		t.Parallel()

		dst := newLeaf("dst-value")
		src := newMap("key", newLeaf("src-value"))
		deepMergeNodes(dst, src)
		// Source map has children, dst leaf gains children but retains its value (maybe weird but allowed).
		require.NotNil(t, dst.Child("key"))
		assert.Equal(t, "src-value", dst.Child("key").Value)
		// dst.Value remains unchanged (still leaf value).
		assert.Equal(t, "dst-value", dst.Value)
	})

	t.Run("DeepMergeNodes_BothLeaf", func(t *testing.T) {
		t.Parallel()

		dst := newLeaf("dst-value")
		src := newLeaf("src-value")
		deepMergeNodes(dst, src)
		// Both leaf, source has no children, nothing changes.
		assert.Equal(t, "dst-value", dst.Value)
	})

	t.Run("DeepMergeNodes_LeafChildVsMapChild", func(t *testing.T) {
		t.Parallel()

		// Both dst and src are maps, but for key "k", dst child is leaf, src child is map.
		dst := newMap("k", newLeaf("dst-leaf"))
		src := newMap("k", newMap("nested", newLeaf("src-nested")))
		deepMergeNodes(dst, src)
		// Since src child is map and dst child is leaf, source wins (line 425)
		// dst child at "k" should be replaced with clone of src child (map).

		keyChild := dst.Child("k")
		require.NotNil(t, keyChild)
		assert.False(t, keyChild.IsLeaf())

		nestedChild := keyChild.Child("nested")
		require.NotNil(t, nestedChild)
		assert.Equal(t, "src-nested", nestedChild.Value)
	})

	t.Run("DeepMergeNodes_MapChildVsLeafChild", func(t *testing.T) {
		t.Parallel()

		dst := newMap("k", newMap("nested", newLeaf("dst-nested")))
		src := newMap("k", newLeaf("src-leaf"))
		deepMergeNodes(dst, src)
		// dst child is map, src child is leaf, source wins (line 425).
		keyChild := dst.Child("k")
		require.NotNil(t, keyChild)
		assert.True(t, keyChild.IsLeaf())
		assert.Equal(t, "src-leaf", keyChild.Value)
	})

	t.Run("DeepMergeNodes_BothMap", func(t *testing.T) {
		t.Parallel()

		dst := newMap("common", newLeaf("dst"))
		dst.SetChild("dst-only", newLeaf("dst-only"))

		src := newMap("common", newLeaf("src"))
		src.SetChild("src-only", newLeaf("src-only"))
		deepMergeNodes(dst, src)
		// Should merge: common key replaced, src-only added, dst-only retained.

		commonChild := dst.Child("common")
		require.NotNil(t, commonChild)
		assert.Equal(t, "src", commonChild.Value)

		dstOnlyChild := dst.Child("dst-only")
		require.NotNil(t, dstOnlyChild)
		assert.Equal(t, "dst-only", dstOnlyChild.Value)

		srcOnlyChild := dst.Child("src-only")
		require.NotNil(t, srcOnlyChild)
		assert.Equal(t, "src-only", srcOnlyChild.Value)
	})
}

func TestWalkNodes_CtxCancelled(t *testing.T) {
	t.Parallel()
	// Create a leaf node.
	node := tree.New()

	node.Value = "test"

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	valueCh := make(chan Value, 1)

	go func() {
		defer close(valueCh)

		walkNodes(ctx, node, NewKeyPath(""), -1, valueCh)
	}()
	// Channel should be closed, may have zero or one values.
	val, ok := <-valueCh
	if ok {
		// One value received; channel should now be closed.
		_, ok = <-valueCh
		assert.False(t, ok)
		// Optional: check val.
		_ = val
	}
}
