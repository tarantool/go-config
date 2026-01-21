package config

import "github.com/tarantool/go-config/path"

// KeyPath represents a hierarchical key.
//
// For example, for the key "/server/http/port" it will look like
// []string{"server", "http", "port"}.
type KeyPath = path.KeyPath

// NewKeyPath creates a KeyPath from a string, splitting it by "/" slash.
// This is the main way to create a path from a textual representation.
func NewKeyPath(p string) KeyPath {
	return path.NewKeyPath(p)
}

// NewKeyPathWithDelim creates a KeyPath from a string, splitting it by the given delimiter.
// All segments are preserved, including empty ones.
func NewKeyPathWithDelim(p, delim string) KeyPath {
	return path.NewKeyPathWithDelim(p, delim)
}
