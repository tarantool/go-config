package jsonschema_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tarantool/go-config/v2/collectors"
	"github.com/tarantool/go-config/v2/keypath"
	"github.com/tarantool/go-config/v2/tarantool"
	"github.com/tarantool/go-config/v2/tree"
	"github.com/tarantool/go-config/v2/validator"
	"github.com/tarantool/go-config/v2/validators/jsonschema"
)

func TestValidateAlternativeErrors(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name, schema string
		value        any
		want         []string
	}{
		{
			name:   "anyOf number",
			schema: `{"anyOf":[{"type":"number"},{"type":"string"}]}`,
			value:  0,
		},
		{
			name:   "anyOf string",
			schema: `{"anyOf":[{"type":"number"},{"type":"string"}]}`,
			value:  "fatal",
		},
		{
			name:   "anyOf enum fails",
			schema: `{"anyOf":[{"type":"number"},{"type":"string"}],"enum":[0,"fatal"]}`,
			value:  8,
			want:   []string{"value enum"},
		},
		{
			name:   "anyOf no match",
			schema: `{"anyOf":[{"type":"number"},{"type":"string"}]}`,
			value:  true,
			want:   []string{"value anyOf", "value type", "value type"},
		},
		{
			name:   "oneOf match",
			schema: `{"oneOf":[{"type":"number"},{"type":"string"}]}`,
			value:  0,
		},
		{
			name:   "oneOf enum fails",
			schema: `{"oneOf":[{"type":"number"},{"type":"string"}],"enum":[0]}`,
			value:  8,
			want:   []string{"value enum"},
		},
		{
			name:   "oneOf no match",
			schema: `{"oneOf":[{"type":"number"},{"type":"string"}]}`,
			value:  true,
			want:   []string{"value oneOf", "value type", "value type"},
		},
		{
			name:   "oneOf multiple matches",
			schema: `{"oneOf":[{"type":"number"},{"type":"integer"},{"type":"string"}]}`,
			value:  0,
			want:   []string{"value oneOf"},
		},
		{
			name:   "not passes",
			schema: `{"not":{"type":"string"}}`,
			value:  0,
		},
		{
			name:   "not fails",
			schema: `{"not":{"type":"string"}}`,
			value:  "fatal",
			want:   []string{"value not"},
		},
		{
			name:   "not passes enum fails",
			schema: `{"not":{"type":"string"},"enum":[1]}`,
			value:  0,
			want:   []string{"value enum"},
		},
		{
			name:   "then passes",
			schema: `{"if":{"type":"number"},"then":{"minimum":0},"else":{"type":"string"}}`,
			value:  0,
		},
		{
			name:   "then fails",
			schema: `{"if":{"type":"number"},"then":{"minimum":0},"else":{"type":"string"}}`,
			value:  -1,
			want:   []string{"value minimum"},
		},
		{
			name:   "else passes",
			schema: `{"if":{"type":"number"},"then":{"minimum":0},"else":{"type":"string"}}`,
			value:  "fatal",
		},
		{
			name:   "else fails",
			schema: `{"if":{"type":"number"},"then":{"minimum":0},"else":{"type":"string"}}`,
			value:  true,
			want:   []string{"value type"},
		},
		{
			name:   "else passes enum fails",
			schema: `{"if":{"type":"number"},"then":{"minimum":0},"else":{"type":"string"},"enum":[0]}`,
			value:  "fatal",
			want:   []string{"value enum"},
		},
		{
			name:   "allOf fails",
			schema: `{"allOf":[{"type":"integer"},{"minimum":1}]}`,
			value:  0,
			want:   []string{"value minimum"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			schema := fmt.Sprintf(
				`{"type":"object","properties":{"value":%s,"broken":{"type":"integer"}}}`,
				tt.schema,
			)
			schemaValidator, err := jsonschema.New([]byte(schema))
			require.NoError(t, err)

			root := tree.New()
			root.Set(keypath.KeyPath{"value"}, tt.value)
			root.Get(keypath.KeyPath{"value"}).SetTypeFixed(true)
			root.Set(keypath.KeyPath{"broken"}, "bad")
			root.Get(keypath.KeyPath{"broken"}).SetTypeFixed(true)

			errs := schemaValidator.Validate(root)
			got := make([]string, 0, len(errs))

			for _, err := range errs {
				got = append(got, err.Path.String()+" "+err.Code)
			}

			want := append([]string{"broken type"}, tt.want...)
			require.ElementsMatch(t, want, got)
		})
	}
}

func TestValidateTarantoolUnionBesideInvalidEndpoints(t *testing.T) {
	t.Parallel()

	schema, err := tarantool.Schema("3.8.1")
	require.NoError(t, err)

	for _, level := range []string{"0", "fatal"} {
		t.Run(level, func(t *testing.T) {
			t.Parallel()

			schemaValidator, err := jsonschema.New(schema)
			require.NoError(t, err)

			text := "log: {level: " + level + ", modules: {test.module: 0}}\n" +
				"replication: {synchro_quorum: 1}\n" +
				"config: {storage: {endpoints: [1, 2, 3]}}"
			format := collectors.NewYamlFormat(collectors.WithTarantoolParserFormatting())
			root, err := format.From(strings.NewReader(text)).Parse()
			require.NoError(t, err)

			errs := schemaValidator.Validate(root)
			require.Len(t, errs, 3)

			for _, err := range errs {
				require.Equal(t, keypath.KeyPath{"config", "storage", "endpoints"}, err.Path[:len(err.Path)-1])
				require.Equal(t, "type", err.Code)
				require.Equal(t, validator.RangeFromTree(root.Get(err.Path).Range), err.Range)
			}
		})
	}
}
