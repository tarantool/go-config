package jsonschema //nolint:testpackage

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tarantool/go-config/keypath"
)

func TestJsonPointerToKeyPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		pointer  string
		expected keypath.KeyPath
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
			expected: keypath.KeyPath{"name"},
		},
		{
			name:     "multiple segments",
			pointer:  "/path/to/field",
			expected: keypath.KeyPath{"path", "to", "field"},
		},
		{
			name:     "no leading slash (should still work)",
			pointer:  "path/to/field",
			expected: keypath.KeyPath{"path", "to", "field"},
		},
		{
			name:     "empty segments",
			pointer:  "//",
			expected: keypath.KeyPath{"", ""},
		},
		{
			name:     "escaped tilde",
			pointer:  "/~0",
			expected: keypath.KeyPath{"~"},
		},
		{
			name:     "escaped slash",
			pointer:  "/~1",
			expected: keypath.KeyPath{"/"},
		},
		{
			name:     "multiple escapes",
			pointer:  "/~0~1~0",
			expected: keypath.KeyPath{"~/~"},
		},
		{
			name:     "mixed escaped and normal",
			pointer:  "/path~1to~0field/normal",
			expected: keypath.KeyPath{"path/to~field", "normal"},
		},
		{
			name:     "trailing slash",
			pointer:  "/path/to/",
			expected: keypath.KeyPath{"path", "to", ""},
		},
		{
			name:     "only escapes",
			pointer:  "/~0~1",
			expected: keypath.KeyPath{"~/"},
		},
		{
			name:     "complex nested",
			pointer:  "/a/b/c/d/e/f",
			expected: keypath.KeyPath{"a", "b", "c", "d", "e", "f"},
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
