package envpath_test

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tarantool/go-config/v2"
	"github.com/tarantool/go-config/v2/tarantool/internal/envpath"
)

// typedSchema mirrors the shapes Tarantool's JSON schema generator emits:
// scalars, byte_size as an integer/string type list with a pattern, records
// with additionalProperties: false, maps with additionalProperties alone,
// anyOf unions and arrays of records, plus a $ref.
//
//go:embed testdata/typed.schema.json
var typedSchema []byte

func resolveTyped(t *testing.T, schema []byte, key string) (config.KeyPath, *envpath.Type) {
	t.Helper()

	root, err := envpath.Build(schema)
	require.NoError(t, err)

	path, typ := root.Resolve(key)
	require.NotNil(t, path, key)
	require.NotNil(t, typ, key)

	return path, typ
}

func TestResolve_Type_Plain(t *testing.T) {
	t.Parallel()

	path, typ := resolveTyped(t, typedSchema, "CONSOLE_ENABLED")
	assert.Equal(t, config.NewKeyPath("console/enabled"), path)
	assert.Equal(t, envpath.KindBoolean, typ.Kind)

	_, typ = resolveTyped(t, typedSchema, "CONSOLE_SOCKET")
	assert.Equal(t, envpath.KindString, typ.Kind)

	_, typ = resolveTyped(t, typedSchema, "CONSOLE")
	assert.Equal(t, envpath.KindRecord, typ.Kind)
}

func TestResolve_Type_Compound(t *testing.T) {
	t.Parallel()

	path, typ := resolveTyped(t, typedSchema, "WAL_QUEUE_MAX_SIZE")
	assert.Equal(t, config.NewKeyPathFromSegments([]string{"wal", "queue_max_size"}), path)
	require.Equal(t, envpath.KindUnion, typ.Kind)
	require.Len(t, typ.Variants, 2)
	assert.Equal(t, envpath.KindInteger, typ.Variants[0].Kind)
	assert.Equal(t, envpath.KindString, typ.Variants[1].Kind)

	// The fixture's top-level compound key.
	path, typ = resolveTyped(t, fixtureSchema, "WAL_QUEUE_MAX_SIZE")
	assert.Equal(t, config.NewKeyPathFromSegments([]string{"wal_queue_max_size"}), path)
	assert.Equal(t, envpath.KindString, typ.Kind)
}

func TestResolve_Type_Wildcard(t *testing.T) {
	t.Parallel()

	path, typ := resolveTyped(t, typedSchema, "GROUPS_G_REPLICASETS_R_INSTANCES_I_IPROTO_LISTEN")
	assert.Equal(t, config.NewKeyPath("groups/g/replicasets/r/instances/i/iproto/listen"), path)
	require.Equal(t, envpath.KindArray, typ.Kind, "the $ref is followed")
	assert.Equal(t, envpath.KindRecord, typ.Items.Kind)

	_, typ = resolveTyped(t, typedSchema, "GROUPS_G_REPLICASETS_R_INSTANCES_I_SNAPSHOT_COUNT")
	assert.Equal(t, envpath.KindInteger, typ.Kind)

	_, typ = resolveTyped(t, typedSchema, "GROUPS_G_REPLICASETS_R_INSTANCES_I")
	assert.Equal(t, envpath.KindRecord, typ.Kind, "a wildcard level has the additionalProperties type")

	_, typ = resolveTyped(t, fixtureSchema, "GROUPS_FOO_REPLICASETS_BAR_INSTANCES_BAZ_IPROTO_LISTEN")
	assert.Equal(t, envpath.KindString, typ.Kind)
}

func TestResolve_Type_Map(t *testing.T) {
	t.Parallel()

	_, typ := resolveTyped(t, typedSchema, "LABELS")
	require.Equal(t, envpath.KindMap, typ.Kind)
	assert.Equal(t, envpath.KindString, typ.Values.Kind)

	path, typ := resolveTyped(t, typedSchema, "LABELS_ROLE")
	assert.Equal(t, config.NewKeyPath("labels/role"), path)
	assert.Equal(t, envpath.KindString, typ.Kind, "a map key has the values type")

	_, typ = resolveTyped(t, typedSchema, "ROLES_CFG")
	require.Equal(t, envpath.KindMap, typ.Kind)
	assert.Equal(t, envpath.KindAny, typ.Values.Kind)

	_, typ = resolveTyped(t, typedSchema, "LOG_MODULES")
	require.Equal(t, envpath.KindMap, typ.Kind)
	assert.Equal(t, envpath.KindUnion, typ.Values.Kind)
}

func TestResolve_Type_Union(t *testing.T) {
	t.Parallel()

	_, typ := resolveTyped(t, typedSchema, "LOG_LEVEL")
	require.Equal(t, envpath.KindUnion, typ.Kind)
	require.Len(t, typ.Variants, 2)
	assert.Equal(t, envpath.KindNumber, typ.Variants[0].Kind)
	assert.Equal(t, envpath.KindString, typ.Variants[1].Kind)
	assert.Empty(t, typ.Variants[0].Enum, "the union's enum is not a variant's")
}

func TestResolve_Type_Inferred(t *testing.T) {
	t.Parallel()

	schema := []byte(`{
		"properties": {
			"record": {"properties": {"a": {"type": "string"}}},
			"map": {"additionalProperties": {"type": "integer"}},
			"open": {"type": "object"},
			"closed": {"type": "object", "additionalProperties": false},
			"list": {"items": {"type": "boolean"}},
			"bare": {"type": "array"},
			"enum": {"type": "string", "enum": ["a", "b"]},
			"nullable": {"type": ["string", "null"]},
			"one": {"oneOf": [{"type": "boolean"}]},
			"all": {"allOf": [{"type": "boolean"}]}
		}
	}`)

	for key, want := range map[string]envpath.Kind{
		"RECORD":   envpath.KindRecord,
		"MAP":      envpath.KindMap,
		"OPEN":     envpath.KindMap,
		"CLOSED":   envpath.KindRecord,
		"LIST":     envpath.KindArray,
		"BARE":     envpath.KindArray,
		"ENUM":     envpath.KindString,
		"NULLABLE": envpath.KindUnion,
		"ONE":      envpath.KindBoolean,
		"ALL":      envpath.KindAny,
	} {
		_, typ := resolveTyped(t, schema, key)
		assert.Equal(t, want, typ.Kind, key)
	}

	_, typ := resolveTyped(t, schema, "LIST")
	assert.Equal(t, envpath.KindBoolean, typ.Items.Kind)

	_, typ = resolveTyped(t, schema, "BARE")
	assert.Equal(t, envpath.KindAny, typ.Items.Kind)

	_, typ = resolveTyped(t, schema, "OPEN")
	assert.Equal(t, envpath.KindAny, typ.Values.Kind)

	_, typ = resolveTyped(t, schema, "ENUM")
	assert.Equal(t, []any{"a", "b"}, typ.Enum)
}

func TestResolve_Type_RefCycle(t *testing.T) {
	t.Parallel()

	// An array whose items are the array itself: the type stops where the
	// cycle closes instead of recursing forever.
	schema := []byte(`{
		"$defs": {"nested": {"type": "array", "items": {"$ref": "#/$defs/nested"}}},
		"properties": {"nested": {"$ref": "#/$defs/nested"}}
	}`)

	_, typ := resolveTyped(t, schema, "NESTED")
	require.Equal(t, envpath.KindArray, typ.Kind)
	assert.Equal(t, envpath.KindAny, typ.Items.Kind)
}
