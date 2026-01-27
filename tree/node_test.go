package tree_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tarantool/go-config/path"
	"github.com/tarantool/go-config/tree"
)

func TestNode_Set_Get_leaf(t *testing.T) {
	t.Parallel()

	root := tree.New()

	root.Set(path.NewKeyPath("a/b/c"), 42)
	root.Set(path.NewKeyPath("a/b/d"), "hello")
	root.Set(path.NewKeyPath("x/y"), true)

	node := root.Get(path.NewKeyPath("a/b/c"))
	require.NotNil(t, node)
	assert.True(t, node.IsLeaf())
	assert.Equal(t, 42, node.Value)

	node = root.Get(path.NewKeyPath("a/b/d"))
	require.NotNil(t, node)
	assert.Equal(t, "hello", node.Value)

	node = root.Get(path.NewKeyPath("x/y"))
	require.NotNil(t, node)
	assert.Equal(t, true, node.Value)
}

func TestNode_Set_Get_nonLeaf(t *testing.T) {
	t.Parallel()

	root := tree.New()

	root.Set(path.NewKeyPath("a/b/c"), 42)
	root.Set(path.NewKeyPath("a/b/d"), "hello")
	root.Set(path.NewKeyPath("x/y"), true)

	node := root.Get(path.NewKeyPath("a/b"))
	require.NotNil(t, node)
	assert.False(t, node.IsLeaf())
}

func TestNode_Set_Get_missing(t *testing.T) {
	t.Parallel()

	root := tree.New()

	root.Set(path.NewKeyPath("a/b/c"), 42)
	root.Set(path.NewKeyPath("a/b/d"), "hello")
	root.Set(path.NewKeyPath("x/y"), true)

	node := root.Get(path.NewKeyPath("nonexistent"))
	assert.Nil(t, node)
}

func TestNode_Get_emptyPath(t *testing.T) {
	t.Parallel()

	root := tree.New()

	root.Value = "test value"

	node := root.Get(path.KeyPath{})
	require.NotNil(t, node)
	assert.Equal(t, "test value", node.Value)
}

func TestNode_IsLeaf_true(t *testing.T) {
	t.Parallel()

	node := tree.New()
	assert.True(t, node.IsLeaf())
}

func TestNode_IsLeaf_false(t *testing.T) {
	t.Parallel()

	node := tree.New()
	node.SetChild("child", tree.New())
	assert.False(t, node.IsLeaf())
}

func TestNode_IsLeaf_withValue(t *testing.T) {
	t.Parallel()

	node := tree.New()

	node.Value = "some value"
	assert.True(t, node.IsLeaf())
}

func TestNode_IsLeaf_nilValue(t *testing.T) {
	t.Parallel()

	node := tree.New()

	node.Value = nil
	assert.True(t, node.IsLeaf())
}

func TestNode_IsLeaf_childrenInitializedButEmpty(t *testing.T) {
	t.Parallel()

	node := tree.New()
	node.SetChild("child", tree.New())
	node.DeleteChild("child")
	assert.True(t, node.IsLeaf())
}

func TestNode_Child_existing(t *testing.T) {
	t.Parallel()

	node := tree.New()
	child := tree.New()

	child.Value = "child value"
	node.SetChild("key", child)

	result := node.Child("key")
	require.NotNil(t, result)
	assert.Equal(t, "child value", result.Value)
}

func TestNode_Child_missing(t *testing.T) {
	t.Parallel()

	node := tree.New()
	node.SetChild("other", tree.New())

	result := node.Child("missing")
	assert.Nil(t, result)
}

func TestNode_Child_nilChildren(t *testing.T) {
	t.Parallel()

	node := tree.New()
	result := node.Child("any")
	assert.Nil(t, result)
}

func TestNode_GetValue_leaf(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("a/b"), 100)

	val := root.GetValue(path.NewKeyPath("a/b"))
	assert.Equal(t, 100, val)
}

func TestNode_GetValue_nonLeaf(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("a/b"), 100)

	val := root.GetValue(path.NewKeyPath("a"))
	assert.Nil(t, val)
}

func TestNode_GetValue_missing(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("a/b"), 100)

	val := root.GetValue(path.NewKeyPath("missing"))
	assert.Nil(t, val)
}

func TestNode_Set_Overwrite(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("a/b"), "first")
	root.Set(path.NewKeyPath("a/b"), "second")

	node := root.Get(path.NewKeyPath("a/b"))
	require.NotNil(t, node)
	assert.Equal(t, "second", node.Value)
}

