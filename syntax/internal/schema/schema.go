// Package schema compiles and navigates JSON Schemas for editor operations.
package schema

import (
	"errors"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"sync"

	"github.com/kaptinlin/jsonschema"
	"github.com/tarantool/go-config/v2/internal/schemautil"
	"github.com/tarantool/go-config/v2/syntax/internal/path"
)

// JSON Schema types used by editor operations.
const (
	TypeArray  = "array"
	TypeObject = "object"
	TypeString = "string"
)

// Schema owns a compiled schema and its validation caches.
// The validator lazily updates regex caches, so candidate validation is
// serialized when callers share a schema concurrently.
type Schema struct {
	root *jsonschema.Schema
	mu   sync.Mutex
}

// ErrUnresolvedReference is returned when a schema reference cannot be resolved.
var ErrUnresolvedReference = errors.New("schema: unresolved JSON Schema references")

// Compile parses JSON Schema, resolves its references, and prepares shared
// validation state. The returned schema is independent of the source bytes.
func Compile(source []byte) (*Schema, error) {
	compiled, err := jsonschema.NewCompiler().Compile(source)
	if err != nil {
		return nil, fmt.Errorf("schema: compile JSON Schema: %w", err)
	}

	if unresolved := compiled.UnresolvedReferenceURIs(); len(unresolved) > 0 {
		return nil, fmt.Errorf("%w: %v", ErrUnresolvedReference, unresolved)
	}

	return &Schema{root: compiled, mu: sync.Mutex{}}, nil
}

// Lookup returns candidate schemas at a typed property/index path.
// It follows references and composition branches, preserving overlapping
// constraints; AcceptsAt performs the final path and value checks. Returned
// schemas are read-only and must not be passed directly to jsonschema.Schema.Validate.
func (s *Schema) Lookup(steps []path.Step) []*jsonschema.Schema {
	if s == nil || s.root == nil {
		return nil
	}

	current := []*jsonschema.Schema{s.root}

	for _, step := range steps {
		var next []*jsonschema.Schema

		seen := make(map[*jsonschema.Schema]bool)

		for _, schema := range current {
			for _, variant := range schemautil.Variants(schema) {
				for _, child := range children(variant, step) {
					if child != nil && !seen[child] && (child.Boolean == nil || *child.Boolean) {
						seen[child] = true
						next = append(next, child)
					}
				}
			}
		}

		current = next
	}

	return current
}

// children selects one path step without resolving composition branches.
// Properties and all matching patterns apply together; additionalProperties
// is used only when neither matches. Array steps prefer prefixItems over items.
func children(schema *jsonschema.Schema, step path.Step) []*jsonschema.Schema {
	if step.IsIndex {
		if step.Index < 0 {
			return nil
		}

		if step.Index < len(schema.PrefixItems) {
			return []*jsonschema.Schema{schema.PrefixItems[step.Index]}
		}

		if schema.Items != nil {
			return []*jsonschema.Schema{schema.Items}
		}

		return nil
	}

	var children []*jsonschema.Schema

	if schema.Properties != nil {
		if child, ok := (*schema.Properties)[step.Property]; ok {
			children = append(children, child)
		}
	}

	if schema.PatternProperties != nil {
		for _, pattern := range slices.Sorted(maps.Keys(*schema.PatternProperties)) {
			matches, err := regexp.MatchString(pattern, step.Property)
			if err == nil && matches {
				children = append(children, (*schema.PatternProperties)[pattern])
			}
		}
	}

	if len(children) == 0 && schema.AdditionalProperties != nil {
		children = append(children, schema.AdditionalProperties)
	}

	return children
}

// AcceptsAt checks whether the schema permits a path and, when validateValue is
// true, its candidate value. References and allOf intersect constraints; anyOf
// and oneOf provide alternatives along the path. Leaf validation enforces value
// constraints, including oneOf exclusivity, without validating a whole configuration.
func (s *Schema) AcceptsAt(steps []path.Step, value any, validateValue bool) bool {
	if s == nil {
		return true
	}

	return s.accepts(s.root, steps, value, validateValue)
}

// HasObjectSchema reports whether any candidate at path describes an object,
// including schemas that imply an object through properties without a type.
func (s *Schema) HasObjectSchema(steps []path.Step) bool {
	for _, schema := range s.Lookup(steps) {
		for _, variant := range schemautil.Variants(schema) {
			if slices.Contains(variant.Type, TypeObject) || len(variant.Type) == 0 &&
				(variant.Properties != nil || variant.PatternProperties != nil || variant.AdditionalProperties != nil) {
				return true
			}
		}
	}

	return false
}

// Type returns a type only when all explicit candidate types agree.
// Missing or conflicting types produce an empty string.
func Type(schemas []*jsonschema.Schema) string {
	var itemType string

	for _, schema := range schemas {
		for _, variant := range schemautil.Variants(schema) {
			for _, candidate := range variant.Type {
				if itemType != "" && candidate != itemType {
					return ""
				}

				itemType = candidate
			}
		}
	}

	return itemType
}

