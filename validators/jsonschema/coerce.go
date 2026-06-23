package jsonschema

import (
	"regexp"
	"slices"

	"github.com/kaptinlin/jsonschema"
)

// NullCoercion controls how a JSON null (produced by an empty YAML value such
// as `key:`) is treated, just before validation, when the schema at that
// location expects a scalar type (string, number, integer or boolean).
//
// Null values whose schema expects an object or an array are ALWAYS coerced to
// an empty object ({}) or empty array ([]) respectively, regardless of this
// setting: an empty mapping/sequence is the unambiguous YAML intent there. This
// knob only governs the genuinely ambiguous scalar case.
type NullCoercion int

const (
	// NullLeave keeps a scalar null as null. This is the JSON-Schema-pure
	// behaviour: if the schema does not declare the field nullable
	// (type includes "null"), validation will reject the null. Default.
	NullLeave NullCoercion = iota
	// NullDrop removes a null-valued scalar key from its parent object,
	// treating an empty value as "unset" so the field falls back to its
	// default. A required-but-empty field then fails as missing.
	NullDrop
	// NullZero replaces a scalar null with the zero value of the schema's
	// declared type: "" for string, 0 for number/integer, false for boolean.
	NullZero
)

// DefaultNullCoercion is the policy applied by validators created without an
// explicit WithNullCoercion option. Set it once at startup to change the
// behaviour globally.
//
//nolint:gochecknoglobals // intentional global default knob, set once at startup.
var DefaultNullCoercion = NullLeave

// dropMarker is returned by coerceNulls to signal that a null scalar key should
// be removed from its parent object (NullDrop policy).
type dropMarker struct{}

// coerceNulls walks data alongside the schema and rewrites null values into the
// empty shape the schema expects. Containers ({} / []) are always coerced;
// scalars follow the policy. The (possibly mutated) data is returned.
func coerceNulls(data any, schema *jsonschema.Schema, policy NullCoercion) any {
	schema = effectiveSchema(schema)

	switch typed := data.(type) {
	case nil:
		return coerceNull(schema, policy)
	case map[string]any:
		for key, child := range typed {
			coerced := coerceNulls(child, subschemaForProperty(schema, key), policy)
			if _, drop := coerced.(dropMarker); drop {
				delete(typed, key)

				continue
			}

			typed[key] = coerced
		}

		return typed
	case []any:
		for i, item := range typed {
			coerced := coerceNulls(item, subschemaForItem(schema, i), policy)
			// An array element cannot be dropped without shifting indices;
			// fall back to leaving it null.
			if _, drop := coerced.(dropMarker); drop {
				coerced = nil
			}

			typed[i] = coerced
		}

		return typed
	default:
		return data
	}
}

// coerceNull resolves a single null value against its schema.
func coerceNull(schema *jsonschema.Schema, policy NullCoercion) any {
	// An explicitly nullable schema accepts null as-is.
	if schema != nil && schemaAllows(schema, "null") {
		return nil
	}

	switch {
	case schema != nil && schemaIsObject(schema):
		return map[string]any{}
	case schema != nil && schemaIsArray(schema):
		return []any{}
	}

	switch policy {
	case NullDrop:
		return dropMarker{}
	case NullZero:
		return zeroForSchema(schema)
	case NullLeave:
		fallthrough
	default:
		return nil
	}
}

// effectiveSchema follows $ref links to the concrete schema that actually
// constrains the value. Combinators (allOf/anyOf/oneOf) are not collapsed here;
// the schemaIs*/subschemaFor* helpers look through them where needed.
func effectiveSchema(schema *jsonschema.Schema) *jsonschema.Schema {
	seen := make(map[*jsonschema.Schema]struct{})

	current := schema
	for current != nil && current.Ref != "" && current.ResolvedRef != nil {
		if _, ok := seen[current]; ok {
			break
		}

		seen[current] = struct{}{}
		current = current.ResolvedRef
	}

	return current
}

// schemaAllows reports whether the schema's type list includes the given type.
func schemaAllows(schema *jsonschema.Schema, typ string) bool {
	return slices.Contains([]string(schema.Type), typ)
}

// branches returns the combinator subschemas (allOf/anyOf/oneOf) of schema.
func branches(schema *jsonschema.Schema) []*jsonschema.Schema {
	out := make([]*jsonschema.Schema, 0, len(schema.AllOf)+len(schema.AnyOf)+len(schema.OneOf))

	out = append(out, schema.AllOf...)
	out = append(out, schema.AnyOf...)
	out = append(out, schema.OneOf...)

	return out
}

// schemaIsObject reports whether the schema describes an object, looking through
// $ref and combinators.
func schemaIsObject(schema *jsonschema.Schema) bool {
	schema = effectiveSchema(schema)
	if schema == nil {
		return false
	}

	if len(schema.Type) > 0 {
		return schemaAllows(schema, "object")
	}

	if schema.Properties != nil || schema.PatternProperties != nil ||
		schema.AdditionalProperties != nil || len(schema.Required) > 0 {
		return true
	}

	return slices.ContainsFunc(branches(schema), schemaIsObject)
}

// schemaIsArray reports whether the schema describes an array, looking through
// $ref and combinators.
func schemaIsArray(schema *jsonschema.Schema) bool {
	schema = effectiveSchema(schema)
	if schema == nil {
		return false
	}

	if len(schema.Type) > 0 {
		return schemaAllows(schema, "array")
	}

	if schema.Items != nil || len(schema.PrefixItems) > 0 {
		return true
	}

	return slices.ContainsFunc(branches(schema), schemaIsArray)
}

// subschemaForProperty resolves the schema constraining property key of an
// object schema, consulting properties, patternProperties, additionalProperties
// and combinators.
func subschemaForProperty(schema *jsonschema.Schema, key string) *jsonschema.Schema {
	schema = effectiveSchema(schema)
	if schema == nil {
		return nil
	}

	if schema.Properties != nil {
		if sub, ok := (*schema.Properties)[key]; ok {
			return sub
		}
	}

	if schema.PatternProperties != nil {
		for pattern, sub := range *schema.PatternProperties {
			re, err := regexp.Compile(pattern)
			if err == nil && re.MatchString(key) {
				return sub
			}
		}
	}

	for _, branch := range branches(schema) {
		if sub := subschemaForProperty(branch, key); sub != nil {
			return sub
		}
	}

	if schema.AdditionalProperties != nil {
		return schema.AdditionalProperties
	}

	return nil
}

// subschemaForItem resolves the schema constraining the array element at index.
func subschemaForItem(schema *jsonschema.Schema, index int) *jsonschema.Schema {
	schema = effectiveSchema(schema)
	if schema == nil {
		return nil
	}

	if index < len(schema.PrefixItems) {
		return schema.PrefixItems[index]
	}

	if schema.Items != nil {
		return schema.Items
	}

	for _, branch := range branches(schema) {
		if sub := subschemaForItem(branch, index); sub != nil {
			return sub
		}
	}

	return nil
}

// zeroForSchema returns the zero value for the schema's declared scalar type.
func zeroForSchema(schema *jsonschema.Schema) any {
	if schema == nil {
		return nil
	}

	switch {
	case schemaAllows(schema, "string"):
		return ""
	case schemaAllows(schema, "boolean"):
		return false
	case schemaAllows(schema, "integer"):
		return 0
	case schemaAllows(schema, "number"):
		return 0.0
	default:
		return nil
	}
}
