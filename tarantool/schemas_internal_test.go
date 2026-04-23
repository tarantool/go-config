package tarantool

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewestEmbeddedSchema_IgnoresUserRegistry(t *testing.T) {
	t.Parallel()

	want, embeddedBytes, ok := newestEmbeddedSchema()
	require.True(t, ok, "must ship at least one embedded schema")
	require.NotEmpty(t, embeddedBytes)

	// "zzz" is non-parseable — compareSemver sorts it after any numeric
	// version, which is what made the pre-split default path flaky.
	require.NoError(t, RegisterSchema("99.99.0", []byte(`{"type":"object"}`)))
	require.NoError(t, RegisterSchema("zzz", []byte(`{"type":"object"}`)))

	got, _, ok := newestEmbeddedSchema()
	require.True(t, ok)
	assert.Equal(t, want, got,
		"newestEmbeddedSchema must only consider embedded versions")
}
