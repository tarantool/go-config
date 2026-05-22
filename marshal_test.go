package config_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tarantool/go-config"
	"github.com/tarantool/go-config/collectors"
)

// buildFromYAML writes data to a temp file and returns a built MutableConfig.
func buildFromYAML(t *testing.T, data string) *config.MutableConfig {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(data), 0o600))

	col, err := collectors.NewSource(t.Context(), collectors.NewFile(path), collectors.NewYamlFormat())
	require.NoError(t, err)

	builder := config.NewBuilder()

	builder = builder.WithoutValidation()
	builder = builder.AddCollector(col)

	cfg, errs := builder.BuildMutable(t.Context())
	require.Empty(t, errs)

	return &cfg
}

// TestMarshal_RoundTrip pins that an unmutated config marshals back to the
// canonical re-emission of the input — i.e., parsing then marshaling without
// any mutation does not lose key order or comments.
func TestMarshal_RoundTrip(t *testing.T) {
	t.Parallel()

	input := `# top-level head
server:
    host: localhost
    port: 8080
    tls:
        cert: a.pem
        key: a.key
client:
    name: example
    timeout: 30
`

	cfg := buildFromYAML(t, input)

	out, err := cfg.MarshalYAML()
	require.NoError(t, err)

	// Round-trip: re-parsing the output reproduces the same key order and values.
	got := buildFromYAML(t, string(out))

	for path, want := range map[string]any{
		"server/host":     "localhost",
		"server/port":     int64(8080),
		"server/tls/cert": "a.pem",
		"server/tls/key":  "a.key",
		"client/name":     "example",
		"client/timeout":  int64(30),
	} {
		var actual any

		_, err := got.Get(config.NewKeyPath(path), &actual)
		require.NoErrorf(t, err, "path %s missing after round-trip", path)
		assert.Equalf(t, want, actual, "path %s", path)
	}

	// Order is preserved at every level.
	assertKeyOrder(t, got, "", []string{"server", "client"})
	assertKeyOrder(t, got, "server", []string{"host", "port", "tls"})
	assertKeyOrder(t, got, "server/tls", []string{"cert", "key"})
	assertKeyOrder(t, got, "client", []string{"name", "timeout"})
}

// TestMarshal_PreservesOrder pins that, after Set of a new key, existing
// keys keep their source order and the new key is appended.
func TestMarshal_PreservesOrder(t *testing.T) {
	t.Parallel()

	input := `zeta: 1
alpha: 2
mu: 3
`

	cfg := buildFromYAML(t, input)

	require.NoError(t, cfg.Set(config.NewKeyPath("inserted"), "x"))

	out, err := cfg.MarshalYAML()
	require.NoError(t, err)

	got := buildFromYAML(t, string(out))

	assertKeyOrder(t, got, "", []string{"zeta", "alpha", "mu", "inserted"})
}

// TestMarshal_PreservesNestedMapOrder pins that ordered YAML map entries are
// kept even when map-valued siblings are flattened into child leaves.
func TestMarshal_PreservesNestedMapOrder(t *testing.T) {
	t.Parallel()

	input := `scope:
    nested:
        leaf: 1
    scalar: 2
    trailing:
        leaf: 3
`

	cfg := buildFromYAML(t, input)

	out, err := cfg.MarshalYAML()
	require.NoError(t, err)

	got := buildFromYAML(t, string(out))

	assertKeyOrder(t, got, "scope", []string{"nested", "scalar", "trailing"})
}

// TestMarshal_PreservesComments pins that, after a Set on one leaf, head/line/foot
// comments on neighboring nodes survive the marshal.
func TestMarshal_PreservesComments(t *testing.T) {
	t.Parallel()

	input := `# header for alpha
alpha: 1 # inline on alpha
# header for beta
beta: 2
# header for gamma
gamma: 3
`

	cfg := buildFromYAML(t, input)

	// Mutate beta. Neighbor comments must survive.
	require.NoError(t, cfg.Set(config.NewKeyPath("beta"), int64(20)))

	out, err := cfg.MarshalYAML()
	require.NoError(t, err)

	str := string(out)

	for _, fragment := range []string{
		"# header for alpha",
		"# inline on alpha",
		"# header for beta",
		"# header for gamma",
	} {
		assert.Containsf(t, str, fragment, "expected %q in output:\n%s", fragment, str)
	}
}

