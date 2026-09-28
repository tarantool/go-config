package tree_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tarantool/go-config/v2/keypath"
	"github.com/tarantool/go-config/v2/tree"
)

// fixedLeaf builds a tree holding value at path, with the leaf's type fixed
// as fixed says, and returns the tree and the leaf.
func fixedLeaf(path string, value any, fixed bool) (*tree.Node, *tree.Node) {
	root := tree.New()
	root.Set(keypath.NewKeyPath(path), value)

	node := root.Get(keypath.NewKeyPath(path))
	node.SetTypeFixed(fixed)

	return root, node
}

func TestNode_TypeFixed(t *testing.T) {
	t.Parallel()

	node := tree.New()
	assert.False(t, node.TypeFixed(), "a new node has no fixed type")

	node.SetTypeFixed(true)
	assert.True(t, node.TypeFixed())

	node.SetTypeFixed(false)
	assert.False(t, node.TypeFixed())
}

func TestNode_Set_ClearsTypeFixed(t *testing.T) {
	t.Parallel()

	root, node := fixedLeaf("a/b", "true", true)
	require.True(t, node.TypeFixed())

	root.Set(keypath.NewKeyPath("a/b"), "true")
	assert.False(t, node.TypeFixed(), "a value set anew says nothing about its source")
}

func TestValue_TypeFixed(t *testing.T) {
	t.Parallel()

	for _, fixed := range []bool{true, false} {
		_, node := fixedLeaf("k", "v", fixed)

		carrier, ok := tree.NewValue(node, keypath.NewKeyPath("k")).(interface{ TypeFixed() bool })
		require.True(t, ok, "the merge pipeline discovers the flag through this method")
		assert.Equal(t, fixed, carrier.TypeFixed())
	}
}

func TestValue_Get_FixedStringIntoScalar(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   string
		dest    any
		convert error
	}{
		{"bool", "true", new(bool), tree.ErrConvertToBool},
		{"int", "3301", new(int), tree.ErrConvertToInt},
		{"int8", "1", new(int8), tree.ErrConvertToInt},
		{"uint", "3301", new(uint), tree.ErrConvertToUint},
		{"uint16", "1", new(uint16), tree.ErrConvertToUint},
		{"float64", "0.5", new(float64), tree.ErrConvertToFloat},
		{"float32", "0.5", new(float32), tree.ErrConvertToFloat},
		{"pointer to bool", "false", new(*bool), tree.ErrConvertToBool},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, node := fixedLeaf("k", tt.value, true)

			err := tree.NewValue(node, keypath.NewKeyPath("k")).Get(tt.dest)
			require.ErrorIs(t, err, tree.ErrFixedTypeString)
			require.ErrorIs(t, err, tt.convert)
			assert.Contains(t, err.Error(), `"`+tt.value+`"`)
		})
	}
}

func TestValue_Get_UntypedStringIntoScalar(t *testing.T) {
	t.Parallel()

	_, node := fixedLeaf("k", "true", false)

	var got bool

	require.NoError(t, tree.NewValue(node, keypath.NewKeyPath("k")).Get(&got))
	assert.True(t, got, "a string without a fixed type is still parsed")
}

func TestValue_Get_FixedStringIntoString(t *testing.T) {
	t.Parallel()

	_, node := fixedLeaf("k", "false", true)
	val := tree.NewValue(node, keypath.NewKeyPath("k"))

	var str string

	require.NoError(t, val.Get(&str))
	assert.Equal(t, "false", str)

	var anything any

	require.NoError(t, val.Get(&anything))
	assert.Equal(t, "false", anything)
}

func TestValue_Get_FixedStringIntoDuration(t *testing.T) {
	t.Parallel()

	_, node := fixedLeaf("k", "5s", true)

	var got time.Duration

	require.NoError(t, tree.NewValue(node, keypath.NewKeyPath("k")).Get(&got),
		"a string is how a duration is written, quoted or not")
	assert.Equal(t, 5*time.Second, got)
}

func TestValue_Get_FixedStringInStruct(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("svc/enabled"), "true")
	root.Set(keypath.NewKeyPath("svc/port"), "3301")
	root.Get(keypath.NewKeyPath("svc/enabled")).SetTypeFixed(true)

	val := tree.NewValue(root.Get(keypath.NewKeyPath("svc")), keypath.NewKeyPath("svc"))

	var strict struct {
		Enabled bool `yaml:"enabled"`
		Port    int  `yaml:"port"`
	}

	err := val.Get(&strict)
	require.ErrorIs(t, err, tree.ErrFixedTypeString)
	assert.Contains(t, err.Error(), `field "Enabled"`)

	var loose struct {
		Enabled string `yaml:"enabled"`
		Port    int    `yaml:"port"`
	}

	require.NoError(t, val.Get(&loose))
	assert.Equal(t, "true", loose.Enabled)
	assert.Equal(t, 3301, loose.Port, "the untyped sibling is still parsed")
}

func TestValue_Get_FixedStringInInlineStruct(t *testing.T) {
	t.Parallel()

	type Inner struct {
		Enabled bool `yaml:"enabled"`
	}

	root := tree.New()
	root.Set(keypath.NewKeyPath("svc/enabled"), "true")
	root.Get(keypath.NewKeyPath("svc/enabled")).SetTypeFixed(true)

	var dest struct {
		Inner `yaml:",inline"`
	}

	err := tree.NewValue(root.Get(keypath.NewKeyPath("svc")), keypath.NewKeyPath("svc")).Get(&dest)
	require.ErrorIs(t, err, tree.ErrFixedTypeString)
}

func TestValue_Get_FixedStringInMap(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("m/a"), "true")
	root.Set(keypath.NewKeyPath("m/b"), "false")
	root.Get(keypath.NewKeyPath("m/b")).SetTypeFixed(true)

	val := tree.NewValue(root.Get(keypath.NewKeyPath("m")), keypath.NewKeyPath("m"))

	var bools map[string]bool

	err := val.Get(&bools)
	require.ErrorIs(t, err, tree.ErrFixedTypeString)
	assert.Contains(t, err.Error(), `map key "b"`)

	var strs map[string]string

	require.NoError(t, val.Get(&strs))
	assert.Equal(t, map[string]string{"a": "true", "b": "false"}, strs)
}

func TestValue_Get_FixedStringInArray(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("arr/0"), "1")
	root.Set(keypath.NewKeyPath("arr/1"), "2")
	root.Get(keypath.NewKeyPath("arr")).MarkArray()

	val := tree.NewValue(root.Get(keypath.NewKeyPath("arr")), keypath.NewKeyPath("arr"))

	var ints []int

	require.NoError(t, val.Get(&ints))
	assert.Equal(t, []int{1, 2}, ints)

	root.Get(keypath.NewKeyPath("arr/1")).SetTypeFixed(true)

	err := val.Get(&ints)
	require.ErrorIs(t, err, tree.ErrFixedTypeString)
	assert.Contains(t, err.Error(), "slice element [1]")
}
