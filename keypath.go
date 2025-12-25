package config

import (
	"slices"
	"strings"
)

// KeyPath represents a hierarchical key.
//
// For example, for the key "/server/http/port" it will look like
// []string{"server", "http", "port"}.
type KeyPath []string

// NewKeyPath creates a KeyPath from a string, splitting it by "/" slash.
// This is the main way to create a path from a textual representation.
func NewKeyPath(path string) KeyPath {
	return NewKeyPathWithDelim(path, "/")
}

// NewKeyPathWithDelim creates a KeyPath from a string, splitting it by the given delimiter.
// All segments are preserved, including empty ones.
func NewKeyPathWithDelim(path, delim string) KeyPath {
	if path == "" {
		return KeyPath{}
	}

	return strings.Split(path, delim)
}

// String returns a textual representation of the path with slash ("/") as the delimiter.
func (p KeyPath) String() string {
	return p.MakeString("/")
}

// MakeString returns a textual representation of the path with the given delimiter.
func (p KeyPath) MakeString(delim string) string {
	return strings.Join(p, delim)
}

// Parent returns the parent path.
// If the path consists of one element or is empty, returns nil.
func (p KeyPath) Parent() KeyPath {
	if len(p) <= 1 {
		return nil
	}

	return p[:len(p)-1]
}

// Leaf returns the last segment of the path.
// If the path is empty, returns an empty string.
func (p KeyPath) Leaf() string {
	if len(p) == 0 {
		return ""
	}

	return p[len(p)-1]
}

// Append adds new segment(s) to the path, returning a new KeyPath.
// The original path is not changed (immutability).
func (p KeyPath) Append(segments ...string) KeyPath {
	if len(segments) == 0 {
		return p
	}

	newPath := make(KeyPath, 0, len(p)+len(segments))

	newPath = append(append(newPath, p...), segments...)

	return newPath
}

// Equals checks that two paths are completely identical.
func (p KeyPath) Equals(other KeyPath) bool {
	if len(p) != len(other) {
		return false
	}

	for i := range p {
		if p[i] != other[i] {
			return false
		}
	}

	return true
}

// Match checks whether the path matches a pattern using wildcards.
// A wildcard is "*", which matches any single segment.
// A double wildcard "**" matches zero or more segments.
// Pattern matches if it is a prefix of the path (pattern length <= path length).
// For example, pattern "a/*/c" matches path "a/b/c".
// Pattern "a/*/c" also matches path "a/b/c/d" (since pattern is prefix).
// Pattern "a/**/c" matches path "a/b/c", "a/x/y/c", and "a/c".
func (p KeyPath) Match(pattern KeyPath) bool {
	var (
		pointerI          = 0
		pointerJ          = 0
		backtrackPointerI = -1
		backtrackPointerJ = -1
	)

	for pointerI < len(p) && pointerJ < len(pattern) {
		seg := pattern[pointerJ]
		if seg == "*" {
			pointerI++

			pointerJ++

			continue
		}

		if seg == "**" {
			backtrackPointerI = pointerI
			backtrackPointerJ = pointerJ
			pointerJ++

			continue
		}

		if seg == p[pointerI] {
			pointerI++

			pointerJ++

			continue
		}

		if backtrackPointerJ >= 0 {
			pointerI = backtrackPointerI + 1
			pointerJ = backtrackPointerJ
			backtrackPointerI = pointerI

			continue
		}

		return false
	}

	for pointerJ < len(pattern) && pattern[pointerJ] == "**" {
		pointerJ++
	}

	return pointerJ == len(pattern)
}

// HasEmptySegment returns true if the path contains any empty segment ("").
func (p KeyPath) HasEmptySegment() bool {
	return slices.Contains(p, "")
}
