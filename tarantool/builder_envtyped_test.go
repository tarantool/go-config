package tarantool_test

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tarantool/go-config/v2"
	"github.com/tarantool/go-config/v2/internal/race"
	"github.com/tarantool/go-config/v2/tarantool"
	"github.com/tarantool/go-config/v2/tree"
)

const (
	// envTypedSchemaVersion is the newest embedded schema when these tests
	// were written. The tests that use the whole schema take the one New()
	// picks.
	envTypedSchemaVersion = "3.8.1"

	consoleSection = "console"
	enabledPath    = "console/enabled"
	countPath      = "snapshot/count"
	endpointsPath  = "config/etcd/endpoints"
	labelsPath     = "labels"
	endpointA      = "http://a:2379"
	endpointB      = "http://b:2379"
	bothEndpoints  = endpointA + "," + endpointB
	zoneEast       = "east"
	zoneKey        = "zone"
	lowerTrue      = "true"
	upperTrue      = "TRUE"
	notBoolean     = "yes"
)

// envTypedSchema retains the sections of the embedded schema the env typing
// tests set, so that a case does not compile the cluster sections.
func envTypedSchema(t *testing.T) []byte {
	t.Helper()

	data, err := tarantool.Schema(envTypedSchemaVersion)
	require.NoError(t, err)

	var schema map[string]json.RawMessage

	require.NoError(t, json.Unmarshal(data, &schema))

	var properties map[string]json.RawMessage

	require.NoError(t, json.Unmarshal(schema["properties"], &properties))

	selected := make(map[string]json.RawMessage)

	for _, section := range []string{
		"config", consoleSection, "iproto", labelsPath, "log", "memtx", "roles", "roles_cfg", "snapshot",
	} {
		require.Contains(t, properties, section)

		selected[section] = properties[section]
	}

	schema["properties"], err = json.Marshal(selected)
	require.NoError(t, err)

	data, err = json.Marshal(schema)
	require.NoError(t, err)

	return data
}

// skipUnderRace skips an env typing test under -race. Every case builds a
// config against a schema, 0.2 to 0.7 seconds under race instrumentation,
// and make testrace runs each test 100 times: the package ran past its
// 10-minute timeout. The tests set the environment, so they run
// serially and race coverage adds nothing; TestBuild_Env_SchemaAware_* keep
// the env collector under -race.
func skipUnderRace(t *testing.T) {
	t.Helper()

	if race.Enabled() {
		t.Skip("skipping env typing cases under -race")
	}
}

// getAny reads path from cfg into an any.
func getAny(t *testing.T, cfg config.Config, path string) any {
	t.Helper()

	var value any

	_, err := cfg.Get(config.NewKeyPath(path), &value)
	require.NoError(t, err, path)

	return value
}

func TestBuild_EnvTyped_EtcdEndpoints(t *testing.T) {
	skipUnderRace(t)

	for _, tt := range []struct {
		name string
		raw  string
		want []string
	}{
		{"one", "http://127.0.0.1:2379", []string{"http://127.0.0.1:2379"}},
		{"comma", bothEndpoints, []string{endpointA, endpointB}},
		{"json", `["` + endpointA + `", "` + endpointB + `"]`, []string{endpointA, endpointB}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("TT_CONFIG_ETCD_ENDPOINTS", tt.raw)
			t.Setenv("TT_CONFIG_ETCD_PREFIX", "/p")

			// The whole schema New() picks by default.
			cfg, err := tarantool.New().Build(t.Context())
			require.NoError(t, err)

			var endpoints []string

			_, err = cfg.Get(config.NewKeyPath(endpointsPath), &endpoints)
			require.NoError(t, err)
			assert.Equal(t, tt.want, endpoints)

			// tt reads the endpoints into an any and expects a []any.
			raw := getAny(t, cfg, endpointsPath)
			require.IsType(t, []any{}, raw)
			assert.Len(t, raw, len(tt.want))

			assert.Equal(t, "/p", getAny(t, cfg, "config/etcd/prefix"))
		})
	}
}