// MetadataAt returns annotations at path, following references and composition,
// including branches above path. It does not select alternatives by the current
// configuration value. An undocumented or forbidden path returns nil.
func (s *Schema) MetadataAt(steps []path.Step) *Metadata {
	if s == nil || s.root == nil {
		return nil
	}

	// A recursive reference may consume path steps; only a cycle at the same
	// depth must stop. Shared references in separate alternatives remain visible.
	type visit struct {
		schema *jsonschema.Schema
		depth  int
	}

	active := make(map[visit]bool)

	var describe func(*jsonschema.Schema, int) *Metadata

	describe = func(current *jsonschema.Schema, depth int) *Metadata {
		if current == nil {
			return nil
		}

		key := visit{current, depth}
		if active[key] {
			// Stop expanding a cycle without excluding the alternative.
			return new(Metadata)
		}

		if !s.accepts(current, steps[depth:], nil, false) {
			return nil
		}

		active[key] = true

		defer delete(active, key)

		if schemautil.RefOverridesSiblings(current) {
			return describe(current.ResolvedRef, depth)
		}

		var metadata Metadata

		if depth == len(steps) {
			metadata = annotations(current)
		} else {
			for _, child := range children(current, steps[depth]) {
				if part := describe(child, depth+1); part != nil && !part.empty() {
					metadata.AllOf = append(metadata.AllOf, *part)
				}
			}
		}

		for _, reference := range []*jsonschema.Schema{current.ResolvedRef, current.ResolvedDynamicRef} {
			if part := describe(reference, depth); part != nil && !part.empty() {
				metadata.AllOf = append(metadata.AllOf, *part)
			}
		}

		for _, group := range []struct {
			branches  []*jsonschema.Schema
			target    *[]Metadata
			keepEmpty bool
		}{
			{current.AllOf, &metadata.AllOf, false},
			{current.AnyOf, &metadata.AnyOf, true},
			{current.OneOf, &metadata.OneOf, true},
		} {
			for _, branch := range group.branches {
				if part := describe(branch, depth); part != nil && (group.keepEmpty || !part.empty()) {
					*group.target = append(*group.target, *part)
				}
			}
		}

		// A lone reference or path step adds no information of its own.
		// Keep explicit composition groups, including groups with one branch.
		if !metadata.hasAnnotations() && len(metadata.AllOf) == 1 &&
			len(metadata.AnyOf) == 0 && len(metadata.OneOf) == 0 && len(current.AllOf) == 0 {
			return &metadata.AllOf[0]
		}

		return &metadata
	}

	metadata := describe(s.root, 0)
	if metadata == nil || metadata.empty() {
		return nil
	}

	return metadata
}

// accepts checks a path relative to any compiled branch using the same rules
// as AcceptsAt. The root must belong to this compiled schema so validation
// uses the mutex protecting its lazy caches.
func (s *Schema) accepts(root *jsonschema.Schema, steps []path.Step, value any, validateValue bool) bool {
	// Schema validation and property-name checks share the same lazy caches.
	s.mu.Lock()
	defer s.mu.Unlock()

	// Depth allows recursive references to consume path steps while stopping
	// cycles that revisit the same schema at the same location.
	type visit struct {
		schema *jsonschema.Schema
		depth  int
	}

	active := make(map[visit]bool)

	var accepts func(*jsonschema.Schema, int) bool

	accepts = func(schema *jsonschema.Schema, depth int) bool {
		if schema == nil {
			return true
		}

		if schema.Boolean != nil {
			return *schema.Boolean
		}

		key := visit{schema, depth}
		if active[key] {
			return true
		}

		active[key] = true

		defer delete(active, key)

		if depth == len(steps) && validateValue {
			return schema.Validate(value).IsValid()
		}

		if schemautil.RefOverridesSiblings(schema) {
			return accepts(schema.ResolvedRef, depth)
		}

		if !accepts(schema.ResolvedRef, depth) || !accepts(schema.ResolvedDynamicRef, depth) {
			return false
		}

		for _, branch := range schema.AllOf {
			if !accepts(branch, depth) {
				return false
			}
		}

		for _, alternatives := range [][]*jsonschema.Schema{schema.AnyOf, schema.OneOf} {
			if len(alternatives) > 0 && !slices.ContainsFunc(alternatives, func(branch *jsonschema.Schema) bool {
				return accepts(branch, depth)
			}) {
				return false
			}
		}

		if depth == len(steps) {
			return true
		}

		step := steps[depth]
		itemType := TypeObject

		if step.IsIndex {
			itemType = TypeArray
		}

		if len(schema.Type) > 0 && !slices.Contains(schema.Type, itemType) {
			return false
		}

		if !step.IsIndex && schema.PropertyNames != nil && !schema.PropertyNames.Validate(step.Property).IsValid() {
			return false
		}

		for _, child := range children(schema, step) {
			if !accepts(child, depth+1) {
				return false
			}
		}

		return true
	}

	return accepts(root, 0)
}
