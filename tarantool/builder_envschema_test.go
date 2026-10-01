package tarantool_test

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tarantool/go-config/v2"
	"github.com/tarantool/go-config/v2/tarantool"
	"github.com/tarantool/go-config/v2/validators/jsonschema"
)

const fixtureSchemaPath = "testdata/config.schema.json"

// tarantoolScalarSchema retains the tested sections of the embedded schema.
// Compiling the unrelated cluster sections for every table entry is too costly
// under make testrace, which runs each test 100 times with race instrumentation.
func tarantoolScalarSchema(t *testing.T) []byte {
	t.Helper()

	data, err := tarantool.Schema("3.8.0")
	require.NoError(t, err)

	var schema map[string]json.RawMessage

	require.NoError(t, json.Unmarshal(data, &schema))

	var properties map[string]json.RawMessage

	require.NoError(t, json.Unmarshal(schema["properties"], &properties))

	selected := make(map[string]json.RawMessage)

	for _, section := range []string{"console", "iproto", "memtx"} {
		require.Contains(t, properties, section)

		selected[section] = properties[section]
	}

	schema["properties"], err = json.Marshal(selected)
	require.NoError(t, err)

	data, err = json.Marshal(schema)
	require.NoError(t, err)

	return data
}

func TestBuild_TarantoolParserFormatting_WithSchema(t *testing.T) {
	t.Parallel()

	schema := tarantoolScalarSchema(t)

	const (
		booleanPath = "console/enabled"
		integerPath = "iproto/threads"
		numberPath  = "memtx/slab_alloc_factor"
		stringPath  = "console/socket"
		titleTrue   = "True"
		titleFalse  = "False"
		upperFalse  = "FALSE"
		upperYes    = "YES"
		separated   = "1_0.5"
		trueValue   = "true"
		falseValue  = "false"
	)

	tests := []struct {
		path  string
		input string
		want  any
		valid bool
	}{
		{booleanPath, "no", false, true},
		{booleanPath, "yes", true, true},
		{booleanPath, trueValue, true, true},
		{booleanPath, falseValue, false, true},
		{booleanPath, titleTrue, titleTrue, false},
		{booleanPath, titleFalse, titleFalse, false},
		{booleanPath, "TRUE", "TRUE", false},
		{booleanPath, upperFalse, upperFalse, false},
		{booleanPath, upperYes, upperYes, false},
		{booleanPath, "NO", "NO", false},
		{booleanPath, "\"no\"", "no", false},
		{booleanPath, "\"true\"", trueValue, false},
		{booleanPath, "'true'", trueValue, false},
		{booleanPath, "\"false\"", falseValue, false},
		{booleanPath, "'false'", falseValue, false},
		{integerPath, "020", int64(20), true},
		{integerPath, "09", int64(9), true},
		{integerPath, "1_000", "1_000", false},
		{integerPath, "\"020\"", "020", false},
		{numberPath, "01.5", float64(1.5), true},
		{numberPath, separated, separated, false},
		{numberPath, "1_000", "1_000", false},
		{stringPath, titleTrue, titleTrue, true},
		{stringPath, titleFalse, titleFalse, true},
		{stringPath, "1_000", "1_000", true},
		{stringPath, "\"true\"", trueValue, true},
		{stringPath, "'true'", trueValue, true},
		{stringPath, "\"false\"", falseValue, true},
		{stringPath, "'false'", falseValue, true},
		{stringPath, "no", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.path+"="+tt.input, func(t *testing.T) {
			t.Parallel()

			parts := strings.Split(tt.path, "/")
			cfgPath := filepath.Join(t.TempDir(), "config.yaml")
			writeFile(t, cfgPath, fmt.Sprintf("%s:\n  %s: %s\n", parts[0], parts[1], tt.input))

			cfg, err := tarantool.New().
				WithConfigFile(cfgPath).
				WithSchema(schema).
				WithEnvPrefix("GO_CONFIG_SCHEMA_TEST_").
				Build(t.Context())
			if !tt.valid {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.path+" [type]")

				return
			}

			require.NoError(t, err)

			var value any

			_, err = cfg.Get(config.NewKeyPath(tt.path), &value)
			require.NoError(t, err)
			assert.Equal(t, tt.want, value)
		})
	}
}

func TestBuild_TarantoolParserFormatting_WithEmbeddedSchema(t *testing.T) {
	t.Parallel()

	const titleTrue = "True"

	for _, enabled := range []string{"no", titleTrue} {
		t.Run(enabled, func(t *testing.T) {
			t.Parallel()

			cfgPath := filepath.Join(t.TempDir(), "config.yaml")
			writeFile(t, cfgPath, "console:\n  enabled: "+enabled+"\n  socket: \"true\"\n"+
				"iproto:\n  threads: 020\nmemtx:\n  slab_alloc_factor: 01.5\n")

			cfg, err := tarantool.New().
				WithConfigFile(cfgPath).
				WithSchemaVersion("3.8.0").
				WithEnvPrefix("GO_CONFIG_SCHEMA_TEST_").
				Build(t.Context())
			if enabled == titleTrue {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "console/enabled [type]")

				return
			}

			require.NoError(t, err)

			for path, want := range map[string]any{
				"console/enabled":         false,
				"console/socket":          "true",
				"iproto/threads":          int64(20),
				"memtx/slab_alloc_factor": float64(1.5),
			} {
				var value any

				_, err = cfg.Get(config.NewKeyPath(path), &value)
				require.NoError(t, err)
				assert.Equal(t, want, value, path)
			}
		})
	}
}