func TestBuild_EnvTyped_IprotoListen(t *testing.T) {
	skipUnderRace(t)

	t.Setenv("TT_IPROTO_LISTEN",
		`[{"uri": "127.0.0.1:3301"}, {"uri": "127.0.0.1:3302", "params": {"transport": "plain"}}]`)

	cfg, err := tarantool.New().Build(t.Context())
	require.NoError(t, err, "a JSON array of records validates")

	assert.Equal(t, "127.0.0.1:3301", getAny(t, cfg, "iproto/listen/0/uri"),
		"the array is a subtree, as it is from a file")
	assert.Equal(t, "plain", getAny(t, cfg, "iproto/listen/1/params/transport"))

	var listen []map[string]any

	_, err = cfg.Get(config.NewKeyPath("iproto/listen"), &listen)
	require.NoError(t, err)
	assert.Len(t, listen, 2)
}

func TestBuild_EnvTyped_Wildcard(t *testing.T) {
	skipUnderRace(t)

	t.Setenv("TT_GROUPS_G_REPLICASETS_R_INSTANCES_I_IPROTO_LISTEN", `[{"uri": "127.0.0.1:3303"}]`)
	t.Setenv("TT_GROUPS_G_REPLICASETS_R_INSTANCES_I_DATABASE_HOT_STANDBY", upperTrue)

	cfg, err := tarantool.New().Build(t.Context())
	require.NoError(t, err)

	inst, err := cfg.Effective(config.NewKeyPath("groups/g/replicasets/r/instances/i"))
	require.NoError(t, err)

	assert.Equal(t, "127.0.0.1:3303", getAny(t, inst, "iproto/listen/0/uri"))
	assert.Equal(t, true, getAny(t, inst, "database/hot_standby"))
}

func TestBuild_EnvTyped_Scalars(t *testing.T) {
	skipUnderRace(t)

	schema := envTypedSchema(t)

	const (
		enabled = "TT_CONSOLE_ENABLED"
		count   = "TT_SNAPSHOT_COUNT"
		memory  = "TT_MEMTX_MEMORY"
		level   = "TT_LOG_LEVEL"
		factor  = "TT_MEMTX_SLAB_ALLOC_FACTOR"
	)

	for _, tt := range []struct {
		name    string
		raw     string
		path    string
		want    any
		wantErr string
	}{
		{enabled, lowerTrue, enabledPath, true, ""},
		{enabled, upperTrue, enabledPath, true, ""},
		{enabled, "1", enabledPath, true, ""},
		{enabled, "False", enabledPath, false, ""},
		{enabled, "0", enabledPath, false, ""},
		{enabled, notBoolean, enabledPath, notBoolean, enabledPath + " [type]"},
		{count, "5", countPath, int64(5), ""},
		{count, "010", countPath, int64(10), ""},
		{count, "5.5", countPath, "5.5", countPath + " [type]"},
		{factor, "1.5", "memtx/slab_alloc_factor", 1.5, ""},
		// byte_size: a plain number is a number, a unit keeps the string.
		{memory, "1073741824", "memtx/memory", int64(1073741824), ""},
		{memory, "1GiB", "memtx/memory", "1GiB", ""},
		// A number-or-string union.
		{level, "5", "log/level", int64(5), ""},
		{level, "verbose", "log/level", "verbose", ""},
	} {
		t.Run(tt.name+"="+tt.raw, func(t *testing.T) {
			t.Setenv(tt.name, tt.raw)

			cfg, err := tarantool.New().WithSchema(schema).Build(t.Context())
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)

				// Best-effort readers still get the raw string.
				cfg, err = tarantool.New().WithSchema(schema).WithoutValidation().Build(t.Context())
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, getAny(t, cfg, tt.path))
		})
	}
}

func TestBuild_EnvTyped_StringFieldKeepsDigits(t *testing.T) {
	skipUnderRace(t)

	t.Setenv("TT_CONSOLE_SOCKET", "123")
	t.Setenv("TT_LABELS_ZONE", "007")

	cfg, err := tarantool.New().WithSchema(envTypedSchema(t)).Build(t.Context())
	require.NoError(t, err, "a string field holding digits validates as a string")

	assert.Equal(t, "123", getAny(t, cfg, "console/socket"))
	assert.Equal(t, "007", getAny(t, cfg, "labels/zone"))

	var socket int

	_, err = cfg.Get(config.NewKeyPath("console/socket"), &socket)
	require.ErrorIs(t, err, tree.ErrFixedTypeString, "the schema fixed the value as a string")
}

