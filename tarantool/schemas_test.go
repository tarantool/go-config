package tarantool_test

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tarantool/go-config/tarantool"
)

var minimalValidSchema = []byte(`{"type":"object"}`) //nolint:gochecknoglobals // test fixtures

var invalidJSONBytes = []byte(`{not valid json}`) //nolint:gochecknoglobals // test fixtures

// invalidSchemaBytes parses as JSON but is rejected by the compiler because
// "type" must be a string or array, not an integer.
var invalidSchemaBytes = []byte(`{"type":42}`) //nolint:gochecknoglobals // test fixtures

func TestRegisterSchema_ValidSchema(t *testing.T) {
	t.Parallel()

	// Versions outside the embedded set to avoid collisions.
	const ver = "99.1.0"

	err := tarantool.RegisterSchema(ver, minimalValidSchema)
	require.NoError(t, err)

	got, ok := tarantool.Schema(ver)
	require.True(t, ok)
	assert.Equal(t, minimalValidSchema, got)
}

func TestRegisterSchema_DefensiveCopy(t *testing.T) {
	t.Parallel()

	const ver = "99.2.0"

	input := []byte(`{"type":"object"}`)

	err := tarantool.RegisterSchema(ver, input)
	require.NoError(t, err)

	got1, ok := tarantool.Schema(ver)
	require.True(t, ok)

	got1[0] = 'X'

	got2, ok := tarantool.Schema(ver)
	require.True(t, ok)
	assert.JSONEq(t, `{"type":"object"}`, string(got2), "stored bytes must not be affected by mutating a returned copy")
}

func TestRegisterSchema_InputDefensiveCopy(t *testing.T) {
	t.Parallel()

	const ver = "99.3.0"

	input := []byte(`{"type":"object"}`)
	original := make([]byte, len(input))
	copy(original, input)

	err := tarantool.RegisterSchema(ver, input)
	require.NoError(t, err)

	input[0] = 'Z'

	got, ok := tarantool.Schema(ver)
	require.True(t, ok)
	assert.Equal(t, original, got, "stored bytes must not be affected by mutating the input slice after registration")
}

func TestRegisterSchema_InvalidJSON(t *testing.T) {
	t.Parallel()

	const ver = "99.4.0"

	err := tarantool.RegisterSchema(ver, invalidJSONBytes)
	require.Error(t, err)
	require.ErrorIsf(t, err, tarantool.ErrInvalidSchema, "error must wrap ErrInvalidSchema, got: %v", err)

	_, ok := tarantool.Schema(ver)
	assert.False(t, ok, "Schema() must return (nil, false) after a failed registration")
}

func TestRegisterSchema_InvalidSchema(t *testing.T) {
	t.Parallel()

	const ver = "99.5.0"

	err := tarantool.RegisterSchema(ver, invalidSchemaBytes)
	require.Error(t, err)
	require.ErrorIsf(t, err, tarantool.ErrInvalidSchema, "error must wrap ErrInvalidSchema, got: %v", err)

	_, ok := tarantool.Schema(ver)
	assert.False(t, ok, "Schema() must return (nil, false) after a failed registration")
}

func TestRegisterSchema_OverwriteVersion(t *testing.T) {
	t.Parallel()

	const ver = "99.6.0"

	first := []byte(`{"type":"object","description":"first"}`)
	second := []byte(`{"type":"object","description":"second"}`)

	err := tarantool.RegisterSchema(ver, first)
	require.NoError(t, err)

	err = tarantool.RegisterSchema(ver, second)
	require.NoError(t, err)

	got, ok := tarantool.Schema(ver)
	require.True(t, ok)
	assert.Equal(t, second, got, "second registration must overwrite the first")
}

func TestSchemaVersions_ContainsEmbedded(t *testing.T) {
	t.Parallel()

	versions := tarantool.SchemaVersions()

	assert.Contains(t, versions, "3.5.0")
	assert.Contains(t, versions, "3.6.0")
	assert.Contains(t, versions, "3.7.0")
}

// Semver ordering, not lexicographic — lexicographic would place "3.10.0"
// before "3.5.0".
func TestSchemaVersions_SemverOrder(t *testing.T) {
	t.Parallel()

	err := tarantool.RegisterSchema("3.10.0", minimalValidSchema)
	require.NoError(t, err)

	versions := tarantool.SchemaVersions()

	pos350, pos310 := -1, -1

	for i, v := range versions {
		switch v {
		case "3.5.0":
			pos350 = i
		case "3.10.0":
			pos310 = i
		}
	}

	require.NotEqual(t, -1, pos350, "3.5.0 must appear in SchemaVersions()")
	require.NotEqual(t, -1, pos310, "3.10.0 must appear in SchemaVersions()")
	assert.Greater(t, pos310, pos350, "3.10.0 must come after 3.5.0 in semver order")
}

//nolint:varnamelen // standard WaitGroup alias / goroutine index
func TestConcurrentSafety(t *testing.T) {
	t.Parallel()

	const goroutines = 50

	var wg sync.WaitGroup

	wg.Add(goroutines)

	for i := range goroutines {
		go func(n int) {
			defer wg.Done()

			if n%2 == 0 {
				// 98.x.x avoids collision with embedded set and other tests.
				ver := "98.0." + string(rune('0'+n%10))

				_ = tarantool.RegisterSchema(ver, minimalValidSchema)
			}

			_ = tarantool.SchemaVersions()

			_, _ = tarantool.Schema("3.5.0")
			_, _ = tarantool.Schema("3.6.0")
			_, _ = tarantool.Schema("3.7.0")
		}(i)
	}

	wg.Wait()
}