func TestNode_Set_overwriteLeafToNonLeaf(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("a"), "value")

	node := root.Get(path.NewKeyPath("a"))
	require.NotNil(t, node)
	assert.Equal(t, "value", node.Value)
	assert.True(t, node.IsLeaf())

	node.SetChild("b", tree.New())

	node = root.Get(path.NewKeyPath("a"))
	require.NotNil(t, node)
	assert.Equal(t, "value", node.Value)
	assert.False(t, node.IsLeaf())
}

func TestNode_Set_overwriteNonLeafToLeaf(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.SetChild("a", tree.New())

	nodeA := root.Child("a")
	nodeA.SetChild("b", tree.New())

	node := root.Get(path.NewKeyPath("a"))
	require.NotNil(t, node)
	assert.False(t, node.IsLeaf())

	root.Set(path.NewKeyPath("a"), "value")

	node = root.Get(path.NewKeyPath("a"))
	require.NotNil(t, node)
	assert.Equal(t, "value", node.Value)
	assert.False(t, node.IsLeaf())
}

func TestNode_ChildrenOrder_length(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("a/x"), 1)
	root.Set(path.NewKeyPath("a/y"), 2)
	root.Set(path.NewKeyPath("a/z"), 3)

	parentNode := root.Get(path.NewKeyPath("a"))
	require.NotNil(t, parentNode)

	children := parentNode.Children()
	assert.Len(t, children, 3)
}

func TestNode_ChildrenOrder_values(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("a/x"), 1)
	root.Set(path.NewKeyPath("a/y"), 2)
	root.Set(path.NewKeyPath("a/z"), 3)

	parentNode := root.Get(path.NewKeyPath("a"))
	require.NotNil(t, parentNode)

	children := parentNode.Children()
	assert.Equal(t, 1, children[0].Value)
	assert.Equal(t, 2, children[1].Value)
	assert.Equal(t, 3, children[2].Value)
}

func TestNode_ChildrenOrder_keys(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("a/x"), 1)
	root.Set(path.NewKeyPath("a/y"), 2)
	root.Set(path.NewKeyPath("a/z"), 3)

	parentNode := root.Get(path.NewKeyPath("a"))
	require.NotNil(t, parentNode)

	keys := parentNode.ChildrenKeys()
	assert.Equal(t, []string{"x", "y", "z"}, keys)
}

func TestNode_Children_nil(t *testing.T) {
	t.Parallel()

	node := tree.New()
	assert.Nil(t, node.Children())
}

func TestNode_Children_initializedButEmpty(t *testing.T) {
	t.Parallel()

	node := tree.New()
	node.SetChild("child", tree.New())
	node.DeleteChild("child")
	assert.Nil(t, node.Children())
}

func TestNode_ChildrenKeys_nil(t *testing.T) {
	t.Parallel()

	node := tree.New()
	assert.Nil(t, node.ChildrenKeys())
}

func TestNode_ChildrenKeys_initializedButEmpty(t *testing.T) {
	t.Parallel()

	node := tree.New()
	node.SetChild("child", tree.New())
	node.DeleteChild("child")
	assert.Nil(t, node.ChildrenKeys())
}

func TestNode_SetChild_ReplacePreservesOrder_order(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.SetChild("a", tree.New())
	root.SetChild("b", tree.New())
	root.SetChild("c", tree.New())

	keys := root.ChildrenKeys()
	assert.Equal(t, []string{"a", "b", "c"}, keys)

	newNode := tree.New()

	newNode.Value = "replaced"
	root.SetChild("b", newNode)

	keys = root.ChildrenKeys()
	assert.Equal(t, []string{"a", "b", "c"}, keys)
}

func TestNode_SetChild_ReplacePreservesOrder_value(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.SetChild("a", tree.New())
	root.SetChild("b", tree.New())
	root.SetChild("c", tree.New())

	newNode := tree.New()

	newNode.Value = "replaced"
	root.SetChild("b", newNode)

	child := root.Child("b")
	require.NotNil(t, child)
	assert.Equal(t, "replaced", child.Value)
}

func TestNode_SetChild_nilChild(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.SetChild("a", nil)

	child := root.Child("a")
	assert.Nil(t, child)
}

func TestNode_SetChild_overwriteNil(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.SetChild("a", tree.New())

	child := root.Child("a")
	require.NotNil(t, child)

	root.SetChild("a", nil)

	child = root.Child("a")
	assert.Nil(t, child)
}

func TestNode_DeleteChild_existing(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.SetChild("a", tree.New())
	root.SetChild("b", tree.New())
	root.SetChild("c", tree.New())

	ok := root.DeleteChild("b")
	assert.True(t, ok)

	keys := root.ChildrenKeys()
	assert.Equal(t, []string{"a", "c"}, keys)
	assert.Nil(t, root.Child("b"))
}

