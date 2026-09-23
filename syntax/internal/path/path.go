// Package path describes locations in a document using property names and array indices.
package path

// Step identifies an object property or an array index.
// IsIndex distinguishes numeric and empty property names from array indices.
// The zero value addresses the empty property name.
type Step struct {
	Property string
	Index    int
	IsIndex  bool
}

// Property addresses an object property by its name, including an empty name.
func Property(name string) Step { return Step{Property: name, Index: 0, IsIndex: false} }

// Index addresses an array item by its zero-based index.
func Index(index int) Step { return Step{Property: "", Index: index, IsIndex: true} }
