package schema //nolint:testpackage // Tests also check the compiled schema identity.

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/kaptinlin/jsonschema"
	"github.com/tarantool/go-config/v2/syntax/internal/path"
	"github.com/tarantool/go-config/v2/testdata"
)

func TestCompileErrors(t *testing.T) {
	t.Parallel()

	for _, source := range []string{
		`{`, `{"$ref":"#/$defs/missing"}`, `{"properties":{"mode":{"$dynamicRef":"#missing"}}}`,
	} {
		t.Run(source, func(t *testing.T) {
			t.Parallel()

			compiled, err := Compile([]byte(source))
			if err == nil || compiled != nil {
				t.Fatalf("Compile = %v, %v; want no schema and an error", compiled, err)
			}

			if source != "{" && !errors.Is(err, ErrUnresolvedReference) {
				t.Errorf("Compile error = %v, want ErrUnresolvedReference", err)
			}
		})
	}
}

func TestAcceptsAt(t *testing.T) {
	t.Parallel()

	property := []path.Step{path.Property("mode")}
	first := []path.Step{path.Index(0)}
	second := []path.Step{path.Index(1)}

	for _, test := range []struct {
		name, source              string
		path                      []path.Step
		value                     any
		permitsPath, permitsValue bool
	}{
		{"unconstrained", `{}`, property, "prod", true, true},
		{"true schema", `true`, property, "prod", true, true},
		{"false schema", `false`, property, "prod", false, false},
		{"unknown allowed", `{"properties":{"other":false}}`, property, "prod", true, true},
		{"unknown forbidden", `{"additionalProperties":false}`, property, "prod", false, false},
		{"false property", `{"properties":{"mode":false}}`, property, "prod", false, false},
		{"true property", `{"properties":{"mode":true}}`, property, "prod", true, true},
		{"enum match", `{"properties":{"mode":{"enum":["prod"]}}}`, property, "prod", true, true},
		{"enum mismatch", `{"properties":{"mode":{"enum":["prod"]}}}`, property, "dev", true, false},
		{"property on scalar", `{"type":"string"}`, property, "prod", false, false},
		{"index on object", `{"type":"object"}`, first, "prod", false, false},
		{"property on array", `{"type":"array"}`, property, "prod", false, false},
		{"prefix item", `{"prefixItems":[{"const":"prod"}],"items":false}`, first, "prod", true, true},
		{"prefix mismatch", `{"prefixItems":[{"const":"prod"}],"items":false}`, first, "dev", true, false},
		{"extra item forbidden", `{"prefixItems":[{}],"items":false}`, second, "prod", false, false},
		{"extra item allowed", `{"prefixItems":[{}],"items":{"const":"prod"}}`, second, "prod", true, true},
		{"extra item mismatch", `{"prefixItems":[{}],"items":{"const":"prod"}}`, second, "dev", true, false},
		{"property name allowed", `{"propertyNames":{"pattern":"^mo"}}`, property, "prod", true, true},
		{"property name forbidden", `{"propertyNames":{"pattern":"^other"}}`, property, "prod", false, false},
		{"patterns intersect", `{"patternProperties":{"^mo":{"type":"string"},"de$":{"const":"prod"}}}`,
			property, "dev", true, false},
		{"explicit property and pattern", `{
			"properties":{"mode":{"enum":["dev","prod"]}},"patternProperties":{"^mo":{"const":"prod"}}
		}`, property, "dev", true, false},
		{"matching property ignores additional", `{
			"properties":{"mode":{"const":"prod"}},"additionalProperties":false
		}`, property, "prod", true, true},
		{"matching pattern ignores additional", `{
			"patternProperties":{"^mo":{"const":"prod"}},"additionalProperties":false
		}`, property, "prod", true, true},
		{"allOf rejects value", `{
			"allOf":[{"properties":{"mode":{"type":"string"}}},{"properties":{"mode":{"const":"prod"}}}]
		}`, property, "dev", true, false},
		{"allOf rejects path", `{"allOf":[{"properties":{"mode":true}},{"properties":{"mode":false}}]}`,
			property, "prod", false, false},
		{"anyOf accepts alternative", `{
			"anyOf":[{"properties":{"mode":{"const":"dev"}}},{"properties":{"mode":{"const":"prod"}}}]
		}`, property, "prod", true, true},
		{"anyOf rejects value", `{
			"anyOf":[{"properties":{"mode":{"const":"dev"}}},{"properties":{"mode":{"const":"prod"}}}]
		}`, property, "other", true, false},
		{"anyOf rejects path", `{"anyOf":[{"properties":{"mode":false}},{"type":"array"}]}`,
			property, "prod", false, false},
		{"oneOf accepts alternative", `{"oneOf":[{"properties":{"mode":false}},{"properties":{"mode":true}}]}`,
			property, "prod", true, true},
		{"oneOf rejects path", `{"oneOf":[{"properties":{"mode":false}},{"type":"array"}]}`,
			property, "prod", false, false},
		{"leaf oneOf exclusive", `{"properties":{"mode":{"oneOf":[{"type":"string"},{"const":"prod"}]}}}`,
			property, "prod", true, false},
		{"leaf oneOf unique match", `{"properties":{"mode":{"oneOf":[{"type":"string"},{"const":"prod"}]}}}`,
			property, "dev", true, true},
		{"reference rejects path", `{"$ref":"#/$defs/root","$defs":{"root":{"properties":{"mode":false}}}}`,
			property, "prod", false, false},
		{"dynamic reference rejects path", `{
			"$dynamicRef":"#root","$defs":{"root":{"$dynamicAnchor":"root","properties":{"mode":false}}}
		}`, property, "prod", false, false},
		{"cyclic reference preserves sibling constraints", `{
			"$ref":"#","properties":{"mode":{"enum":["prod"]}}
		}`, property, "dev", true, false},
		{"recursive path", `{
			"properties":{"next":{"$ref":"#"},"mode":{"enum":["prod"]}},"additionalProperties":false
		}`, []path.Step{path.Property("next"), path.Property("mode")}, "prod", true, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			compiled := schemaForTest(t, []byte(test.source))
			if got := compiled.AcceptsAt(test.path, test.value, false); got != test.permitsPath {
				t.Errorf("AcceptsAt without value validation = %v, want %v", got, test.permitsPath)
			}

			if got := compiled.AcceptsAt(test.path, test.value, true); got != test.permitsValue {
				t.Errorf("AcceptsAt with value validation = %v, want %v", got, test.permitsValue)
			}
		})
	}
}

