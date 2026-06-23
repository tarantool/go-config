package jsonschema_test

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tarantool/go-config/keypath"
	"github.com/tarantool/go-config/tree"
	"github.com/tarantool/go-config/validators/jsonschema"
)

// tlsSchema mirrors the shape that breaks TCM: a scalar string field, an array
// field and an object sub-tree, where the user leaves the YAML values empty
// (null). cert-file is NOT nullable, exercising the scalar-null policy.
const tlsSchema = `{
	"$schema": "https://json-schema.org/draft/2020-12/schema",
	"type": "object",
	"properties": {
		"http": {
			"type": "object",
			"properties": {
				"tls": {
					"type": "object",
					"properties": {
						"cert-file": { "type": "string" },
						"cipher-suites": {
							"type": "array",
							"items": { "type": "string" }
						}
					}
				}
			}
		},
		"instances": {
			"type": "object",
			"additionalProperties": { "type": "object" }
		}
	}
}`

// emptyValuesTree models the parsed config where every value is left empty:
//
//	http: { tls: { cert-file:, cipher-suites: } }
//	instances: { inst-a: }
func emptyValuesTree() *tree.Node {
	root := tree.New()
	root.Set(keypath.NewKeyPath("http/tls/cert-file"), nil)
	root.Set(keypath.NewKeyPath("http/tls/cipher-suites"), nil)
	root.Set(keypath.NewKeyPath("instances/inst-a"), nil)

	return root
}

func TestCoerce_ContainerNullsAlwaysFixed(t *testing.T) {
	t.Parallel()

	// Regardless of policy, an array-typed null becomes [] and an object-typed
	// null becomes {}; only the scalar cert-file is policy-dependent. With the
	// nullable variant below we isolate the container behaviour.
	for _, policy := range []jsonschema.NullCoercion{
		jsonschema.NullLeave, jsonschema.NullDrop, jsonschema.NullZero,
	} {
		validator, err := jsonschema.New([]byte(tlsSchema), jsonschema.WithNullCoercion(policy))
		require.NoError(t, err)

		root := tree.New()
		root.Set(keypath.NewKeyPath("http/tls/cipher-suites"), nil)
		root.Set(keypath.NewKeyPath("instances/inst-a"), nil)

		errs := validator.Validate(root)
		assert.Empty(t, errs, "container nulls must validate for policy %d", policy)
	}
}

func TestCoerce_ScalarNull_Leave(t *testing.T) {
	t.Parallel()

	// NullLeave keeps cert-file null; a non-nullable string schema rejects it.
	validator, err := jsonschema.New([]byte(tlsSchema), jsonschema.WithNullCoercion(jsonschema.NullLeave))
	require.NoError(t, err)

	errs := validator.Validate(emptyValuesTree())
	require.NotEmpty(t, errs)
}

func TestCoerce_ScalarNull_Zero(t *testing.T) {
	t.Parallel()

	// NullZero turns cert-file into "", which satisfies the string schema.
	validator, err := jsonschema.New([]byte(tlsSchema), jsonschema.WithNullCoercion(jsonschema.NullZero))
	require.NoError(t, err)

	errs := validator.Validate(emptyValuesTree())
	assert.Empty(t, errs)
}

func TestCoerce_ScalarNull_Drop(t *testing.T) {
	t.Parallel()

	// NullDrop removes the empty cert-file key; the optional field is absent.
	validator, err := jsonschema.New([]byte(tlsSchema), jsonschema.WithNullCoercion(jsonschema.NullDrop))
	require.NoError(t, err)

	errs := validator.Validate(emptyValuesTree())
	assert.Empty(t, errs)
}

func TestCoerce_DefaultPolicyIsLeave(t *testing.T) {
	t.Parallel()

	// A validator created without an option uses DefaultNullCoercion (Leave).
	assert.Equal(t, jsonschema.NullLeave, jsonschema.DefaultNullCoercion)

	validator, err := jsonschema.New([]byte(tlsSchema))
	require.NoError(t, err)

	errs := validator.Validate(emptyValuesTree())
	require.NotEmpty(t, errs)
}

func TestCoerce_NullableScalarStaysNull(t *testing.T) {
	t.Parallel()

	// A field declared nullable accepts null under every policy, and must not
	// be coerced to "".
	schema := `{
		"type": "object",
		"properties": {
			"name": { "type": ["string", "null"] }
		}
	}`

	for _, policy := range []jsonschema.NullCoercion{
		jsonschema.NullLeave, jsonschema.NullDrop, jsonschema.NullZero,
	} {
		validator, err := jsonschema.New([]byte(schema), jsonschema.WithNullCoercion(policy))
		require.NoError(t, err)

		root := tree.New()
		root.Set(keypath.NewKeyPath("name"), nil)

		errs := validator.Validate(root)
		assert.Empty(t, errs, "nullable scalar must validate for policy %d", policy)
	}
}

// arrayNode builds an array node with the given children appended in order.
func arrayNode(children ...*tree.Node) *tree.Node {
	arr := tree.New()
	arr.MarkArray()

	for i, child := range children {
		arr.SetChild(strconv.Itoa(i), child)
	}

	return arr
}

func TestCoerce_RefResolvesToObject(t *testing.T) {
	t.Parallel()

	// A property defined via $ref to an object schema: an empty value is
	// coerced to {} after following ResolvedRef (effectiveSchema + schemaIsObject).
	schema := `{
		"type": "object",
		"properties": { "inst": { "$ref": "#/$defs/Obj" } },
		"$defs": { "Obj": { "type": "object", "properties": { "name": { "type": "string" } } } }
	}`
	validator, err := jsonschema.New([]byte(schema))
	require.NoError(t, err)

	root := tree.New()
	root.Set(keypath.NewKeyPath("inst"), nil)

	assert.Empty(t, validator.Validate(root))
}

