// Package environ provides utilities for parsing environment variables.
package environ

import (
	"iter"
	"os"
	"strings"
)

// Parse returns an iterator over environment variables.
// It yields each environment variable as a key-value pair.
// Malformed entries (without an '=' delimiter or with empty key) are skipped.
func Parse(env []string) iter.Seq2[string, string] {
	return func(yield func(string, string) bool) {
		for _, entry := range env {
			key, value, ok := strings.Cut(entry, "=")
			if !ok || key == "" {
				// Malformed entry, skip.
				continue
			}

			if !yield(key, value) {
				return
			}
		}
	}
}

// ParseAll returns an iterator over all environment variables
// obtained from os.Environ().
func ParseAll() iter.Seq2[string, string] {
	return Parse(os.Environ())
}