func TestObjectAndItemTypes(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		source, itemType string
		object           bool
	}{
		{`{}`, "", false},
		{`{"type":"object"}`, "object", true},
		{`{"properties":{}}`, "", true},
		{`{"patternProperties":{"^x":{}}}`, "", true},
		{`{"additionalProperties":false}`, "", true},
		{`{"type":"string","properties":{}}`, "string", false},
		{`{"type":["string","object"]}`, "", true},
		{`{"anyOf":[{"type":"string"},{"type":"integer"}]}`, "", false},
		{`{"allOf":[{"type":"string"},{"type":"string"}]}`, "string", false},
		{`{"anyOf":[{"type":"string"},{"properties":{}}]}`, "string", true},
		{`{"$ref":"#/$defs/object","$defs":{"object":{"type":"object"}}}`, "object", true},
	} {
		t.Run(test.source, func(t *testing.T) {
			t.Parallel()

			compiled := schemaForTest(t, []byte(test.source))
			if got := compiled.HasObjectSchema(nil); got != test.object {
				t.Errorf("HasObjectSchema = %v, want %v", got, test.object)
			}

			if got := Type(compiled.Lookup(nil)); got != test.itemType {
				t.Errorf("Type = %q, want %q", got, test.itemType)
			}
		})
	}
}

func TestAbsentSchema(t *testing.T) {
	t.Parallel()

	for _, compiled := range []*Schema{nil, {}} {
		steps := []path.Step{path.Property("mode")}
		if got := compiled.Lookup(steps); len(got) != 0 {
			t.Errorf("Lookup on absent schema = %v", got)
		}

		if compiled.HasObjectSchema(steps) {
			t.Error("absent schema reports an object")
		}

		if !compiled.AcceptsAt(steps, "prod", true) {
			t.Error("absent schema should not constrain values")
		}
	}

	if got := Type(nil); got != "" {
		t.Errorf("Type(nil) = %q", got)
	}
}

