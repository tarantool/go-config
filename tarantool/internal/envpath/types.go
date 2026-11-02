package envpath

import (
	"slices"
)

// Kind is the shape the schema gives a value, in the terms of Tarantool's
// config schema (src/box/lua/config/utils/schema.lua), which the JSON schema
// is generated from: a scalar, an array, a map with free keys, a record with
// fixed fields, or a union of several variants.
type Kind int

const (
	// KindAny is a value the schema puts no type on, like Tarantool's "any"
	// scalar (an empty JSON schema: app.cfg.*, roles_cfg.*).
	KindAny Kind = iota
	// KindString is a "string" scalar.
	KindString
	// KindNumber is a "number" scalar.
	KindNumber
	// KindInteger is an "integer" scalar.
	KindInteger
	// KindBoolean is a "boolean" scalar.
	KindBoolean
	// KindNull is "null" in a JSON schema type list. Tarantool's schema has
	// no such type, and no environment variable is read as one.
	KindNull
	// KindArray is an array; [Type.Items] is the type of its items.
	KindArray
	// KindMap is an object whose keys are free (additionalProperties,
	// patternProperties); [Type.Values] is the type of its values.
	KindMap
	// KindRecord is an object with fixed properties.
	KindRecord
	// KindUnion is a value of one of several types: a JSON schema type list
	// or anyOf / oneOf; [Type.Variants] are the types, in schema order.
	KindUnion
)

// Type describes the value the schema expects at a path, as much of it as
// reading the value from an environment variable needs: the fields of a
// record are not described, since a record never comes from one variable.
type Type struct {
	// Kind is the shape of the value.
	Kind Kind
	// Items is the type of the items of an array.
	Items *Type
	// Values is the type of the values of a map.
	Values *Type
	// Variants are the types a union value may have, in schema order.
	Variants []*Type
	// Enum lists the values a scalar may take, as the JSON schema "enum"
	// keyword of the scalar's own schema gives them. Empty means any.
	Enum []any
}

// newType returns a Type of the given kind with nothing else set.
func newType(kind Kind) *Type {
	return &Type{
		Kind:     kind,
		Items:    nil,
		Values:   nil,
		Variants: nil,
		Enum:     nil,
	}
}

// unionOf returns the union of variants. A union of one variant is that
// variant, and a union of none puts no type on the value.
func unionOf(variants []*Type) *Type {
	switch len(variants) {
	case 0:
		return newType(KindAny)
	case 1:
		return variants[0]
	default:
		return &Type{Kind: KindUnion, Items: nil, Values: nil, Variants: variants, Enum: nil}
	}
}

// typeOf derives the Type of the value a schema node describes. defs resolves
// local $refs; visited holds the definitions expanded on the way here, so a
// schema that contains itself yields KindAny where the cycle closes instead
// of recursing forever.
//
// The node's "type" decides when it is present, a list of types being a
// union of them. Without it, anyOf / oneOf make a union of their branches,
// and the presence of properties, additionalProperties, patternProperties or
// items says the node is an object or an array. Anything else, allOf
// included, puts no type on the value.
func typeOf(node map[string]any, defs map[string]any, visited map[string]bool) *Type {
	node, marked, ok := followRef(node, defs, visited)
	defer unmark(visited, marked)

	if !ok {
		return newType(KindAny)
	}

	switch declared := node["type"].(type) {
	case string:
		return namedType(declared, node, defs, visited, true)
	case []any:
		variants := make([]*Type, 0, len(declared))

		for _, raw := range declared {
			name, isString := raw.(string)
			if !isString {
				continue
			}

			// The enum of a node with a type list constrains the union as a
			// whole, not each of its variants.
			variants = append(variants, namedType(name, node, defs, visited, false))
		}

		return unionOf(variants)
	}

	for _, keyword := range []string{"anyOf", "oneOf"} {
		branches, isList := node[keyword].([]any)
		if !isList {
			continue
		}

		variants := make([]*Type, 0, len(branches))

		for _, raw := range branches {
			branch, isObj := raw.(map[string]any)
			if !isObj {
				continue
			}

			variants = append(variants, typeOf(branch, defs, visited))
		}

		return unionOf(variants)
	}

	switch {
	case hasAny(node, "properties", "additionalProperties", "patternProperties"):
		return objectType(node, defs, visited)
	case hasAny(node, "items"):
		return arrayType(node, defs, visited)
	default:
		return newType(KindAny)
	}
}

