package schema_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/tarantool/go-config/v2/syntax/internal/path"
	"github.com/tarantool/go-config/v2/syntax/internal/schema"
)

func TestMetadata(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name, source string
		path         []path.Step
		want         *schema.Metadata
	}{
		{
			name: "field metadata",
			source: `{"type":"object","description":"Parent.","properties":{"mode":{
			  "type":"string","description":"Application mode.","enum":["dev","prod"],"default":"prod"
			}}}`,
			path: []path.Step{path.Property("mode")},
			want: &schema.Metadata{
				Description: "Application mode.",
				Types:       []string{"string"},
				Enum:        []any{"dev", "prod"},
				Default:     "prod",
			},
		},
		{
			name:   "parent without children",
			source: `{"type":"object","description":"Parent.","properties":{"child":{"description":"Child."}}}`,
			want:   &schema.Metadata{Description: "Parent.", Types: []string{"object"}},
		},
		{
			name:   "metadata without description",
			source: `{"type":["string","null"],"enum":["null",null],"default":null,"deprecated":true}`,
			want: &schema.Metadata{
				Types:      []string{"string", "null"},
				Enum:       []any{"null", nil},
				Deprecated: true,
			},
		},
		{
			name: "anyOf descriptions",
			source: `{"description":"Address.","anyOf":[
			  {"type":"integer","description":"TCP port.","default":3301},
			  {"type":"string","description":"Unix socket.","default":"/tmp/app.sock"}
			]}`,
			want: &schema.Metadata{
				Description: "Address.",
				AnyOf: []schema.Metadata{
					{Description: "TCP port.", Types: []string{"integer"}, Default: json.Number("3301")},
					{Description: "Unix socket.", Types: []string{"string"}, Default: "/tmp/app.sock"},
				},
			},
		},
		{
			name:   "anyOf empty alternative",
			source: `{"anyOf":[{}, {"type":"string"}]}`,
			want:   &schema.Metadata{AnyOf: []schema.Metadata{{}, {Types: []string{"string"}}}},
		},
		{
			name:   "anyOf true alternative",
			source: `{"anyOf":[true, {"type":"string"}]}`,
			want:   &schema.Metadata{AnyOf: []schema.Metadata{{}, {Types: []string{"string"}}}},
		},
		{
			name:   "oneOf constraint without annotations",
			source: `{"oneOf":[{"minimum":10}, {"enum":[1,2]}]}`,
			want: &schema.Metadata{OneOf: []schema.Metadata{
				{}, {Enum: []any{json.Number("1"), json.Number("2")}},
			}},
		},
		{
			name:   "empty alternative above field",
			source: `{"anyOf":[{}, {"properties":{"field":{"type":"string"}}}]}`,
			path:   []path.Step{path.Property("field")},
			want:   &schema.Metadata{AnyOf: []schema.Metadata{{}, {Types: []string{"string"}}}},
		},
		{
			name:   "empty alternative behind reference",
			source: `{"$defs":{"empty":{}},"anyOf":[{"$ref":"#/$defs/empty"}, {"type":"string"}]}`,
			want:   &schema.Metadata{AnyOf: []schema.Metadata{{}, {Types: []string{"string"}}}},
		},
		{
			name:   "cyclic alternative",
			source: `{"anyOf":[{"$ref":"#"}, {"type":"string"}]}`,
			want:   &schema.Metadata{AnyOf: []schema.Metadata{{}, {Types: []string{"string"}}}},
		},
		{
			name:   "false alternative omitted",
			source: `{"anyOf":[false, {"type":"string"}]}`,
			want:   &schema.Metadata{AnyOf: []schema.Metadata{{Types: []string{"string"}}}},
		},
		{
			name:   "nested alternative without annotations",
			source: `{"anyOf":[{"allOf":[{},true]}, {"type":"string"}]}`,
			want:   &schema.Metadata{AnyOf: []schema.Metadata{{}, {Types: []string{"string"}}}},
		},
		{
			name:   "documented field with empty alternatives",
			source: `{"description":"Field.","anyOf":[{},true]}`,
			want:   &schema.Metadata{Description: "Field.", AnyOf: []schema.Metadata{{}, {}}},
		},
		{
			name:   "allOf omits missing annotations",
			source: `{"allOf":[{},true,{"type":"string"}]}`,
			want:   &schema.Metadata{AllOf: []schema.Metadata{{Types: []string{"string"}}}},
		},
		{
			name: "oneOf object alternatives",
			source: `{"oneOf":[
			  {"type":"object","description":"TCP.","properties":{"port":{"description":"Child."}}},
			  {"type":"object","description":"Unix."}
			]}`,
			want: &schema.Metadata{
				OneOf: []schema.Metadata{
					{Description: "TCP.", Types: []string{"object"}},
					{Description: "Unix.", Types: []string{"object"}},
				},
			},
		},
		{
			name:   "allOf keeps intersecting constraints",
			source: `{"allOf":[{"enum":[1,2]},{"enum":[2,3]}]}`,
			want: &schema.Metadata{
				AllOf: []schema.Metadata{
					{Enum: []any{json.Number("1"), json.Number("2")}},
					{Enum: []any{json.Number("2"), json.Number("3")}},
				},
			},
		},
		{
			name: "alternatives above field",
			source: `{"description":"Parent.","anyOf":[
			  {"properties":{"field":{"type":"integer","description":"Number."}}},
			  {"properties":{"field":{"type":"string","description":"Text."}}}
			]}`,
			path: []path.Step{path.Property("field")},
			want: &schema.Metadata{
				AnyOf: []schema.Metadata{
					{Description: "Number.", Types: []string{"integer"}},
					{Description: "Text.", Types: []string{"string"}},
				},
			},
		},
		{
			name: "shared references retain each alternative",
			source: `{"$defs":{"value":{"type":"string","description":"Shared."}},"oneOf":[
			  {"$ref":"#/$defs/value","enum":["first"]},
			  {"$ref":"#/$defs/value","enum":["second"]}
			]}`,
			want: &schema.Metadata{
				OneOf: []schema.Metadata{
					{
						Enum:  []any{"first"},
						AllOf: []schema.Metadata{{Description: "Shared.", Types: []string{"string"}}},
					},
					{
						Enum:  []any{"second"},
						AllOf: []schema.Metadata{{Description: "Shared.", Types: []string{"string"}}},
					},
				},
			},
		},
		{
			name: "reference metadata",
			source: `{"$defs":{"mode":{"type":"string","description":"Mode.","default":"dev"}},
			  "properties":{"mode":{"$ref":"#/$defs/mode"}}}`,
			path: []path.Step{path.Property("mode")},
			want: &schema.Metadata{Description: "Mode.", Types: []string{"string"}, Default: "dev"},
		},
		{
			name: "draft7 ignores reference siblings",
			source: `{"$schema":"http://json-schema.org/draft-07/schema#",
			  "definitions":{"mode":{"description":"Mode."}},
			  "$ref":"#/definitions/mode","description":"Ignored.","default":0}`,
			want: &schema.Metadata{Description: "Mode."},
		},
		{
			name:   "modern reference siblings",
			source: `{"$defs":{"mode":{"description":"Mode."}},"$ref":"#/$defs/mode","description":"Local."}`,
			want: &schema.Metadata{
				Description: "Local.",
				AllOf:       []schema.Metadata{{Description: "Mode."}},
			},
		},
		{
			name:   "reference cycle",
			source: `{"description":"Recursive.","$ref":"#"}`,
			want:   &schema.Metadata{Description: "Recursive."},
		},
		{
			name: "recursive path",
			source: `{"$defs":{"node":{"description":"Node.","properties":{"next":{"$ref":"#/$defs/node"}}}},
			  "$ref":"#/$defs/node"}`,
			path: []path.Step{path.Property("next"), path.Property("next")},
			want: &schema.Metadata{Description: "Node."},
		},
		{name: "false default", source: `{"default":false}`, want: &schema.Metadata{Default: false}},
		{name: "zero default", source: `{"default":0}`, want: &schema.Metadata{Default: json.Number("0")}},
		{name: "empty default", source: `{"default":""}`, want: &schema.Metadata{Default: ""}},
		{name: "null default", source: `{"default":null}`, want: nil},
		{
			name:   "compound defaults",
			source: `{"anyOf":[{"default":{"host":"localhost","port":3301}},{"default":[1,"two",null]}]}`,
			want: &schema.Metadata{
				AnyOf: []schema.Metadata{
					{Default: map[string]any{"host": "localhost", "port": json.Number("3301")}},
					{Default: []any{json.Number("1"), "two", nil}},
				},
			},
		},
		{
			name:   "null default behind reference",
			source: `{"$defs":{"value":{"default":null}},"$ref":"#/$defs/value"}`,
			want:   nil,
		},
		{
			name:   "null legacy tuple default",
			source: `{"$schema":"http://json-schema.org/draft-07/schema#", "type":"array", "items":[{"default":null}]}`,
			path:   []path.Step{path.Index(0)}, want: nil,
		},
		{
			name: "exact number", source: `{"default":9007199254740993}`,
			want: &schema.Metadata{Default: json.Number("9007199254740993")},
		},
		{name: "null constant", source: `{"const":null}`, want: &schema.Metadata{HasConst: true}},
		{
			name: "verbatim description", source: `{"description":"**Keep** [link](https://example.org)."}`,
			want: &schema.Metadata{Description: "**Keep** [link](https://example.org)."},
		},
		{
			name: "backticks in enum", source: "{\"enum\":[\"a``b\"]}", want: &schema.Metadata{Enum: []any{"a``b"}},
		},
		{
			name: "newline in default", source: `{"default":"one\ntwo"}`,
			want: &schema.Metadata{Default: "one\ntwo"},
		},
		{
			name: "title", source: `{"title":"Mode","description":"Mode"}`,
			want: &schema.Metadata{Title: "Mode", Description: "Mode"},
		},
		{
			name: "unknown path", source: `{"description":"Parent."}`,
			path: []path.Step{path.Property("absent")},
		},
		{
			name: "forbidden path", source: `{"properties":{"value":false}}`,
			path: []path.Step{path.Property("value")},
		},
		{
			name:   "forbidden alternative",
			source: `{"anyOf":[{"description":"Forbidden.","allOf":[false]},{"description":"Allowed."}]}`,
			want:   &schema.Metadata{AnyOf: []schema.Metadata{{Description: "Allowed."}}},
		},
		{
			name: "alternative with incompatible ancestor type",
			source: `{"anyOf":[
			  {"type":"object","properties":{"value":{"description":"Object field."}}},
			  {"type":"string","properties":{"value":{"description":"Not an object."}}}
			]}`,
			path: []path.Step{path.Property("value")},
			want: &schema.Metadata{AnyOf: []schema.Metadata{{Description: "Object field."}}},
		},
		{
			name: "nested composition",
			source: `{"allOf":[
			  {"properties":{"field":{"anyOf":[{"type":"integer"},{"type":"string"}]}}},
			  {"properties":{"field":{"enum":[1,"auto"]}}}
			]}`,
			path: []path.Step{path.Property("field")},
			want: &schema.Metadata{
				AllOf: []schema.Metadata{
					{AnyOf: []schema.Metadata{{Types: []string{"integer"}}, {Types: []string{"string"}}}},
					{Enum: []any{json.Number("1"), "auto"}},
				},
			},
		},
		{
			name: "overlapping property patterns",
			source: `{"properties":{"field":{"description":"Field."}},
			  "patternProperties":{"^f":{"description":"Pattern."}}}`,
			path: []path.Step{path.Property("field")},
			want: &schema.Metadata{
				AllOf: []schema.Metadata{{Description: "Field."}, {Description: "Pattern."}},
			},
		},
		{name: "undocumented", source: `{}`},
		{name: "undocumented alternatives", source: `{"anyOf":[{},true]}`},
		{name: "undocumented nested composition", source: `{"allOf":[{"oneOf":[{"minimum":10},{}]},true]}`},
		{name: "undocumented reference cycle", source: `{"anyOf":[{"$ref":"#"},{}]}`},
		{name: "blank description", source: `{"description":"  "}`},
		{name: "true schema", source: `true`},
		{name: "false schema", source: `false`},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			compiled, err := schema.Compile([]byte(test.source))
			if err != nil {
				t.Fatal(err)
			}

			if got := compiled.MetadataAt(test.path); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("Metadata =\n%+v\nwant:\n%+v", got, test.want)
			}
		})
	}

	var missing *schema.Schema

	if got := missing.MetadataAt(nil); got != nil {
		t.Errorf("nil schema metadata = %+v", got)
	}
}

func TestMetadataOwnership(t *testing.T) {
	t.Parallel()

	compiled, err := schema.Compile([]byte(`{
	  "type":"object", "enum":[{"nested":["original"]}],
	  "const":{"nested":["original"]}, "default":{"nested":["original"]},
	  "anyOf":[{"type":"object","default":{"nested":["original"]}}]
	}`))
	if err != nil {
		t.Fatal(err)
	}

	original := compiled.MetadataAt(nil)
	modified := compiled.MetadataAt(nil)

	modified.Types[0] = schema.TypeString
	modified.AnyOf[0].Types[0] = schema.TypeString

	for _, value := range []any{modified.Enum[0], modified.Const, modified.Default, modified.AnyOf[0].Default} {
		object, ok := value.(map[string]any)
		if !ok {
			t.Fatalf("expected an object, got %T", value)
		}

		items, ok := object["nested"].([]any)
		if !ok {
			t.Fatalf("expected a nested array, got %T", object["nested"])
		}

		items[0] = "changed"
	}

	if got := compiled.MetadataAt(nil); !reflect.DeepEqual(got, original) {
		t.Fatalf("modifying metadata changed the schema: %+v", got)
	}
}