func TestLookup(t *testing.T) {
	t.Parallel()

	source := testdata.Schema

	tree := schemaForTest(t, source)
	tests := []struct {
		name string
		path []path.Step
		want []string
	}{
		{name: "root", want: []string{"object"}},
		{name: "property", path: []path.Step{path.Property("mode")}, want: []string{"string"}},
		{name: "local reference", path: []path.Step{path.Property("server")}, want: []string{"object"}},
		{
			name: "property behind reference",
			path: []path.Step{path.Property("server"), path.Property("host")},
			want: []string{"string"},
		},
		{name: "array item", path: []path.Step{path.Property("regions"), path.Index(0)}, want: []string{"string"}},
		{
			name: "array item reference",
			path: []path.Step{path.Property("servers"), path.Index(0)},
			want: []string{"object"},
		},
		{
			name: "property behind item reference",
			path: []path.Step{path.Property("servers"), path.Index(0), path.Property("tls")},
			want: []string{"boolean"},
		},
		{name: "unknown root property", path: []path.Step{path.Property("missing")}},
		{name: "array index on object", path: []path.Step{path.Index(0)}},
		{name: "negative array index", path: []path.Step{path.Property("regions"), path.Index(-1)}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tree.Lookup(tt.path)
			types := make([]string, len(got))

			for i, schema := range got {
				types[i] = Type([]*jsonschema.Schema{schema})
			}

			if !slices.Equal(types, tt.want) {
				t.Fatalf("Lookup(%v) types = %v, want %v", tt.path, types, tt.want)
			}
		})
	}

	server := tree.Lookup([]path.Step{path.Property("server")})
	if len(server) != 1 || server[0].ResolvedRef != tree.root.Defs["server"] {
		t.Fatal("server path should preserve the compiled reference and its resolved target")
	}
}

func TestLookupDynamicKeysAndBranches(t *testing.T) {
	t.Parallel()

	const schema = `{
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "groups": {
      "type": "object",
      "additionalProperties": {
        "type": "object",
        "properties": {
          "enabled": {
            "type": "boolean"
          }
        }
      }
    },
    "tuple": {
      "type": "array",
      "prefixItems": [
        {
          "type": "string"
        },
        {
          "type": "integer"
        }
      ],
      "items": {
        "type": "boolean"
      }
    },
    "dynamic": {
      "type": "object",
      "patternProperties": {
        "^x-": {
          "type": "string"
        }
      },
      "additionalProperties": {
        "type": "integer"
      }
    },
    "choice": {
      "anyOf": [
        {
          "type": "array",
          "items": {
            "type": "string"
          }
        },
        {
          "type": "object",
          "additionalProperties": {
            "type": "boolean"
          }
        }
      ]
    },
    "ambiguous": {
      "anyOf": [
        {
          "type": "object",
          "properties": {
            "value": {
              "type": "string"
            }
          }
        },
        {
          "type": "object",
          "properties": {
            "value": {
              "type": "integer"
            }
          }
        }
      ]
    }
  }
}`

	tree := schemaForTest(t, []byte(schema))
	tests := []struct {
		name string
		path []path.Step
		want []string
	}{
		{
			name: "arbitrary group name",
			path: []path.Step{path.Property("groups"), path.Property("north")},
			want: []string{"object"},
		},
		{
			name: "field inside arbitrary group",
			path: []path.Step{path.Property("groups"), path.Property("north"), path.Property("enabled")},
			want: []string{"boolean"},
		},
		{name: "first tuple item", path: []path.Step{path.Property("tuple"), path.Index(0)}, want: []string{"string"}},
		{
			name: "second tuple item",
			path: []path.Step{path.Property("tuple"), path.Index(1)},
			want: []string{"integer"},
		},
		{
			name: "remaining tuple item",
			path: []path.Step{path.Property("tuple"), path.Index(2)},
			want: []string{"boolean"},
		},
		{
			name: "matching pattern property",
			path: []path.Step{path.Property("dynamic"), path.Property("x-name")},
			want: []string{"string"},
		},
		{
			name: "other dynamic property",
			path: []path.Step{path.Property("dynamic"), path.Property("other")},
			want: []string{"integer"},
		},
		{
			name: "array branch of anyOf",
			path: []path.Step{path.Property("choice"), path.Index(0)},
			want: []string{"string"},
		},
		{
			name: "object branch of anyOf",
			path: []path.Step{path.Property("choice"), path.Property("some-key")},
			want: []string{"boolean"},
		},
		{
			name: "two plausible anyOf branches",
			path: []path.Step{path.Property("ambiguous"), path.Property("value")},
			want: []string{"string", "integer"},
		},
		{name: "forbidden property", path: []path.Step{path.Property("other")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tree.Lookup(tt.path)
			types := make([]string, len(got))

			for i, schema := range got {
				types[i] = Type([]*jsonschema.Schema{schema})
			}

			if !slices.Equal(types, tt.want) {
				t.Fatalf("Lookup(%v) types = %v, want %v", tt.path, types, tt.want)
			}
		})
	}

	choice := tree.Lookup([]path.Step{path.Property("choice")})
	if len(choice) != 1 || len(choice[0].AnyOf) != 2 {
		t.Fatal("path ending at anyOf should retain the owning schema and its branches")
	}
}