// TestMarshal_PreservesScalarStyle pins that single-quoted, double-quoted, and
// unquoted scalars from the source round-trip with the same style on
// un-mutated leaves.
func TestMarshal_PreservesScalarStyle(t *testing.T) {
	t.Parallel()

	input := `single: 'foo'
double: "bar"
plain: baz
literal: |
  line one
  line two
`

	cfg := buildFromYAML(t, input)

	out, err := cfg.MarshalYAML()
	require.NoError(t, err)

	str := string(out)

	// Single-quoted style retained.
	assert.Containsf(t, str, "'foo'", "single-quoted style lost:\n%s", str)
	// Double-quoted style retained.
	assert.Containsf(t, str, `"bar"`, "double-quoted style lost:\n%s", str)
	// Plain remains plain — i.e., no quotes around baz.
	assert.Regexpf(t, `(?m)^plain: baz\b`, str, "plain style lost:\n%s", str)
	// Literal block style retained.
	assert.Containsf(t, str, "literal: |", "literal block style lost:\n%s", str)
}

// TestMarshal_QuotesYAML11AmbiguousStrings pins that an unquoted source token
// that is a string under YAML 1.2 but a bool/null under YAML 1.1 (off, on,
// yes, ...) is re-emitted quoted, so a YAML 1.1 reader (e.g. Tarantool's
// libyaml loader) does not reinterpret it as a bool/null. Genuinely plain
// strings stay plain.
func TestMarshal_QuotesYAML11AmbiguousStrings(t *testing.T) {
	t.Parallel()

	input := `failover: off
toggle: on
answer: yes
denied: no
host: localhost
mode: manual
count: 8080
`

	cfg := buildFromYAML(t, input)

	out, err := cfg.MarshalYAML()
	require.NoError(t, err)

	str := string(out)

	// Ambiguous tokens become quoted.
	for _, want := range []string{
		`failover: "off"`,
		`toggle: "on"`,
		`answer: "yes"`,
		`denied: "no"`,
	} {
		assert.Containsf(t, str, want, "ambiguous token not quoted (want %q):\n%s", want, str)
	}

	// Genuine plain strings and non-string scalars are untouched.
	assert.Regexpf(t, `(?m)^host: localhost$`, str, "plain string requoted:\n%s", str)
	assert.Regexpf(t, `(?m)^mode: manual$`, str, "plain string requoted:\n%s", str)
	assert.Regexpf(t, `(?m)^count: 8080$`, str, "integer requoted:\n%s", str)

	// Values still round-trip as their original strings/ints.
	got := buildFromYAML(t, str)

	for path, want := range map[string]any{
		"failover": "off",
		"toggle":   "on",
		"answer":   "yes",
		"host":     "localhost",
		"count":    int64(8080),
	} {
		var actual any

		_, gerr := got.Get(config.NewKeyPath(path), &actual)
		require.NoErrorf(t, gerr, "path %s missing after round-trip", path)
		assert.Equalf(t, want, actual, "path %s", path)
	}
}

func assertKeyOrder(t *testing.T, cfg *config.MutableConfig, path string, want []string) {
	t.Helper()

	keys := childKeysAt(t, cfg, path)
	assert.Equalf(t, want, keys, "child key order at %q", path)
}

// childKeysAt returns the immediate child keys at the given path, in tree order.
// Walk emits leaves only, so we descend the entire subtree and collect the
// segment immediately following `path` from each leaf's full key path.
func childKeysAt(t *testing.T, cfg *config.MutableConfig, path string) []string {
	t.Helper()

	values, err := cfg.Walk(t.Context(), config.NewKeyPath(path), -1)
	require.NoError(t, err)

	var prefixLen int
	if path != "" {
		prefixLen = len(strings.Split(path, "/"))
	}

	var keys []string

	for v := range values {
		segments := v.Meta().Key
		if len(segments) <= prefixLen {
			continue
		}

		keys = appendUnique(keys, segments[prefixLen])
	}

	return keys
}

func appendUnique(s []string, v string) []string {
	if slices.Contains(s, v) {
		return s
	}

	return append(s, v)
}
