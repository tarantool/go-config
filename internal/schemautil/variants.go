// Package schemautil shares JSON Schema traversal between validation and syntax.
package schemautil

import "github.com/kaptinlin/jsonschema"

// Branches returns allOf, anyOf, and oneOf branches without resolving references.
func Branches(schema *jsonschema.Schema) []*jsonschema.Schema {
	branches := make([]*jsonschema.Schema, 0, len(schema.AllOf)+len(schema.AnyOf)+len(schema.OneOf))

	branches = append(branches, schema.AllOf...)
	branches = append(branches, schema.AnyOf...)

	return append(branches, schema.OneOf...)
}

// RefOverridesSiblings reports the reference-object semantics of older drafts.
// Since Draft 2019-09, siblings of $ref apply alongside the referenced schema.
func RefOverridesSiblings(schema *jsonschema.Schema) bool {
	if schema.ResolvedRef == nil {
		return false
	}

	dialect := schema.Dialect()

	return dialect == jsonschema.Draft4 || dialect == jsonschema.Draft6 || dialect == jsonschema.Draft7
}

// Variants visits references and composition branches once, preserving sibling keywords.
func Variants(schema *jsonschema.Schema) []*jsonschema.Schema {
	var variants []*jsonschema.Schema

	seen := make(map[*jsonschema.Schema]bool)

	var visit func(*jsonschema.Schema)

	visit = func(current *jsonschema.Schema) {
		if current == nil || seen[current] {
			return
		}

		seen[current] = true
		if RefOverridesSiblings(current) {
			visit(current.ResolvedRef)
			return
		}

		variants = append(variants, current)
		visit(current.ResolvedRef)
		visit(current.ResolvedDynamicRef)

		for _, branch := range Branches(current) {
			visit(branch)
		}
	}
	visit(schema)

	return variants
}