func TestNode_DeleteChild_missing(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.SetChild("a", tree.New())
	root.SetChild("b", tree.New())
	root.SetChild("c", tree.New())

	ok := root.DeleteChild("missing")
	assert.False(t, ok)
}

func TestNode_DeleteChild_NilChildren(t *testing.T) {
	t.Parallel()

	root := tree.New()

	ok := root.DeleteChild("any")
	assert.False(t, ok)
}

func TestNode_SourceRevision(t *testing.T) {
	t.Parallel()

	root := tree.New()

	root.Source = "file"
	root.Revision = "123"

	root.Set(path.NewKeyPath("a/b"), "value")

	node := root.Get(path.NewKeyPath("a/b"))
	require.NotNil(t, node)
	assert.Equal(t, "value", node.Value)
	assert.Empty(t, node.Source)
	assert.Empty(t, node.Revision)

	node.Source = "env"
	node.Revision = "456"
	assert.Equal(t, "env", node.Source)
	assert.Equal(t, "456", node.Revision)
}

func TestNode_EmptyPath(t *testing.T) {
	t.Parallel()

	root := tree.New()

	root.Value = "root value"
	root.Set(path.KeyPath{}, "new root")

	assert.Equal(t, "new root", root.Value)
}

func TestNode_IntermediateNodesCreated(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("deep/nested/path"), 99)

	deep := root.Get(path.NewKeyPath("deep"))
	require.NotNil(t, deep)
	assert.False(t, deep.IsLeaf())

	nested := deep.Get(path.NewKeyPath("nested"))
	require.NotNil(t, nested)

	leaf := root.Get(path.NewKeyPath("deep/nested/path"))
	require.NotNil(t, leaf)
	assert.Equal(t, 99, leaf.Value)
}

func TestNode_PathEmptySegment(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("a//c"), 42)

	node := root.Get(path.NewKeyPath("a//c"))
	require.NotNil(t, node)
	assert.Equal(t, 42, node.Value)

	emptyNode := root.Get(path.NewKeyPath("a/"))
	require.NotNil(t, emptyNode)
	assert.False(t, emptyNode.IsLeaf())
}

func TestNode_HasChild_Existing(t *testing.T) {
	t.Parallel()

	node := tree.New()
	node.SetChild("child", tree.New())

	result := node.HasChild("child")
	assert.True(t, result)
}

func TestNode_HasChild_Missing(t *testing.T) {
	t.Parallel()

	node := tree.New()
	node.SetChild("other", tree.New())

	result := node.HasChild("missing")
	assert.False(t, result)
}

func TestNode_HasChild_NilChildren(t *testing.T) {
	t.Parallel()

	node := tree.New()

	result := node.HasChild("any")
	assert.False(t, result)
}

func TestNode_ClearChildren_NilChildren(t *testing.T) {
	t.Parallel()

	node := tree.New()

	node.SetOrderSet(true)

	node.ClearChildren()

	assert.False(t, node.OrderSet())
	assert.Nil(t, node.Children())
}

func TestNode_ClearChildren_InitializedEmpty(t *testing.T) {
	t.Parallel()

	node := tree.New()
	node.SetChild("temp", tree.New())
	node.DeleteChild("temp")
	node.SetOrderSet(true)

	node.ClearChildren()

	assert.False(t, node.OrderSet())
	assert.Nil(t, node.Children())
}

func TestNode_ClearChildren_WithChildren(t *testing.T) {
	t.Parallel()

	node := tree.New()
	node.SetChild("a", tree.New())
	node.SetChild("b", tree.New())
	node.SetOrderSet(true)

	node.ClearChildren()

	assert.False(t, node.OrderSet())
	assert.Nil(t, node.Children())
	assert.False(t, node.HasChild("a"))
	assert.False(t, node.HasChild("b"))
}

func TestNode_OrderSet_DefaultFalse(t *testing.T) {
	t.Parallel()

	node := tree.New()
	assert.False(t, node.OrderSet())
}

func TestNode_OrderSet_SetTrue(t *testing.T) {
	t.Parallel()

	node := tree.New()
	node.SetOrderSet(true)
	assert.True(t, node.OrderSet())
}

func TestNode_OrderSet_SetFalse(t *testing.T) {
	t.Parallel()

	node := tree.New()
	node.SetOrderSet(true)
	node.SetOrderSet(false)
	assert.False(t, node.OrderSet())
}

