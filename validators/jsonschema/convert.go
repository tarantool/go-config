package jsonschema

import (
	"strings"

	"github.com/tarantool/go-config/keypath"
)

// jsonPointerToKeyPath converts "/path/to/field" to KeyPath{"path", "to", "field"}.
func jsonPointerToKeyPath(pointer string) keypath.KeyPath {
	if pointer == "" || pointer == "/" {
		return nil
	}

	parts := strings.Split(strings.TrimPrefix(pointer, "/"), "/")

	// Handle JSON pointer escaping (~0 = ~, ~1 = /).
	for i, p := range parts {
		parts[i] = strings.ReplaceAll(strings.ReplaceAll(p, "~1", "/"), "~0", "~")
	}

	return parts
}
