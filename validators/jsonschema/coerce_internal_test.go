package jsonschema

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestCoerce_NilSchemaGuards exercises the defensive nil-schema branches that
// the public Validate path rarely reaches: an absent subschema means the value
// is unconstrained, so the helpers must report "not a container" and yield no
// zero rather than panic.
func TestCoerce_NilSchemaGuards(t *testing.T) {
	t.Parallel()

	assert.Nil(t, effectiveSchema(nil))
	assert.False(t, schemaIsObject(nil))
	assert.False(t, schemaIsArray(nil))
	assert.Nil(t, subschemaForProperty(nil, "k"))
	assert.Nil(t, subschemaForItem(nil, 0))
	assert.Nil(t, zeroForSchema(nil))

	// A nil schema under each policy yields the policy's empty-scalar result.
	assert.Nil(t, coerceNull(nil, NullLeave))
	assert.Nil(t, coerceNull(nil, NullZero))
	assert.IsType(t, dropMarker{}, coerceNull(nil, NullDrop))
}