func TestCoerce_ArrayItemsObject(t *testing.T) {
	t.Parallel()

	// A null element of an array-of-objects is coerced to {} via subschemaForItem.
	schema := `{
		"type": "object",
		"properties": { "servers": { "type": "array", "items": { "type": "object" } } }
	}`
	validator, err := jsonschema.New([]byte(schema))
	require.NoError(t, err)

	root := tree.New()
	root.SetChild("servers", arrayNode(tree.New()))

	assert.Empty(t, validator.Validate(root))
}

func TestCoerce_PrefixItemsZero(t *testing.T) {
	t.Parallel()

	// Tuple (prefixItems) elements left empty become typed zeros under NullZero,
	// exercising subschemaForItem's prefix path and zeroForSchema string+integer.
	schema := `{
		"type": "object",
		"properties": { "pair": { "type": "array", "prefixItems": [ { "type": "string" }, { "type": "integer" } ] } }
	}`
	validator, err := jsonschema.New([]byte(schema), jsonschema.WithNullCoercion(jsonschema.NullZero))
	require.NoError(t, err)

	root := tree.New()
	root.SetChild("pair", arrayNode(tree.New(), tree.New()))

	assert.Empty(t, validator.Validate(root))
}

func TestCoerce_PatternProperties(t *testing.T) {
	t.Parallel()

	// An empty value under a patternProperties object schema becomes {}.
	schema := `{ "type": "object", "patternProperties": { "^x-": { "type": "object" } } }`
	validator, err := jsonschema.New([]byte(schema))
	require.NoError(t, err)

	root := tree.New()
	root.Set(keypath.NewKeyPath("x-meta"), nil)

	assert.Empty(t, validator.Validate(root))
}

func TestCoerce_CombinatorObjectAndArray(t *testing.T) {
	t.Parallel()

	// Type is reachable only through allOf/anyOf branches, exercising
	// schemaIsObject/schemaIsArray's combinator descent.
	schema := `{
		"type": "object",
		"properties": {
			"obj": { "allOf": [ { "type": "object" } ] },
			"arr": { "anyOf": [ { "type": "array", "items": { "type": "string" } } ] }
		}
	}`
	validator, err := jsonschema.New([]byte(schema))
	require.NoError(t, err)

	root := tree.New()
	root.Set(keypath.NewKeyPath("obj"), nil)
	root.Set(keypath.NewKeyPath("arr"), nil)

	assert.Empty(t, validator.Validate(root))
}

func TestCoerce_ZeroAllScalarTypes(t *testing.T) {
	t.Parallel()

	// NullZero produces the typed zero for every scalar kind.
	schema := `{
		"type": "object",
		"properties": {
			"b": { "type": "boolean" },
			"n": { "type": "number" },
			"i": { "type": "integer" },
			"s": { "type": "string" }
		}
	}`
	validator, err := jsonschema.New([]byte(schema), jsonschema.WithNullCoercion(jsonschema.NullZero))
	require.NoError(t, err)

	root := tree.New()
	for _, key := range []string{"b", "n", "i", "s"} {
		root.Set(keypath.NewKeyPath(key), nil)
	}

	assert.Empty(t, validator.Validate(root))
}

func TestCoerce_DropInArrayFallsBackToNull(t *testing.T) {
	t.Parallel()

	// A scalar array element cannot be dropped (indices would shift), so NullDrop
	// falls back to leaving it null — which a non-nullable item schema rejects.
	schema := `{
		"type": "object",
		"properties": { "tags": { "type": "array", "items": { "type": "string" } } }
	}`
	validator, err := jsonschema.New([]byte(schema), jsonschema.WithNullCoercion(jsonschema.NullDrop))
	require.NoError(t, err)

	root := tree.New()
	root.SetChild("tags", arrayNode(tree.New()))

	require.NotEmpty(t, validator.Validate(root))
}

func TestCoerce_TypelessContainersByKeyword(t *testing.T) {
	t.Parallel()

	// Schemas that omit "type" but carry object/array keywords are still
	// recognized (schemaIsObject via Properties, schemaIsArray via items).
	schema := `{
		"type": "object",
		"properties": {
			"obj": { "properties": { "a": { "type": "string" } } },
			"arr": { "items": { "type": "string" } }
		}
	}`
	validator, err := jsonschema.New([]byte(schema))
	require.NoError(t, err)

	root := tree.New()
	root.Set(keypath.NewKeyPath("obj"), nil)
	root.Set(keypath.NewKeyPath("arr"), nil)

	assert.Empty(t, validator.Validate(root))
}

func TestCoerce_UnconstrainedNullKeepsPolicy(t *testing.T) {
	t.Parallel()

	// A null at a path the schema does not describe has no subschema, so
	// coercion falls through to the policy with a nil schema (subschemaForProperty
	// and subschemaForItem returning nil, zeroForSchema(nil)).
	schema := `{
		"type": "object",
		"properties": { "arr": { "type": "array" } }
	}`
	validator, err := jsonschema.New([]byte(schema), jsonschema.WithNullCoercion(jsonschema.NullZero))
	require.NoError(t, err)

	root := tree.New()
	root.Set(keypath.NewKeyPath("extra"), nil)  // no property/additionalProperties schema.
	root.SetChild("arr", arrayNode(tree.New())) // array, no items schema.

	assert.Empty(t, validator.Validate(root))
}
