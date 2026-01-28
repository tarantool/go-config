package jsonschema

import (
	"strings"

	"github.com/tarantool/go-config/path"
)

// jsonPointerToKeyPath converts "/path/to/field" to KeyPath{"path", "to", "field"}.
func jsonPointerToKeyPath(pointer string) path.KeyPath {
	if pointer == "" || pointer == "/" {
		return nil
	}
	// Remove leading slash and split.
	trimmed := strings.TrimPrefix(pointer, "/")
	parts := strings.Split(trimmed, "/")

	// Handle JSON pointer escaping (~0 = ~, ~1 = /).
	for i, p := range parts {
		parts[i] = strings.ReplaceAll(strings.ReplaceAll(p, "~1", "/"), "~0", "~")
	}

	return path.KeyPath(parts)
}
