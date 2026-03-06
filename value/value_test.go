package value_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tarantool/go-config/keypath"
	"github.com/tarantool/go-config/meta"
	"github.com/tarantool/go-config/tree"
)

type mockValue struct {
	meta meta.Info
}

func (m *mockValue) Get(_ any) error {
	return nil
}

func (m *mockValue) Meta() meta.Info {
	return m.meta
}

func TestValue_Interface(t *testing.T) {
	t.Parallel()

	var (
		_ = (*mockValue)(nil)
		_ = tree.NewValue(tree.New(), keypath.NewKeyPath("key"))
	)
}

func TestValue_Meta_ZeroValue(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("key"), "value")

	node := root.Get(keypath.NewKeyPath("key"))
	assert.NotNil(t, node)

	val := tree.NewValue(node, keypath.NewKeyPath("key"))
	m := val.Meta()

	assert.True(t, m.Key.Equals(keypath.NewKeyPath("key")))
	assert.Equal(t, meta.SourceInfo{Name: "", Type: meta.UnknownSource}, m.Source)
	assert.Equal(t, meta.RevisionType(""), m.Revision)
}

func TestValue_Meta_WithSource(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("key"), "value")

	node := root.Get(keypath.NewKeyPath("key"))
	assert.NotNil(t, node)

	node.Source = "myfile.yaml"
	node.Revision = "v2"

	val := tree.NewValue(node, keypath.NewKeyPath("key"))
	m := val.Meta()

	assert.True(t, m.Key.Equals(keypath.NewKeyPath("key")))
	assert.Equal(t, "myfile.yaml", m.Source.Name)
	assert.Equal(t, meta.RevisionType("v2"), m.Revision)
}

func TestValue_Meta_WithKeyPath(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("a/b/c"), 42)

	node := root.Get(keypath.NewKeyPath("a/b/c"))
	assert.NotNil(t, node)

	val := tree.NewValue(node, keypath.NewKeyPath("a/b/c"))
	m := val.Meta()

	assert.True(t, m.Key.Equals(keypath.NewKeyPath("a/b/c")))
}