func TestBuild_Env_SchemaAware_AuditLog(t *testing.T) {
	t.Setenv("TT_AUDIT_LOG_NONBLOCK", "true")

	ctx := context.Background()

	cfg, err := tarantool.New().
		WithSchemaFile(fixtureSchemaPath).
		Build(ctx)
	require.NoError(t, err)

	var nonblock string

	_, err = cfg.Get(config.NewKeyPath("audit_log/nonblock"), &nonblock)
	require.NoError(t, err)
	assert.Equal(t, "true", nonblock)
}

func TestBuild_Env_SchemaAware_WalQueueMaxSize(t *testing.T) {
	t.Setenv("TT_WAL_QUEUE_MAX_SIZE", "123")

	ctx := context.Background()

	cfg, err := tarantool.New().
		WithSchemaFile(fixtureSchemaPath).
		Build(ctx)
	require.NoError(t, err)

	var size string

	_, err = cfg.Get(config.NewKeyPath("wal_queue_max_size"), &size)
	require.NoError(t, err)
	assert.Equal(t, "123", size)
}

func TestBuild_Env_SchemaAware_ReplicationFailover(t *testing.T) {
	t.Setenv("TT_REPLICATION_FAILOVER", "manual")

	ctx := context.Background()

	cfg, err := tarantool.New().
		WithSchemaFile(fixtureSchemaPath).
		Build(ctx)
	require.NoError(t, err)

	var failover string

	_, err = cfg.Get(config.NewKeyPath("replication/failover"), &failover)
	require.NoError(t, err)
	assert.Equal(t, "manual", failover)
}

func TestBuild_Env_SchemaAware_IprotoListen(t *testing.T) {
	t.Setenv("TT_IPROTO_LISTEN", "3301")

	ctx := context.Background()

	cfg, err := tarantool.New().
		WithSchemaFile(fixtureSchemaPath).
		Build(ctx)
	require.NoError(t, err)

	var listen string

	_, err = cfg.Get(config.NewKeyPath("iproto/listen"), &listen)
	require.NoError(t, err)
	assert.Equal(t, "3301", listen)
}

func TestBuild_Env_SchemaAware_UnknownSkipped(t *testing.T) {
	t.Setenv("TT_UNKNOWN_THING", "x")

	ctx := context.Background()

	cfg, err := tarantool.New().
		WithSchemaFile(fixtureSchemaPath).
		Build(ctx)
	require.NoError(t, err)

	_, ok := cfg.Lookup(config.NewKeyPath("unknown/thing"))
	assert.False(t, ok, "unknown env var should not be applied")

	_, ok = cfg.Lookup(config.NewKeyPath("unknown_thing"))
	assert.False(t, ok, "unknown env var should not be applied as a single segment either")
}

func TestBuild_Env_SchemaAware_DefaultSuffix(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	writeFile(t, cfgPath, "audit_log:\n  nonblock: from-file\n")

	t.Setenv("TT_AUDIT_LOG_NONBLOCK_DEFAULT", "from-default-env")

	ctx := context.Background()

	cfg, err := tarantool.New().
		WithConfigFile(cfgPath).
		WithSchemaFile(fixtureSchemaPath).
		Build(ctx)
	require.NoError(t, err)

	var nonblock string

	_, err = cfg.Get(config.NewKeyPath("audit_log/nonblock"), &nonblock)
	require.NoError(t, err)
	assert.Equal(t, "from-file", nonblock, "file should override default-env")
}

func TestBuild_Env_NoSchema_HeuristicUnchanged(t *testing.T) {
	t.Setenv("TT_AUDIT_LOG_NONBLOCK", "true")

	ctx := context.Background()

	cfg, err := tarantool.New().
		WithoutSchema().
		Build(ctx)
	require.NoError(t, err)

	var nonblock string

	_, err = cfg.Get(config.NewKeyPath("audit/log/nonblock"), &nonblock)
	require.NoError(t, err)
	assert.Equal(t, "true", nonblock,
		"without schema, naive split sends the value to the wrong path")
}

