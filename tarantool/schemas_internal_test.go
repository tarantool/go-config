package tarantool

import (
	"testing"

	"github.com/kaptinlin/jsonschema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tarantool/go-config/internal/race"
)

// Replaces the eager init-time validation: ensures every shipped schema
// decompresses cleanly and compiles as a valid JSON Schema. A broken embed
// payload would otherwise only surface when a caller hits that exact version.
//
// Skipped under -race and -short: kaptinlin/jsonschema's deeply recursive
// UnmarshalJSON over the largest shipped schema is ~minutes per compile under
// race instrumentation, and `make testrace` runs `-count=100`, which would
// blow the per-package timeout. The decompress+compile work has no
// concurrency to exercise, so race coverage adds nothing here.
func TestEmbeddedSchemas_AllDecompressAndCompile(t *testing.T) {
	if testing.Short() || race.Enabled() {
		t.Skip("skipping schema compile sweep under -short / -race")
	}

	t.Parallel()

	require.NotEmpty(t, embeddedVersions, "must ship at least one embedded schema")

	compiler := jsonschema.NewCompiler()

	for _, version := range embeddedVersions {
		data, ok, err := loadEmbedded(version)
		require.NoErrorf(t, err, "loadEmbedded(%q) must not error", version)
		require.Truef(t, ok, "loadEmbedded(%q) must find the version", version)
		require.NotEmptyf(t, data, "embedded schema %q must be non-empty", version)

		_, err = compiler.Compile(data)
		require.NoErrorf(t, err, "embedded schema %q must compile", version)
	}
}

func TestNewestEmbeddedSchema_IgnoresUserRegistry(t *testing.T) {
	t.Parallel()

	want, embeddedBytes, err := newestEmbeddedSchema()
	require.NoError(t, err, "must ship at least one embedded schema")
	require.NotEmpty(t, embeddedBytes)

	// "zzz" is non-parseable — compareSemver sorts it after any numeric
	// version, which is what made the pre-split default path flaky.
	require.NoError(t, RegisterSchema("99.99.0", []byte(`{"type":"object"}`)))
	require.NoError(t, RegisterSchema("zzz", []byte(`{"type":"object"}`)))

	got, _, err := newestEmbeddedSchema()
	require.NoError(t, err)
	assert.Equal(t, want, got,
		"newestEmbeddedSchema must only consider embedded versions")
}
