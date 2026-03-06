package tree_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tarantool/go-config/keypath"
	"github.com/tarantool/go-config/tree"
)

func TestToAny_Leaf(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value any
	}{
		{"integer", 42},
		{"string", "hello"},
		{"boolean", true},
		{"nil", nil},
		{"float", 3.14},
		{"slice", []any{1, 2, 3}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			node := tree.New()

			node.Value = tt.value

			result := tree.ToAny(node)

			expected := tt.value
			if tt.value == nil {
				// Leaf with nil value converts to empty map (special case).
				expected = map[string]any{}
			}

			assert.Equal(t, expected, result)
		})
	}
}

func TestToAny_Nested(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("server/port"), 8080)
	root.Set(keypath.NewKeyPath("server/host"), "localhost")
	root.Set(keypath.NewKeyPath("database/name"), "testdb")

	result := tree.ToAny(root)
	expected := map[string]any{
		"server": map[string]any{
			"port": 8080,
			"host": "localhost",
		},
		"database": map[string]any{
			"name": "testdb",
		},
	}
	assert.Equal(t, any(expected), result)
}

func TestToAny_EmptyNode(t *testing.T) {
	t.Parallel()

	node := tree.New()
	result := tree.ToAny(node)
	expected := map[string]any{}
	assert.Equal(t, any(expected), result)
}

func TestToAny_MissingChild(t *testing.T) {
	t.Parallel()

	// Create a node with a child key but child is nil (should not happen in practice).
	// The function should skip nil children.
	node := tree.New()
	// Manually set children map? Not possible via public API.
	// We'll just test that ToAny handles nil child gracefully.
	// Since node.Child returns nil for non-existent key, ToAny iterates over ChildrenKeys()
	// which returns only keys where child exists. So nil child won't appear.
	// We'll just ensure no panic.
	node.Set(keypath.NewKeyPath("existing"), "value")
	// No child for "missing".
	result := tree.ToAny(node)
	expected := map[string]any{
		"existing": "value",
	}
	assert.Equal(t, any(expected), result)
}

func TestToAny_DeepNesting(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.Set(keypath.NewKeyPath("a/b/c/d"), "deep")
	root.Set(keypath.NewKeyPath("a/b/x"), 123)
	root.Set(keypath.NewKeyPath("a/y"), true)

	result := tree.ToAny(root)
	expected := map[string]any{
		"a": map[string]any{
			"b": map[string]any{
				"c": map[string]any{
					"d": "deep",
				},
				"x": 123,
			},
			"y": true,
		},
	}
	assert.Equal(t, any(expected), result)
}

func TestToAny_Array(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.MarkArray()
	root.SetChild("0", tree.New())
	root.SetChild("1", tree.New())
	root.SetChild("2", tree.New())

	root.Child("0").Value = "a"
	root.Child("1").Value = "b"
	root.Child("2").Value = "c"

	result := tree.ToAny(root)
	expected := []any{"a", "b", "c"}
	assert.Equal(t, any(expected), result)
}

func TestToAny_ArrayOfMaps(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.MarkArray()

	child0 := tree.New()
	child0.Set(keypath.NewKeyPath("name"), "Alice")
	child0.Set(keypath.NewKeyPath("age"), 30)
	root.SetChild("0", child0)

	child1 := tree.New()
	child1.Set(keypath.NewKeyPath("name"), "Bob")
	child1.Set(keypath.NewKeyPath("age"), 25)
	root.SetChild("1", child1)

	result := tree.ToAny(root)
	expected := []any{
		map[string]any{"name": "Alice", "age": 30},
		map[string]any{"name": "Bob", "age": 25},
	}
	assert.Equal(t, any(expected), result)
}

func TestToArray_EmptyArray(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.MarkArray()

	result := tree.ToAny(root)
	expected := []any{}
	assert.Equal(t, any(expected), result)
}

func TestToAny_ArrayWithNil(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.MarkArray()
	root.SetChild("0", tree.New())
	// Skip index 1 (nil).
	root.SetChild("2", tree.New())

	root.Child("0").Value = "a"
	root.Child("2").Value = "c"

	result := tree.ToAny(root)
	// Only non-nil children are included.
	expected := []any{"a", "c"}
	assert.Equal(t, any(expected), result)
}

func TestToAny_NestedArray(t *testing.T) {
	t.Parallel()

	root := tree.New()
	root.MarkArray()

	outer0 := tree.New()
	outer0.MarkArray()
	outer0.SetChild("0", tree.New())
	outer0.SetChild("1", tree.New())

	outer0.Child("0").Value = 1
	outer0.Child("1").Value = 2

	outer1 := tree.New()
	outer1.MarkArray()
	outer1.SetChild("0", tree.New())
	outer1.SetChild("1", tree.New())

	outer1.Child("0").Value = 3
	outer1.Child("1").Value = 4

	root.SetChild("0", outer0)
	root.SetChild("1", outer1)

	result := tree.ToAny(root)
	expected := []any{
		[]any{1, 2},
		[]any{3, 4},
	}
	assert.Equal(t, any(expected), result)
}
