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