func TestBuild_EnvTyped_Map(t *testing.T) {
	skipUnderRace(t)

	schema := envTypedSchema(t)

	t.Run("simple", func(t *testing.T) {
		t.Setenv("TT_LABELS", "zone=east,rack=7")

		cfg, err := tarantool.New().WithSchema(schema).Build(t.Context())
		require.NoError(t, err)
		assert.Equal(t, map[string]any{zoneKey: zoneEast, "rack": "7"}, getAny(t, cfg, labelsPath))
	})

	t.Run("json", func(t *testing.T) {
		t.Setenv("TT_LABELS", `{"zone": "east"}`)
		t.Setenv("TT_ROLES_CFG", `{"myrole": {"limit": 10, "names": ["a", "b"]}}`)

		cfg, err := tarantool.New().WithSchema(schema).Build(t.Context())
		require.NoError(t, err)
		assert.Equal(t, map[string]any{zoneKey: zoneEast}, getAny(t, cfg, labelsPath))
		assert.Equal(t, int64(10), getAny(t, cfg, "roles_cfg/myrole/limit"))
		assert.Equal(t, "b", getAny(t, cfg, "roles_cfg/myrole/names/1"))
	})

	t.Run("overlap", func(t *testing.T) {
		// The more specific variable lands inside the map.
		t.Setenv("TT_LABELS", "zone=east,rack=7")
		t.Setenv("TT_LABELS_RACK", "9")

		cfg, err := tarantool.New().WithSchema(schema).Build(t.Context())
		require.NoError(t, err)
		assert.Equal(t, map[string]any{zoneKey: zoneEast, "rack": "9"}, getAny(t, cfg, labelsPath))
	})

	t.Run("merges with file", func(t *testing.T) {
		cfgPath := filepath.Join(t.TempDir(), "config.yaml")
		writeFile(t, cfgPath, "labels:\n  zone: west\n  dc: one\n")

		t.Setenv("TT_LABELS", "zone=east")

		cfg, err := tarantool.New().WithConfigFile(cfgPath).WithSchema(schema).Build(t.Context())
		require.NoError(t, err)
		assert.Equal(t, map[string]any{zoneKey: zoneEast, "dc": "one"}, getAny(t, cfg, labelsPath),
			"maps merge deep, as Tarantool merges them")
	})

	t.Run("untyped values need JSON", func(t *testing.T) {
		t.Setenv("TT_ROLES_CFG", "myrole=x")

		_, err := tarantool.New().WithSchema(schema).Build(t.Context())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "roles_cfg [type]")
	})
}

func TestBuild_EnvTyped_Unparseable(t *testing.T) {
	skipUnderRace(t)

	schema := envTypedSchema(t)

	for _, tt := range []struct {
		name string
		raw  string
		path string
	}{
		{"TT_CONFIG_ETCD_ENDPOINTS", `{"a": 1}`, endpointsPath},
		{"TT_ROLES", "[broken", "roles"},
		{"TT_IPROTO_LISTEN", "127.0.0.1:3301", "iproto/listen"},
		{"TT_CONSOLE", "x", consoleSection},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(tt.name, tt.raw)

			_, err := tarantool.New().WithSchema(schema).Build(t.Context())
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.path+" [type] invalid type: Value is string")

			cfg, err := tarantool.New().WithSchema(schema).WithoutValidation().Build(t.Context())
			require.NoError(t, err)
			assert.Equal(t, tt.raw, getAny(t, cfg, tt.path), "the raw string is kept")
		})
	}
}

func TestBuild_EnvTyped_EmptyIsUnset(t *testing.T) {
	skipUnderRace(t)

	schema := envTypedSchema(t)

	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	writeFile(t, cfgPath, "console:\n  socket: file.sock\n")

	t.Setenv("TT_CONSOLE_SOCKET", "")
	t.Setenv("TT_CONSOLE_ENABLED", "")
	t.Setenv("TT_CONFIG_ETCD_ENDPOINTS", "")

	cfg, err := tarantool.New().WithConfigFile(cfgPath).WithSchema(schema).Build(t.Context())
	require.NoError(t, err, "an empty boolean is unset, not an invalid string")

	assert.Equal(t, "file.sock", getAny(t, cfg, "console/socket"), "an empty value does not override")

	_, ok := cfg.Lookup(config.NewKeyPath(enabledPath))
	assert.False(t, ok)

	_, ok = cfg.Lookup(config.NewKeyPath(endpointsPath))
	assert.False(t, ok)
}