func TestNode_ReorderChildren(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		setup       func(*tree.Node)
		keys        []string
		expected    []string
		description string
	}{
		{
			name: "nil children",
			setup: func(_ *tree.Node) {

			},
			keys:        []string{"a", "b"},
			expected:    nil,
			description: "should do nothing",
		},
		{
			name: "empty children",
			setup: func(n *tree.Node) {
				n.SetChild("temp", tree.New())
				n.DeleteChild("temp")
			},
			keys:        []string{"a", "b"},
			expected:    nil,
			description: "should do nothing",
		},
		{
			name: "empty keys",
			setup: func(n *tree.Node) {
				n.SetChild("a", tree.New())
				n.SetChild("b", tree.New())
			},
			keys:        []string{},
			expected:    []string{"a", "b"},
			description: "should preserve order",
		},
		{
			name: "reorder subset",
			setup: func(n *tree.Node) {
				n.SetChild("a", tree.New())
				n.SetChild("b", tree.New())
				n.SetChild("c", tree.New())
				n.SetChild("d", tree.New())
			},
			keys:        []string{"c", "a"},
			expected:    []string{"c", "a", "b", "d"},
			description: "should move c and a to front preserving input order",
		},
		{
			name: "reorder all",
			setup: func(n *tree.Node) {
				n.SetChild("a", tree.New())
				n.SetChild("b", tree.New())
				n.SetChild("c", tree.New())
			},
			keys:        []string{"c", "b", "a"},
			expected:    []string{"c", "b", "a"},
			description: "should reorder completely",
		},
		{
			name: "keys not present ignored",
			setup: func(n *tree.Node) {
				n.SetChild("a", tree.New())
				n.SetChild("b", tree.New())
			},
			keys:        []string{"x", "a", "y"},
			expected:    []string{"a", "b"},
			description: "should ignore x and y, move a to front",
		},
		{
			name: "duplicate keys in input",
			setup: func(n *tree.Node) {
				n.SetChild("a", tree.New())
				n.SetChild("b", tree.New())
				n.SetChild("c", tree.New())
			},
			keys:        []string{"b", "b", "a"},
			expected:    []string{"b", "a", "c"},
			description: "duplicates should be handled (first occurrence used)",
		},
		{
			name: "preserve values",
			setup: func(node *tree.Node) {
				childA := tree.New()

				childA.Value = "valueA"
				node.SetChild("a", childA)

				childB := tree.New()

				childB.Value = "valueB"
				node.SetChild("b", childB)
			},
			keys:        []string{"b", "a"},
			expected:    []string{"b", "a"},
			description: "values should remain after reorder",
		},
		{
			name: "no keys present",
			setup: func(n *tree.Node) {
				n.SetChild("a", tree.New())
				n.SetChild("b", tree.New())
			},
			keys:        []string{"x", "y"},
			expected:    []string{"a", "b"},
			description: "should ignore all keys, order unchanged",
		},
		{
			name: "single child reorder",
			setup: func(n *tree.Node) {
				n.SetChild("only", tree.New())
			},
			keys:        []string{"only"},
			expected:    []string{"only"},
			description: "single child remains",
		},
		{
			name: "nil keys slice",
			setup: func(n *tree.Node) {
				n.SetChild("a", tree.New())
				n.SetChild("b", tree.New())
			},
			keys:        nil,
			expected:    []string{"a", "b"},
			description: "nil keys treated as empty",
		},
		{
			name: "complex reorder three keys",
			setup: func(n *tree.Node) {
				n.SetChild("x", tree.New())
				n.SetChild("y", tree.New())
				n.SetChild("z", tree.New())
			},
			keys:        []string{"z", "x", "y"},
			expected:    []string{"z", "x", "y"},
			description: "should reorder three keys",
		},
		{
			name: "reorder two keys opposite",
			setup: func(n *tree.Node) {
				n.SetChild("first", tree.New())
				n.SetChild("second", tree.New())
			},
			keys:        []string{"second", "first"},
			expected:    []string{"second", "first"},
			description: "should swap order",
		},
		{
			name: "orderSet flag unchanged",
			setup: func(n *tree.Node) {
				n.SetChild("a", tree.New())
				n.SetChild("b", tree.New())
				n.SetOrderSet(true)
			},
			keys:        []string{"b", "a"},
			expected:    []string{"b", "a"},
			description: "orderSet flag should remain true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			node := tree.New()
			tt.setup(node)

			beforeOrderSet := node.OrderSet()
			err := node.ReorderChildren(tt.keys)
			require.NoError(t, err)

			afterOrderSet := node.OrderSet()
			assert.Equal(t, beforeOrderSet, afterOrderSet)

			keys := node.ChildrenKeys()
			if tt.expected == nil {
				assert.Nil(t, keys)
			} else {
				assert.Equal(t, tt.expected, keys)
			}

			for key, child := range map[string]*tree.Node{
				"a": node.Child("a"),
				"b": node.Child("b"),
				"c": node.Child("c"),
				"d": node.Child("d"),
			} {
				if child != nil {
					assert.NotNil(t, node.Child(key))
				}
			}
		})
	}
}
