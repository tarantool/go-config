package jsonschema_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tarantool/go-config/v2/keypath"
	"github.com/tarantool/go-config/v2/tree"
	"github.com/tarantool/go-config/v2/validators/jsonschema"
)

// scalarSchema mirrors a config with strict scalar types plus a union field.
// Environment-variable collectors deliver every value as a string, so these
// strict types would reject env overrides without scalar coercion.
const scalarSchema = `{
	"$schema": "https://json-schema.org/draft/2020-12/schema",
	"type": "object",
	"properties": {
		"flag": { "type": "boolean" },
		"count": { "type": "integer" },
		"ratio": { "type": "number" },
		"name": { "type": "string" },
		"either": { "type": ["boolean", "string"] },
		"nested": {
			"type": "object",
			"properties": {
				"on": { "type": "boolean" }
			}
		},
		"flags": {
			"type": "array",
			"items": { "type": "boolean" }
		}
	}
}`

// stringTree builds a config tree where key holds the given string value, the
// shape an environment-variable collector produces.
func stringTree(key, value string) *tree.Node {
	root := tree.New()
	root.Set(keypath.NewKeyPath(key), value)

	return root
}

// leaf builds a scalar leaf node holding the given string value.
func leaf(value string) *tree.Node {
	node := tree.New()

	node.Value = value

	return node
}

func TestCoerceScalars_BoolStringForms(t *testing.T) {
	t.Parallel()

	validator, err := jsonschema.New([]byte(scalarSchema))
	require.NoError(t, err)

	// Every string strconv.ParseBool accepts must validate against "boolean".
	for _, form := range []string{"true", "false", "1", "0", "t", "T", "f", "F", "TRUE", "False"} {
		errs := validator.Validate(stringTree("flag", form))
		assert.Emptyf(t, errs, "flag=%q should coerce to boolean and validate", form)
	}
}

func TestCoerceScalars_IntegerAndNumber(t *testing.T) {
	t.Parallel()

	validator, err := jsonschema.New([]byte(scalarSchema))
	require.NoError(t, err)

	assert.Empty(t, validator.Validate(stringTree("count", "123")), "integer string should validate")
	assert.Empty(t, validator.Validate(stringTree("ratio", "1.5")), "number string should validate")
}

func TestCoerceScalars_UnparseableStringStillFails(t *testing.T) {
	t.Parallel()

	validator, err := jsonschema.New([]byte(scalarSchema))
	require.NoError(t, err)

	// "yes" is not accepted by strconv.ParseBool, so it is left as a string and
	// the strict boolean type rejects it.
	assert.NotEmpty(t, validator.Validate(stringTree("flag", "yes")),
		"unparsable bool string must not be coerced and must fail validation")
	assert.NotEmpty(t, validator.Validate(stringTree("count", "abc")),
		"unparsable integer string must fail validation")
}

func TestCoerceScalars_StringFieldUntouched(t *testing.T) {
	t.Parallel()

	validator, err := jsonschema.New([]byte(scalarSchema))
	require.NoError(t, err)

	// A genuine string field accepts any string; it must never be coerced.
	assert.Empty(t, validator.Validate(stringTree("name", "true")),
		"string field must accept a string value unchanged")
}

func TestCoerceScalars_UnionKeepsString(t *testing.T) {
	t.Parallel()

	validator, err := jsonschema.New([]byte(scalarSchema))
	require.NoError(t, err)

	// A ["boolean", "string"] union already permits the string, so no coercion
	// is needed and the string form validates as-is.
	assert.Empty(t, validator.Validate(stringTree("either", "true")),
		"union type must accept the string form without coercion")
}

func TestCoerceScalars_NestedAndArrayItems(t *testing.T) {
	t.Parallel()

	validator, err := jsonschema.New([]byte(scalarSchema))
	require.NoError(t, err)

	assert.Empty(t, validator.Validate(stringTree("nested/on", "true")),
		"nested object scalar string should coerce")

	root := tree.New()
	root.SetChild("flags", arrayNode(leaf("true"), leaf("false")))

	assert.Empty(t, validator.Validate(root), "array item scalar strings should coerce")
}