func TestLookupRecursiveReference(t *testing.T) {
	t.Parallel()

	const schema = `{
  "properties": {
    "root": {
      "$ref": "#/$defs/node"
    }
  },
  "$defs": {
    "node": {
      "type": "object",
      "properties": {
        "next": {
          "$ref": "#/$defs/node"
        }
      }
    }
  }
}`

	tree := schemaForTest(t, []byte(schema))
	got := tree.Lookup([]path.Step{path.Property("root"), path.Property("next"), path.Property("next")})

	if len(got) != 1 || Type(got) != TypeObject {
		t.Fatalf("recursive path resolved to %v, want one object schema", got)
	}
}

func TestReferenceSiblingDialects(t *testing.T) {
	t.Parallel()

	const source = `{
  "$schema":"DIALECT",
  "$ref":"#/definitions/root",
  "additionalProperties":false,
  "definitions":{"root":{"type":"object","properties":{"mode":{"enum":["prod"]}}}}
 }`

	for _, test := range []struct {
		dialect string
		allowed bool
	}{
		{"http://json-schema.org/draft-04/schema#", true},
		{"http://json-schema.org/draft-06/schema#", true},
		{"http://json-schema.org/draft-07/schema#", true},
		{"https://json-schema.org/draft/2019-09/schema", false},
		{"https://json-schema.org/draft/2020-12/schema", false},
	} {
		t.Run(test.dialect, func(t *testing.T) {
			t.Parallel()

			compiled := schemaForTest(t, []byte(strings.ReplaceAll(source, "DIALECT", test.dialect)))

			steps := []path.Step{path.Property("mode")}
			for _, validate := range []bool{false, true} {
				if got := compiled.AcceptsAt(steps, "prod", validate); got != test.allowed {
					t.Errorf("AcceptsAt(validate=%v) = %v, want %v", validate, got, test.allowed)
				}
			}
		})
	}
}

func TestDraft7ReferenceIgnoresSiblingCandidates(t *testing.T) {
	t.Parallel()

	compiled := schemaForTest(t, []byte(`{
  "$schema":"http://json-schema.org/draft-07/schema#",
  "$ref":"#/definitions/value",
  "type":"object",
  "properties":{"fake":{"type":"boolean"}},
  "definitions":{"value":{"type":"string","enum":["prod"]}}
 }`))
	if compiled.HasObjectSchema(nil) {
		t.Error("ignored reference siblings must not select an object context")
	}

	if got := Type(compiled.Lookup(nil)); got != "string" {
		t.Errorf("SchemaType = %q, want string", got)
	}

	if got := compiled.Lookup([]path.Step{path.Property("fake")}); len(got) != 0 {
		t.Errorf("ignored sibling contributes schemas: %+v", got)
	}
}

// schemaForTest compiles a schema without parsing YAML.
func schemaForTest(t *testing.T, source []byte) *Schema {
	t.Helper()

	schema, err := Compile(source)
	if err != nil {
		t.Fatal(err)
	}

	return schema
}
