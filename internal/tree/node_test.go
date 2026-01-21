package tree_test

import (
	"testing"

	"github.com/shoenig/test"
	"github.com/shoenig/test/must"

	"github.com/tarantool/go-config/internal/tree"
	"github.com/tarantool/go-config/path"
)

func TestNode_Set_Get_leaf(t *testing.T) {
	t.Parallel()

	root := tree.New()

	root.Set(path.NewKeyPath("a/b/c"), 42)
	root.Set(path.NewKeyPath("a/b/d"), "hello")
	root.Set(path.NewKeyPath("x/y"), true)

	node := root.Get(path.NewKeyPath("a/b/c"))
	must.NotNil(t, node)
	test.True(t, node.IsLeaf())
	test.Eq(t, 42, node.Value)

	node = root.Get(path.NewKeyPath("a/b/d"))
	must.NotNil(t, node)
	test.Eq(t, "hello", node.Value)

	node = root.Get(path.NewKeyPath("x/y"))
	must.NotNil(t, node)
	test.Eq(t, true, node.Value)
}

func TestNode_Set_Get_nonLeaf(t *testing.T) {
	t.Parallel()

	root := tree.New()

	root.Set(path.NewKeyPath("a/b/c"), 42)
	root.Set(path.NewKeyPath("a/b/d"), "hello")
	root.Set(path.NewKeyPath("x/y"), true)

	node := root.Get(path.NewKeyPath("a/b"))
	must.NotNil(t, node)
	test.False(t, node.IsLeaf())
}

func TestNode_Set_Get_missing(t *testing.T) {
	t.Parallel()

	root := tree.New()

	root.Set(path.NewKeyPath("a/b/c"), 42)
	root.Set(path.NewKeyPath("a/b/d"), "hello")
	root.Set(path.NewKeyPath("x/y"), true)

	node := root.Get(path.NewKeyPath("nonexistent"))
	test.Nil(t, node)
}

func TestNode_Get_emptyPath(t *testing.T) {
	t.Parallel()

	root := tree.New()

	root.Value = "test value"

	node := root.Get(path.KeyPath{})
	must.NotNil(t, node)
	test.Eq(t, "test value", node.Value)
}

func TestNode_IsLeaf_true(t *testing.T) {
	t.Parallel()

	node := tree.New()
	test.True(t, node.IsLeaf())
}

func TestNode_IsLeaf_false(t *testing.T) {
	t.Parallel()

	node := tree.New()
	node.SetChild("child", tree.New())
	test.False(t, node.IsLeaf())
}

func TestNode_IsLeaf_withValue(t *testing.T) {
	t.Parallel()

	node := tree.New()

	node.Value = "some value"
	test.True(t, node.IsLeaf())
}

func TestNode_IsLeaf_nilValue(t *testing.T) {
	t.Parallel()

	node := tree.New()

	node.Value = nil
	test.True(t, node.IsLeaf())
}

func TestNode_IsLeaf_childrenInitializedButEmpty(t *testing.T) {
	t.Parallel()

	node := tree.New()
	node.SetChild("child", tree.New())
	node.DeleteChild("child")
	test.True(t, node.IsLeaf())
}

func TestNode_Child_existing(t *testing.T) {
	t.Parallel()

	node := tree.New()
	child := tree.New()

	child.Value = "child value"
	node.SetChild("key", child)

	result := node.Child("key")
	must.NotNil(t, result)
	test.Eq(t, "child value", result.Value)
}

func TestNode_Child_missing(t *testing.T) {
	t.Parallel()

	node := tree.New()
	node.SetChild("other", tree.New())

	result := node.Child("missing")
	test.Nil(t, result)
}

func TestNode_Child_nilChildren(t *testing.T) {
	t.Parallel()

	node := tree.New()
	result := node.Child("any")
	test.Nil(t, result)
}

func TestNode_GetValue_leaf(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("a/b"), 100)

	val := root.GetValue(path.NewKeyPath("a/b"))
	test.Eq(t, 100, val)
}

func TestNode_GetValue_nonLeaf(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("a/b"), 100)

	val := root.GetValue(path.NewKeyPath("a"))
	test.Nil(t, val)
}

func TestNode_GetValue_missing(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("a/b"), 100)

	val := root.GetValue(path.NewKeyPath("missing"))
	test.Nil(t, val)
}

func TestNode_Set_Overwrite(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("a/b"), "first")
	root.Set(path.NewKeyPath("a/b"), "second")

	node := root.Get(path.NewKeyPath("a/b"))
	must.NotNil(t, node)
	test.Eq(t, "second", node.Value)
}

func TestNode_Set_overwriteLeafToNonLeaf(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("a"), "value")

	node := root.Get(path.NewKeyPath("a"))
	must.NotNil(t, node)
	test.Eq(t, "value", node.Value)
	test.True(t, node.IsLeaf())

	node.SetChild("b", tree.New())

	node = root.Get(path.NewKeyPath("a"))
	must.NotNil(t, node)
	test.Eq(t, "value", node.Value)
	test.False(t, node.IsLeaf())
}

func TestNode_Set_overwriteNonLeafToLeaf(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.SetChild("a", tree.New())

	nodeA := root.Child("a")
	nodeA.SetChild("b", tree.New())

	node := root.Get(path.NewKeyPath("a"))
	must.NotNil(t, node)
	test.False(t, node.IsLeaf())

	root.Set(path.NewKeyPath("a"), "value")

	node = root.Get(path.NewKeyPath("a"))
	must.NotNil(t, node)
	test.Eq(t, "value", node.Value)
	test.False(t, node.IsLeaf())
}

