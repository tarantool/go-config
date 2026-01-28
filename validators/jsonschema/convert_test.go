package jsonschema //nolint:testpackage

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tarantool/go-config/path"
)

func TestJsonPointerToKeyPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		pointer  string
		expected path.KeyPath
	}{
		{
			name:     "empty pointer",
			pointer:  "",
			expected: nil,
		},
		{
			name:     "root slash",
			pointer:  "/",
			expected: nil,
		},
		{
			name:     "single segment",
			pointer:  "/name",
			expected: path.KeyPath{"name"},
		},
		{
			name:     "multiple segments",
			pointer:  "/path/to/field",
			expected: path.KeyPath{"path", "to", "field"},
		},
		{
			name:     "no leading slash (should still work)",
			pointer:  "path/to/field",
			expected: path.KeyPath{"path", "to", "field"},
		},
		{
			name:     "empty segments",
			pointer:  "//",
			expected: path.KeyPath{"", ""},
		},
		{
			name:     "escaped tilde",
			pointer:  "/~0",
			expected: path.KeyPath{"~"},
		},
		{
			name:     "escaped slash",
			pointer:  "/~1",
			expected: path.KeyPath{"/"},
		},
		{
			name:     "multiple escapes",
			pointer:  "/~0~1~0",
			expected: path.KeyPath{"~/~"},
		},
		{
			name:     "mixed escaped and normal",
			pointer:  "/path~1to~0field/normal",
			expected: path.KeyPath{"path/to~field", "normal"},
		},
		{
			name:     "trailing slash",
			pointer:  "/path/to/",
			expected: path.KeyPath{"path", "to", ""},
		},
		{
			name:     "only escapes",
			pointer:  "/~0~1",
			expected: path.KeyPath{"~/"},
		},
		{
			name:     "complex nested",
			pointer:  "/a/b/c/d/e/f",
			expected: path.KeyPath{"a", "b", "c", "d", "e", "f"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := jsonPointerToKeyPath(tt.pointer)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestJsonPointerToKeyPath_EdgeCases(t *testing.T) {
	t.Parallel()

	// Test that the function does not panic on weird inputs.
	tests := []struct {
		name    string
		pointer string
	}{
		{"many slashes", "////////"},
		{"mixed slashes and escapes", "/~0/~1/~0~1"},
		{"only tildes", "~~~"},
		{"tilde not followed by 0 or 1", "/~2"},
		{"unicode", "/café/☕"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Should not panic.
			_ = jsonPointerToKeyPath(tt.pointer)
		})
	}
}

func TestJsonPointerToKeyPath_StringRepresentation(t *testing.T) {
	t.Parallel()

	// Ensure the resulting KeyPath can be converted back to a string.
	tests := []struct {
		pointer string
	}{
		{"/simple"},
		{"/path/to/field"},
		{"/~0home~1user"},
		{"/a/b/c"},
	}

	for _, tt := range tests {
		t.Run(tt.pointer, func(t *testing.T) {
			t.Parallel()

			kp := jsonPointerToKeyPath(tt.pointer)
			// Just ensure it's a valid KeyPath (no crash).
			require.NotPanics(t, func() {
				_ = kp.String()
			})
		})
	}
}