func TestBuild_NullCoercion_EmptyStringField(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	// wal_queue_max_size is a string field in the fixture schema, set to an
	// explicit null in the config.
	writeFile(t, cfgPath, "wal_queue_max_size: ~\n")

	ctx := context.Background()

	// Default policy (NullLeave) keeps the null, which the string schema rejects.
	_, err := tarantool.New().
		WithConfigFile(cfgPath).
		WithSchemaFile(fixtureSchemaPath).
		WithEnvPrefix("TT_TESTONLY_").
		Build(ctx)
	require.Error(t, err)

	// WithNullCoercion(NullZero) coerces the null to "", so it validates.
	cfg, err := tarantool.New().
		WithConfigFile(cfgPath).
		WithSchemaFile(fixtureSchemaPath).
		WithEnvPrefix("TT_TESTONLY_").
		WithNullCoercion(jsonschema.NullZero).
		Build(ctx)
	require.NoError(t, err)

	var size string

	_, err = cfg.Get(config.NewKeyPath("wal_queue_max_size"), &size)
	require.NoError(t, err)
	assert.Empty(t, size)
}

// Tarantool's YAML decoder reads an empty value (`key:`) as "", not as null,
// so the builder must accept and reject what Tarantool accepts and rejects.
func TestBuild_EmptyValueReadAsString(t *testing.T) {
	t.Parallel()

	schema := tarantoolScalarSchema(t)

	tests := []struct {
		name    string
		yaml    string
		wantErr string
		// emptySocket asks to check that console.socket holds "", not null.
		emptySocket bool
	}{
		{
			name:    "empty record",
			yaml:    "console:\n",
			wantErr: "console [type] invalid type: Value is string but should be object",
		},
		{
			name: "null record",
			yaml: "console: ~\n",
		},
		{
			name:        "null word record",
			yaml:        "console: null\n",
			wantErr:     "",
			emptySocket: false,
		},
		{
			name:        "empty mapping",
			yaml:        "console: {}\n",
			wantErr:     "",
			emptySocket: false,
		},
		{
			name:        "empty string field",
			yaml:        "console:\n  socket:\n",
			emptySocket: true,
		},
		{
			name:    "empty boolean field",
			yaml:    "console:\n  enabled:\n",
			wantErr: "console/enabled [type] invalid type: Value is string but should be boolean",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			cfgPath := filepath.Join(dir, "config.yaml")
			writeFile(t, cfgPath, tt.yaml)

			cfg, err := tarantool.New().
				WithConfigFile(cfgPath).
				WithSchema(schema).
				WithEnvPrefix("TT_TESTONLY_").
				Build(context.Background())
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)

				return
			}

			require.NoError(t, err)

			if tt.emptySocket {
				var socket any

				_, err = cfg.Get(config.NewKeyPath("console/socket"), &socket)
				require.NoError(t, err)
				assert.IsType(t, "", socket, "the empty value is a string, not null")
				assert.Empty(t, socket)
			}
		})
	}
}

func TestBuild_WithoutValidation_KeepsSchemaAwareEnvRouting(t *testing.T) {
	schema := []byte(`{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"type": "object",
		"properties": {
			"audit_log": {
				"type": "object",
				"properties": {
					"nonblock": { "type": "boolean" }
				}
			}
		},
		"additionalProperties": false
	}`)

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	writeFile(t, cfgPath, "audit_log:\n  nonblock: not-a-bool\n")

	t.Setenv("TT_AUDIT_LOG_NONBLOCK", "still-not-a-bool")

	ctx := context.Background()

	cfg, err := tarantool.New().
		WithConfigFile(cfgPath).
		WithSchema(schema).
		WithoutValidation().
		Build(ctx)
	require.NoError(t, err, "validation must be skipped despite invalid types")

	var nonblock string

	_, err = cfg.Get(config.NewKeyPath("audit_log/nonblock"), &nonblock)
	require.NoError(t, err)
	assert.Equal(t, "still-not-a-bool", nonblock,
		"env var must be routed via the schema trie, not the naive split")
}

func TestBuild_WithoutValidation_SchemaStillRequired(t *testing.T) {
	t.Setenv("TT_AUDIT_LOG_NONBLOCK", "true")

	ctx := context.Background()

	cfg, err := tarantool.New().
		WithSchemaFile(fixtureSchemaPath).
		WithoutValidation().
		Build(ctx)
	require.NoError(t, err)

	var nonblock string

	_, err = cfg.Get(config.NewKeyPath("audit_log/nonblock"), &nonblock)
	require.NoError(t, err)
	assert.Equal(t, "true", nonblock,
		"WithoutValidation alone keeps schema-aware env routing intact")
}

func TestBuild_Env_SchemaAware_Wildcard(t *testing.T) {
	t.Setenv("TT_GROUPS_FOO_REPLICASETS_BAR_INSTANCES_BAZ_IPROTO_LISTEN", "3302")

	ctx := context.Background()

	cfg, err := tarantool.New().
		WithSchemaFile(fixtureSchemaPath).
		Build(ctx)
	require.NoError(t, err)

	var listen string

	_, err = cfg.Get(config.NewKeyPath(
		"groups/foo/replicasets/bar/instances/baz/iproto/listen"), &listen)
	require.NoError(t, err)
	assert.Equal(t, "3302", listen)
}