func TestNode_ChildrenOrder_length(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("a/x"), 1)
	root.Set(path.NewKeyPath("a/y"), 2)
	root.Set(path.NewKeyPath("a/z"), 3)

	parentNode := root.Get(path.NewKeyPath("a"))
	must.NotNil(t, parentNode)

	children := parentNode.Children()
	must.Eq(t, 3, len(children))
}

func TestNode_ChildrenOrder_values(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("a/x"), 1)
	root.Set(path.NewKeyPath("a/y"), 2)
	root.Set(path.NewKeyPath("a/z"), 3)

	parentNode := root.Get(path.NewKeyPath("a"))
	must.NotNil(t, parentNode)

	children := parentNode.Children()
	test.Eq(t, 1, children[0].Value)
	test.Eq(t, 2, children[1].Value)
	test.Eq(t, 3, children[2].Value)
}

func TestNode_ChildrenOrder_keys(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("a/x"), 1)
	root.Set(path.NewKeyPath("a/y"), 2)
	root.Set(path.NewKeyPath("a/z"), 3)

	parentNode := root.Get(path.NewKeyPath("a"))
	must.NotNil(t, parentNode)

	keys := parentNode.ChildrenKeys()
	test.Eq(t, []string{"x", "y", "z"}, keys)
}

func TestNode_Children_nil(t *testing.T) {
	t.Parallel()

	node := tree.New()
	test.Nil(t, node.Children())
}

func TestNode_Children_initializedButEmpty(t *testing.T) {
	t.Parallel()

	node := tree.New()
	node.SetChild("child", tree.New())
	node.DeleteChild("child")
	test.Nil(t, node.Children())
}

func TestNode_ChildrenKeys_nil(t *testing.T) {
	t.Parallel()

	node := tree.New()
	test.Nil(t, node.ChildrenKeys())
}

func TestNode_ChildrenKeys_initializedButEmpty(t *testing.T) {
	t.Parallel()

	node := tree.New()
	node.SetChild("child", tree.New())
	node.DeleteChild("child")
	test.Nil(t, node.ChildrenKeys())
}

func TestNode_SetChild_ReplacePreservesOrder_order(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.SetChild("a", tree.New())
	root.SetChild("b", tree.New())
	root.SetChild("c", tree.New())

	keys := root.ChildrenKeys()
	test.Eq(t, []string{"a", "b", "c"}, keys)

	newNode := tree.New()

	newNode.Value = "replaced"
	root.SetChild("b", newNode)

	keys = root.ChildrenKeys()
	test.Eq(t, []string{"a", "b", "c"}, keys)
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
	must.NotNil(t, child)
	test.Eq(t, "replaced", child.Value)
}

func TestNode_SetChild_nilChild(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.SetChild("a", nil)

	child := root.Child("a")
	test.Nil(t, child)
}

func TestNode_SetChild_overwriteNil(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.SetChild("a", tree.New())

	child := root.Child("a")
	must.NotNil(t, child)

	root.SetChild("a", nil)

	child = root.Child("a")
	test.Nil(t, child)
}

func TestNode_DeleteChild_existing(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.SetChild("a", tree.New())
	root.SetChild("b", tree.New())
	root.SetChild("c", tree.New())

	ok := root.DeleteChild("b")
	test.True(t, ok)

	keys := root.ChildrenKeys()
	test.Eq(t, []string{"a", "c"}, keys)
	test.Nil(t, root.Child("b"))
}

func TestNode_DeleteChild_missing(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.SetChild("a", tree.New())
	root.SetChild("b", tree.New())
	root.SetChild("c", tree.New())

	ok := root.DeleteChild("missing")
	test.False(t, ok)
}

func TestNode_DeleteChild_NilChildren(t *testing.T) {
	t.Parallel()

	root := tree.New()
	// root.children is nil initially.
	ok := root.DeleteChild("any")
	test.False(t, ok)
}

func TestNode_SourceRevision(t *testing.T) {
	t.Parallel()

	root := tree.New()

	root.Source = "file"
	root.Revision = "123"

	root.Set(path.NewKeyPath("a/b"), "value")

	node := root.Get(path.NewKeyPath("a/b"))
	must.NotNil(t, node)
	test.Eq(t, "value", node.Value)
	test.Eq(t, "", node.Source)
	test.Eq(t, "", node.Revision)

	node.Source = "env"
	node.Revision = "456"
	test.Eq(t, "env", node.Source)
	test.Eq(t, "456", node.Revision)
}

func TestNode_EmptyPath(t *testing.T) {
	t.Parallel()

	root := tree.New()

	root.Value = "root value"
	root.Set(path.KeyPath{}, "new root")

	test.Eq(t, "new root", root.Value)
}

func TestNode_IntermediateNodesCreated(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("deep/nested/path"), 99)

	deep := root.Get(path.NewKeyPath("deep"))
	must.NotNil(t, deep)
	test.False(t, deep.IsLeaf())

	nested := deep.Get(path.NewKeyPath("nested"))
	must.NotNil(t, nested)

	leaf := root.Get(path.NewKeyPath("deep/nested/path"))
	must.NotNil(t, leaf)
	test.Eq(t, 99, leaf.Value)
}

func TestNode_PathEmptySegment(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(path.NewKeyPath("a//c"), 42)

	node := root.Get(path.NewKeyPath("a//c"))
	must.NotNil(t, node)
	test.Eq(t, 42, node.Value)

	emptyNode := root.Get(path.NewKeyPath("a/"))
	must.NotNil(t, emptyNode)
	test.False(t, emptyNode.IsLeaf())
}