func TestBuild_EnvTyped_Default(t *testing.T) {
	skipUnderRace(t)

	schema := envTypedSchema(t)

	t.Setenv("TT_CONFIG_ETCD_ENDPOINTS_DEFAULT", bothEndpoints)
	t.Setenv("TT_CONSOLE_ENABLED_DEFAULT", "false")
	t.Setenv("TT_SNAPSHOT_COUNT_DEFAULT", "3")

	cfg, err := tarantool.New().WithSchema(schema).Build(t.Context())
	require.NoError(t, err)

	assert.Equal(t, []any{endpointA, endpointB}, getAny(t, cfg, endpointsPath))
	assert.Equal(t, false, getAny(t, cfg, enabledPath))
	assert.Equal(t, int64(3), getAny(t, cfg, countPath))

	// The file beats the defaults, and its array replaces theirs whole.
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	writeFile(t, cfgPath, "config:\n  etcd:\n    endpoints: [http://f:2379]\n")

	cfg, err = tarantool.New().WithConfigFile(cfgPath).WithSchema(schema).Build(t.Context())
	require.NoError(t, err)
	assert.Equal(t, []any{"http://f:2379"}, getAny(t, cfg, endpointsPath))
}

func TestBuild_EnvTyped_ArrayReplacesFileArray(t *testing.T) {
	skipUnderRace(t)

	schema := envTypedSchema(t)

	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	writeFile(t, cfgPath, `config:
  etcd:
    endpoints: [http://x:2379, http://y:2379, http://z:2379]
iproto:
  listen:
    - uri: 127.0.0.1:3301
      params:
        transport: ssl
    - uri: 127.0.0.1:3302
roles: [a, b, c]
`)

	t.Setenv("TT_CONFIG_ETCD_ENDPOINTS", endpointA)
	t.Setenv("TT_IPROTO_LISTEN", `[{"uri": "127.0.0.1:4401"}]`)
	t.Setenv("TT_ROLES", "[]")

	cfg, err := tarantool.New().WithConfigFile(cfgPath).WithSchema(schema).Build(t.Context())
	require.NoError(t, err)

	assert.Equal(t, []any{endpointA}, getAny(t, cfg, endpointsPath))
	assert.Equal(t, []any{map[string]any{"uri": "127.0.0.1:4401"}}, getAny(t, cfg, "iproto/listen"),
		"no item or field of the file array leaks into the env one")
	assert.Equal(t, []any{}, getAny(t, cfg, "roles"))
}

func TestBuild_EnvTyped_EnvIgnore(t *testing.T) {
	skipUnderRace(t)

	t.Setenv("TT_CONSOLE_ENABLED", "false")
	t.Setenv("TT_CONSOLE_SOCKET", "env.sock")

	cfg, err := tarantool.New().
		WithSchema(envTypedSchema(t)).
		WithEnvIgnore("TT_CONSOLE_EN*").
		Build(t.Context())
	require.NoError(t, err)

	_, ok := cfg.Lookup(config.NewKeyPath(enabledPath))
	assert.False(t, ok, "the ignored variable is dropped")
	assert.Equal(t, "env.sock", getAny(t, cfg, "console/socket"))
}

func TestBuild_EnvTyped_WithoutSchemaUnchanged(t *testing.T) {
	t.Setenv("TT_CONFIG_ETCD_ENDPOINTS", bothEndpoints)
	t.Setenv("TT_CONSOLE_ENABLED", lowerTrue)
	t.Setenv("TT_CONSOLE_SOCKET", "")

	cfg, err := tarantool.New().WithoutSchema().Build(t.Context())
	require.NoError(t, err)

	assert.Equal(t, bothEndpoints, getAny(t, cfg, endpointsPath))
	assert.Equal(t, lowerTrue, getAny(t, cfg, enabledPath))
	assert.Empty(t, getAny(t, cfg, "console/socket"), "an empty value is kept without a schema")

	var enabled bool

	_, err = cfg.Get(config.NewKeyPath(enabledPath), &enabled)
	require.NoError(t, err, "an env string still converts on Get")
	assert.True(t, enabled)
}