// namedType returns the Type of a JSON schema type name declared on node.
// withEnum attaches the node's enum to a scalar.
func namedType(name string, node map[string]any, defs map[string]any, visited map[string]bool, withEnum bool) *Type {
	var typ *Type

	switch name {
	case "string":
		typ = newType(KindString)
	case "number":
		typ = newType(KindNumber)
	case "integer":
		typ = newType(KindInteger)
	case "boolean":
		typ = newType(KindBoolean)
	case "null":
		return newType(KindNull)
	case "array":
		return arrayType(node, defs, visited)
	case "object":
		return objectType(node, defs, visited)
	default:
		return newType(KindAny)
	}

	if enum, isList := node["enum"].([]any); withEnum && isList {
		typ.Enum = enum
	}

	return typ
}

// arrayType returns the Type of an array schema node. Items given by anything
// but a schema object (absent, true, prefixItems) have no type.
func arrayType(node map[string]any, defs map[string]any, visited map[string]bool) *Type {
	itemsType := newType(KindAny)

	items, isObj := node["items"].(map[string]any)
	if isObj {
		itemsType = typeOf(items, defs, visited)
	}

	return &Type{Kind: KindArray, Items: itemsType, Values: nil, Variants: nil, Enum: nil}
}

// objectType returns the Type of an object schema node: a record when it has
// fixed properties or allows no other keys, a map otherwise. Tarantool's
// schema generates a record as properties plus additionalProperties: false
// and a map as additionalProperties alone.
func objectType(node map[string]any, defs map[string]any, visited map[string]bool) *Type {
	props, _ := node["properties"].(map[string]any)
	patterns, _ := node["patternProperties"].(map[string]any)

	closed := node["additionalProperties"] == false

	if len(props) > 0 || closed && len(patterns) == 0 {
		return newType(KindRecord)
	}

	return &Type{Kind: KindMap, Items: nil, Values: freeValuesType(node, defs, visited), Variants: nil, Enum: nil}
}

// freeValuesType returns the type of the values an object schema node holds
// under keys it does not name: the patternProperties schemas, in the order of
// their patterns, and the additionalProperties one make a union. Which of
// them applies depends on the key, which the type does not know, so the
// union is an approximation for a schema with several; Tarantool's schema
// never has more than one. An object that leaves the other keys free
// (additionalProperties true or absent) adds values without a type.
func freeValuesType(node map[string]any, defs map[string]any, visited map[string]bool) *Type {
	var variants []*Type

	patterns, _ := node["patternProperties"].(map[string]any)

	keys := make([]string, 0, len(patterns))
	for key := range patterns {
		keys = append(keys, key)
	}

	slices.Sort(keys)

	for _, key := range keys {
		schema, isObj := patterns[key].(map[string]any)
		if isObj {
			variants = append(variants, typeOf(schema, defs, visited))
		}
	}

	switch additional := node["additionalProperties"].(type) {
	case map[string]any:
		variants = append(variants, typeOf(additional, defs, visited))
	case bool:
		if additional {
			variants = append(variants, newType(KindAny))
		}
	case nil:
		if len(variants) == 0 {
			variants = append(variants, newType(KindAny))
		}
	}

	return unionOf(variants)
}

// hasAny reports whether node has any of the keywords.
func hasAny(node map[string]any, keywords ...string) bool {
	for _, keyword := range keywords {
		if _, ok := node[keyword]; ok {
			return true
		}
	}

	return false
}
