package syntax_test

import (
	"context"
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"

	"github.com/tarantool/go-config/v2/syntax"
	"github.com/tarantool/go-config/v2/testdata"
	"go.yaml.in/yaml/v3"
)

// completionExpectation is one candidate; the table supplies its shared edit range.
type completionExpectation struct {
	label, itemType, insertText string
}

func TestComplete(t *testing.T) {
	t.Parallel()

	completionSchema := testdata.Schema

	var fixture struct {
		Defs json.RawMessage `json:"$defs"`
	}

	err := json.Unmarshal(completionSchema, &fixture)
	if err != nil {
		t.Fatal(err)
	}

	serverKeys := []completionExpectation{
		{"host", "string", "host: "},
		{"port", "integer", "port: "},
		{"tls", "boolean", "tls: "},
	}

	serverRemaining := []completionExpectation{
		{"port", "integer", "port: "},
		{"tls", "boolean", "tls: "},
	}

	flowServerKeys := []completionExpectation{
		{"host", "string", "{host: }"},
		{"port", "integer", "{port: }"},
		{"tls", "boolean", "{tls: }"},
	}

	modes := []completionExpectation{
		{"dev", "string", "dev"},
		{"prod", "string", "prod"},
	}

	tests := []struct {
		name, schema, source string
		start, end           uint32 // Byte columns of the replacement on the cursor line.
		want                 []completionExpectation
		finish               string // Closing delimiters used only when validating unfinished YAML.
		valueLabel           string // Candidate whose decoded document is checked against wantValue.
		wantValue            any
	}{
		// Cursor boundaries and compound values.
		{name: "no scalar continuation", source: "mode: prod\n  ¦"},
		{name: "no insertion before anchor-only value", source: "mode: ¦ &ref"},
		{name: "no insertion before a container value", source: "server: ¦{}"},
		{name: "no insertion before sequence dash", source: "regions:\n  ¦- east"},
		{name: "no insertion before tagged sequence item", source: "regions:\n  - ¦!!str east"},
		{name: "no replacement of occupied object slot", source: "servers: [ ¦{}, {}]"},
		{
			name:   "space before flow comma replaces preceding scalar",
			source: "regions: [east ¦, west]", start: 10, end: 15,
			want: []completionExpectation{{"east", "string", "east"}},
		},
		{
			name:   "space before bare flow key",
			source: "server: {¦ host, tls: true}", start: 9, end: 14,
			want: []completionExpectation{{"host", "string", "host: "}, {"port", "integer", "port: "}},
		},
		{
			name:   "bare flow key counts as existing",
			source: "server: {host, ¦}", start: 15, end: 15,
			want: serverRemaining,
		},
		{
			name: "compound enum values are not scalar suggestions", schema: "compound_values",
			source: "value: ¦", start: 7, end: 7,
			want: []completionExpectation{{"true", "boolean", "true"}},
		},
		{
			name: "compound default without an explicit type", schema: "compound_values",
			source: "val¦", start: 0, end: 3,
			want: []completionExpectation{{"value", "", "value: "}},
		},

		// Keys and containers.
		{
			name:   "root keys",
			source: "¦", start: 0, end: 0,
			want: []completionExpectation{
				{"enabled", "boolean", "enabled: "},
				{"mode", "string", "mode: "},
				{"regions", "array", "regions:\n  "},
				{"server", "object", "server:\n  "},
				{"servers", "array", "servers:\n  "},
			},
		},
		{
			name:   "existing key replaces whole token",
			source: "mo¦de: dev", start: 0, end: 4,
			want: []completionExpectation{{"mode", "string", "mode"}},
		},
		{
			name:   "partial key before CRLF sibling",
			source: "mo¦\r\nenabled: true", start: 0, end: 2,
			want: []completionExpectation{{"mode", "string", "mode: "}},
		},
		{
			name:   "nested object behind ref",
			source: "server:\n  ¦", start: 2, end: 2,
			want: serverKeys,
		},
		{
			name:   "quoted sibling and comment",
			source: "server:\n  'host': localhost\n  # comment\n  ¦", start: 2, end: 2,
			want: serverRemaining,
		},
		{
			name:   "flow containers",
			source: "{¦}", start: 1, end: 1,
			want: []completionExpectation{
				{"enabled", "boolean", "enabled: "},
				{"mode", "string", "mode: "},
				{"regions", "array", "regions: []"},
				{"server", "object", "server: {}"},
				{"servers", "array", "servers: []"},
			},
		},
		{
			name:   "bare flow key",
			source: "server: {ho¦}", start: 9, end: 11,
			want: []completionExpectation{{"host", "string", "host: "}},
		},
		{
			name:   "existing flow key under quoted parent",
			source: "'server': {ho¦: localhost}", start: 11, end: 13,
			want: []completionExpectation{{"host", "string", "host"}},
		},
		{
			name:   "flow siblings",
			source: "server: {host: localhost, ¦}", start: 26, end: 26,
			want: serverRemaining,
		},
		{
			name:   "object at colon",
			source: "server:¦", start: 7, end: 7,
			want: []completionExpectation{
				{"host", "string", "\n  host: "},
				{"port", "integer", "\n  port: "},
				{"tls", "boolean", "\n  tls: "},
			},
		},
		{
			name:   "CRLF container indentation",
			source: "enabled: true\r\nser¦", start: 0, end: 3,
			want: []completionExpectation{
				{"server", "object", "server:\r\n  "},
				{"servers", "array", "servers:\r\n  "},
			},
		},
		{
			name:   "object item at dash",
			source: "servers:\n  -¦", start: 3, end: 3,
			want: []completionExpectation{
				{"host", "string", " host: "},
				{"port", "integer", " port: "},
				{"tls", "boolean", " tls: "},
			},
		},
		{
			name:   "scalar sequence prefix",
			source: "regions:\n  - ea¦", start: 4, end: 6,
			want: []completionExpectation{{"east", "string", "east"}},
		},
		{
			name:   "field inside sequence object",
			source: "servers:\n  - host: localhost\n    tls: ¦", start: 9, end: 9,
			want: []completionExpectation{
				{"false", "boolean", "false"},
				{"true", "boolean", "true"},
			},
		},
		{
			name: "flow tuple index ignores comments", schema: "tuple",
			source: "tuple: [first, # comment\n  ¦]", start: 2, end: 2,
			want: []completionExpectation{{"second", "string", "second"}},
		},
		{
			name: "block tuple index ignores comments", schema: "tuple",
			source: "tuple:\n  - first\n  # comment\n  - ¦", start: 4, end: 4,
			want: []completionExpectation{{"second", "string", "second"}},
		},

		{
			name:   "empty flow object value",
			source: "{server: ¦}", start: 9, end: 9,
			want:       flowServerKeys,
			valueLabel: "host", wantValue: map[string]any{"server": map[string]any{"host": nil}},
		},
		{
			name:   "empty flow object at colon",
			source: "{server:¦}", start: 8, end: 8,
			want: []completionExpectation{
				{"host", "string", " {host: }"},
				{"port", "integer", " {port: }"},
				{"tls", "boolean", " {tls: }"},
			},
			valueLabel: "host", wantValue: map[string]any{"server": map[string]any{"host": nil}},
		},
		{
			name:   "empty flow object item",
			source: "servers: [¦]", start: 10, end: 10,
			want:       flowServerKeys,
			valueLabel: "host", wantValue: map[string]any{"servers": []any{map[string]any{"host": nil}}},
		},
		{
			name:   "later flow object item",
			source: "servers: [{host: existing}, ¦]", start: 28, end: 28,
			want:       flowServerKeys,
			valueLabel: "host",
			wantValue: map[string]any{"servers": []any{
				map[string]any{"host": "existing"}, map[string]any{"host": nil},
			}},
		},

		// Scalar values and YAML quoting.
		{
			name:   "JSON flow key",
			source: `{"mo¦de":"prod"}`, start: 1, end: 7,
			want:       []completionExpectation{{"mode", "string", `"mode"`}},
			valueLabel: "mode", wantValue: map[string]any{"mode": "prod"},
		},
		{
			name:   "JSON nested key",
			source: `server: {"tl¦s":true}`, start: 9, end: 14,
			want:       []completionExpectation{{"tls", "boolean", `"tls"`}},
			valueLabel: "tls", wantValue: map[string]any{"server": map[string]any{"tls": true}},
		},
		{
			name:   "adjacent comment",
			source: "mode: 'prod'¦# comment", start: 6, end: 12,
			want:       []completionExpectation{{"prod", "string", "prod "}},
			valueLabel: "prod", wantValue: map[string]any{"mode": "prod"},
		},
		{
			name:   "separated comment",
			source: "mode: 'prod'¦ # comment", start: 6, end: 12,
			want:       []completionExpectation{{"prod", "string", "prod"}},
			valueLabel: "prod", wantValue: map[string]any{"mode": "prod"},
		},
		{
			name: "enum and default are deduplicated", schema: "defaults",
			source: "mode: ¦", start: 6, end: 6,
			want: modes,
		},
		{
			name:   "boolean at colon",
			source: "enabled:¦", start: 8, end: 8,
			want: []completionExpectation{
				{"false", "boolean", " false"},
				{"true", "boolean", " true"},
			},
		},
		{
			name:   "flow value before closing brace",
			source: "server: {tls: tr¦}", start: 14, end: 16,
			want: []completionExpectation{{"true", "boolean", "true"}},
		},
		{
			name:   "value before existing scalar",
			source: "mode: ¦prod", start: 6, end: 10,
			want: modes,
		},
		{
			name:   "plain value trailing space",
			source: "mode: prod ¦", start: 6, end: 11,
			want: []completionExpectation{{"prod", "string", "prod"}},
		},
		{
			name: "UTF-8 byte range", schema: "unicode",
			source: "режим: пр¦", start: 12, end: 16,
			want: []completionExpectation{{"прод🚀", "string", "\"прод\\U0001F680\""}},
		},
		{
			name: "double quoted escape", schema: "unicode",
			source: "mode: \"\\u0070r¦\"", start: 6, end: 15,
			want: []completionExpectation{{"prod", "string", "prod"}},
		},
		{
			name: "single quoted escape", schema: "unicode",
			source: "mode: 'it''¦'", start: 6, end: 12,
			want: []completionExpectation{{"it's", "string", "it's"}},
		},
		{
			name:   "anchor",
			source: "mode: &mode pr¦", start: 12, end: 14,
			want: []completionExpectation{{"prod", "string", "prod"}},
		},
		{
			name: "scalar types and flow quoting", schema: "scalar_types",
			source: "[¦]", start: 1, end: 1,
			want: []completionExpectation{
				{"1", "number", "1"},
				{"1", "string", "\"1\""},
				{"9007199254740993", "number", "9007199254740993"},
				{"a,b", "string", "\"a,b\""},
				{"line\nbreak", "string", "\"line\\nbreak\""},
				{"null", "null", "null"},
				{"null", "string", "\"null\""},
				{"true", "boolean", "true"},
				{"true", "string", "\"true\""},
				{"x]y", "string", "\"x]y\""},
			},
		},
		{
			name: "property names requiring quotes", schema: "unsafe_keys",
			source: "{¦}", start: 1, end: 1,
			want: []completionExpectation{
				{"", "string", "\"\": "},
				{"#key", "string", "'#key': "},
				{"a,b", "string", "\"a,b\": "},
				{"a: b", "string", "'a: b': "},
				{"true", "string", "\"true\": "},
			},
		},

		// schema constraints.
		{
			name: "numeric const", schema: "numeric_const",
			source: "value: ¦", start: 7, end: 7,
			want: []completionExpectation{{"42", "number", "42"}},
		},
		{
			name: "enum const and invalid default", schema: "enum_and_const",
			source: "value: ¦", start: 7, end: 7,
			want: []completionExpectation{{"prod", "string", "prod"}},
		},
		{
			name: "parent type constrains anyOf", schema: "parent_constrains_anyOf",
			source: "value: ¦", start: 7, end: 7,
			want: []completionExpectation{{"true", "boolean", "true"}},
		},
		{
			name: "allOf at value", schema: "allOf",
			source: "value: ¦", start: 7, end: 7,
			want: []completionExpectation{{"prod", "string", "prod"}},
		},
		{
			name: "oneOf exclusivity", schema: "oneOf_exclusivity",
			source: "value: ¦", start: 7, end: 7,
			want: []completionExpectation{{"dev", "string", "dev"}},
		},
		{
			name: "ref sibling constraints", schema: "ref_siblings",
			source: "value: ¦", start: 7, end: 7,
			want: []completionExpectation{{"prod", "string", "prod"}},
		},
		{
			name: "property and overlapping patterns", schema: "property_and_pattern",
			source: "value: ¦", start: 7, end: 7,
			want: []completionExpectation{{"prod", "string", "prod"}},
		},
		{
			name: "allOf above path", schema: "allOf_above_path",
			source: "value: ¦", start: 7, end: 7,
			want: []completionExpectation{{"prod", "string", "prod"}},
		},
		{
			name: "anyOf above path", schema: "anyOf_above_path",
			source: "value: ¦", start: 7, end: 7,
			want: modes,
		},
		{
			name: "nullable boolean", schema: "nullable_boolean",
			source: "value: ¦", start: 7, end: 7,
			want: []completionExpectation{
				{"false", "boolean", "false"},
				{"null", "null", "null"},
				{"true", "boolean", "true"},
			},
		},
		{
			name: "scalar root", schema: "root_scalar",
			source: "pr¦", start: 0, end: 2,
			want: []completionExpectation{{"prod", "string", "prod"}},
		},
		{
			name: "empty key", schema: "empty_key",
			source: ": pr¦", start: 2, end: 4,
			want: []completionExpectation{{"prod", "string", "prod"}},
		},
		{
			name: "forbidden keys including ref", schema: "forbidden_key",
			source: "¦", start: 0, end: 0,
			want: []completionExpectation{{"allowed", "string", "allowed: "}},
		},
		{
			name: "propertyNames", schema: "propertyNames",
			source: "¦", start: 0, end: 0,
			want: []completionExpectation{{"allowed", "string", "allowed: "}},
		},
		{
			name: "keys from alternative branches", schema: "branches",
			source: "choice:\n  ¦", start: 2, end: 2,
			want: []completionExpectation{
				{"left", "boolean", "left: "},
				{"right", "string", "right: "},
			},
		},
		{
			name: "numeric dynamic name is a property", schema: "branches",
			source: "context:\n  '0':\n    fr¦", start: 4, end: 6,
			want: []completionExpectation{{"from", "string", "from: "}},
		},
		{
			name: "enum under dynamic property", schema: "branches",
			source: "context:\n  api:\n    from: e¦", start: 10, end: 11,
			want: []completionExpectation{{"env", "string", "env"}},
		},

		// Defaults.
		{
			name: "key defaults and types", schema: "defaults",
			source: "¦", start: 0, end: 0,
			want: []completionExpectation{
				{"array", "array", "array:\n  "},
				{"count", "integer", "count: 0"},
				{"disabled", "boolean", "disabled: false"},
				{"empty", "string", "empty: \"\""},
				{"invalid", "", "invalid: "},
				{"mixed", "", "mixed: "},
				{"mode", "string", "mode: prod"},
				{"nested", "object", "nested:\n  "},
				{"object", "object", "object:\n  "},
				{"ratio", "number", "ratio: 1.5"},
				{"referenced", "string", "referenced: dev"},
				{"separator", "string", "separator: \"a,b\""},
				{"text", "string", "text: \"true\""},
			},
		},
		{
			name: "default preserves existing value", schema: "defaults",
			source: "mo¦: dev", start: 0, end: 2,
			want: []completionExpectation{{"mode", "string", "mode"}},
		},
		{
			name: "nested flow default", schema: "defaults",
			source: "nested: {mo¦}", start: 9, end: 11,
			want: []completionExpectation{{"mode", "string", "mode: dev"}},
		},
		{
			name: "zero and empty defaults are candidates", schema: "scalar_defaults",
			source: "value: ¦", start: 7, end: 7,
			want: []completionExpectation{
				{"", "string", "\"\""},
				{"0", "number", "0"},
			},
		},

		// Recovery of incomplete YAML.
		{
			name:   "unclosed flow siblings",
			source: "server: {host: localhost, ¦", start: 26, end: 26,
			want: serverRemaining,
		},
		{
			name:   "unclosed flow existing key",
			source: "server: {ho¦: localhost", start: 9, end: 11,
			want: []completionExpectation{{"host", "string", "host"}},
		},
		{
			name:   "unclosed flow value",
			source: "server: {tls: tr¦", start: 14, end: 16,
			want: []completionExpectation{{"true", "boolean", "true"}},
		},
		{
			name: "unclosed flow tuple index", schema: "tuple",
			source: "tuple: [first, se¦", start: 15, end: 17,
			want: []completionExpectation{{"second", "string", "second"}},
		},
		{
			name:   "unclosed flow in later block item",
			source: "servers:\n  - host: localhost\n  - {tl¦", start: 5, end: 7,
			want: []completionExpectation{{"tls", "boolean", "tls: "}},
		},
		{
			name: "nested unclosed flow keeps sibling scopes separate", schema: "branches",
			source: "context: {api: {from: env}, other: {en¦", start: 36, end: 38,
			want: []completionExpectation{{"enabled", "boolean", "enabled: "}},
		},
		{
			name:   "unclosed quote",
			source: "mode: \"pr¦", start: 6, end: 9,
			want: []completionExpectation{{"prod", "string", "prod"}},
		},
		{
			name:   "new YAML document",
			source: "enabled: true\n---\nmo¦", start: 0, end: 2,
			want: []completionExpectation{{"mode", "string", "mode: "}},
		},

		{
			name:   "unclosed flow in indentless sequence",
			source: "servers:\n- {ho¦", start: 3, end: 5,
			want: []completionExpectation{{"host", "string", "host: "}}, finish: "}",
		},
		{
			name:   "unclosed flow in later indentless item",
			source: "servers:\n- host: localhost\n- {ho¦", start: 3, end: 5,
			want: []completionExpectation{{"host", "string", "host: "}}, finish: "}",
		},
		{
			name:   "unclosed flow after multiline indentless item",
			source: "servers:\n- host: localhost\n  tls: true\n- {ho¦", start: 3, end: 5,
			want: []completionExpectation{{"host", "string", "host: "}}, finish: "}",
		},
		{
			name:   "open object array",
			source: "servers: [¦", start: 10, end: 10,
			want: flowServerKeys, finish: "]",
		},
		{
			name:   "later object in open array",
			source: "servers: [{host: existing}, ¦", start: 28, end: 28,
			want: flowServerKeys, finish: "]",
		},
		{
			name:   "open array after comma",
			source: "servers: [{host: existing},¦", start: 27, end: 27,
			want: flowServerKeys, finish: "]",
		},
		{
			name:   "open scalar array",
			source: "regions: [¦", start: 10, end: 10,
			want: []completionExpectation{
				{"east", "string", "east"},
				{"west", "string", "west"},
			},
			finish: "]",
		},

		// Contexts without suggestions.
		{name: "indented line after closed flow value", source: "server: {tls: true}\n  ¦"},
		{name: "line after nested flow mapping", source: "server:\n  {host: localhost}\n  ¦"},
		{name: "unrecognized line in open flow mapping", source: "server: {tls: true\nmode: pr¦"},
		{name: "unrecognized line with trailing space", source: "server: {tls: true\nmode: pr ¦"},
		{name: "document boundary inside open flow mapping", source: "server: {tls: true\n---\nmo¦"},
		{name: "after closed root mapping", source: "{}¦"},
		{name: "line after closed root mapping", source: "{}\n¦"},
		{name: "object array item without comma", source: "servers: [{host: existing} ¦"},
		{name: "object array item without comma after comment", source: "servers: [{host: existing} # keep\n  ¦"},
		{name: "start of comment text", source: "mode: #¦ keep"},
		{name: "end of comment text", source: "mode: # keep¦"},
		{name: "comment text in flow mapping", source: "server: { #¦ keep\n}"},
		{name: "free string", source: "server:\n  host: lo¦"},
		{name: "unknown schema path", source: "missing: pr¦"},
		{name: "comment in incomplete flow", source: "server: {host: localhost, # tl¦"},
		{name: "block scalar", source: "mode: |\n  pr¦"},
		{name: "multiline plain scalar", source: "mode: pr¦\n  od"},
		{name: "multiline quoted scalar", source: "mode: \"pr¦\n  od\""},
		{name: "alias", source: "mode: &ref prod\nserver: *ref¦"},
		{name: "incomplete escape", source: "mode: \"\\u00¦\""},

		// Whitespace and empty values.
		{
			name:   "value before comment",
			source: "mode: ¦# keep", start: 6, end: 6,
			want: []completionExpectation{
				{"dev", "string", "dev "},
				{"prod", "string", "prod "},
			},
		},
		{
			name:   "value before separated comment",
			source: "mode: ¦ # keep", start: 6, end: 6,
			want: modes,
		},
		{
			name:   "existing value before comment",
			source: "mode: prod ¦# keep", start: 6, end: 11,
			want: []completionExpectation{{"prod", "string", "prod "}},
		},
		{
			name:   "nested key before comment",
			source: "server:\n  ¦# keep", start: 2, end: 2,
			want: serverKeys,
		},
		{
			name:   "root key before comment",
			source: "¦# keep", start: 0, end: 0,
			want: []completionExpectation{
				{"enabled", "boolean", "enabled: "},
				{"mode", "string", "mode: "},
				{"regions", "array", "regions:\n  "},
				{"server", "object", "server:\n  "},
				{"servers", "array", "servers:\n  "},
			},
		},
		{
			name:   "boolean before comment",
			source: "server: {tls: ¦# keep\n}", start: 14, end: 14,
			want: []completionExpectation{
				{"false", "boolean", "false "},
				{"true", "boolean", "true "},
			},
		},
		{
			name:   "flow key before comment",
			source: "server: { ¦# keep\n}", start: 10, end: 10,
			want: serverKeys,
		},
		{
			name:   "flow object before comment",
			source: "servers: [ ¦# keep\n]", start: 11, end: 11,
			want: []completionExpectation{
				{"host", "string", "{host: } "},
				{"port", "integer", "{port: } "},
				{"tls", "boolean", "{tls: } "},
			},
		},
		{
			name:   "explicit object key",
			source: "? server\n:\n  ¦", start: 2, end: 2,
			want: serverKeys,
		},
		{
			name:   "explicit scalar key",
			source: "? mode\n:\n  ¦", start: 2, end: 2,
			want: modes,
		},
		{
			name:   "explicit nested key",
			source: "? server\n:\n  ? tls\n  :\n    ¦", start: 4, end: 4,
			want: []completionExpectation{
				{"false", "boolean", "false"},
				{"true", "boolean", "true"},
			},
		},
		{name: "no completion after flow value on next line", source: "server: {tls: true\n  ¦}"},
		{
			name:   "colon before existing value retains separator",
			source: "mode:¦ prod", start: 5, end: 10,
			want: []completionExpectation{
				{"dev", "string", " dev"},
				{"prod", "string", " prod"},
			},
		},
		{
			name:   "flow item trailing space replaces token",
			source: "regions: [east ¦]", start: 10, end: 15,
			want: []completionExpectation{{"east", "string", "east"}},
		},
		{
			name:   "flow item leading space replaces token",
			source: "regions: [east, ¦ west]", start: 16, end: 21,
			want: []completionExpectation{
				{"east", "string", "east"},
				{"west", "string", "west"},
			},
		},
		{name: "no completion before existing flow pair", source: "server: { ¦ host: localhost}"},
		{name: "no completion before existing block pair", source: "server:\n  host: localhost\n¦  tls: true"},
		{name: "no replacement across anchor", source: "mode:¦ &m prod\nenabled: *m"},
		{
			name:   "blank before first nested sibling",
			source: "server:\n  ¦\n  host: localhost", start: 2, end: 2,
			want: serverRemaining,
		},
		{
			name:   "blank before first root sibling",
			source: "¦\nmode: dev", start: 0, end: 0,
			want: []completionExpectation{
				{"enabled", "boolean", "enabled: "},
				{"regions", "array", "regions:\n  "},
				{"server", "object", "server:\n  "},
				{"servers", "array", "servers:\n  "},
			},
		},
		{name: "no completion at empty item dash indentation", source: "servers:\n  -\n  ¦"},
		{name: "no completion in scalar item continuation", source: "regions:\n  - east\n    ¦"},
		{
			name:   "empty anchored object",
			source: "server: &ref\n  ¦", start: 2, end: 2,
			want: serverKeys,
		},
		{
			name: "quoted prefix retains significant space", schema: "whitespace_prefix",
			source: "value: \"a ¦\"", start: 7, end: 11,
			want: []completionExpectation{
				{"a ", "string", "'a '"},
				{"a b", "string", "a b"},
			},
		},
		{
			name:   "quoted value trailing separator",
			source: "mode: 'prod' ¦", start: 6, end: 13,
			want: []completionExpectation{{"prod", "string", "prod"}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			schema := completionSchema
			if test.schema != "" {
				var err error

				schema, err = json.Marshal(map[string]any{
					"$defs": fixture.Defs, "$ref": "#/$defs/" + test.schema,
				})
				if err != nil {
					t.Fatal(err)
				}
			}

			tree, source, position := completionTree(t, schema, test.source)

			items, err := tree.Completion(position)
			if err != nil {
				t.Fatal(err)
			}

			want := make([]syntax.CompletionItem, len(test.want))
			for index, item := range test.want {
				want[index] = syntax.CompletionItem{
					Label: item.label, Type: item.itemType, InsertText: item.insertText,
					Replace: syntax.Range{
						Start: syntax.Position{Line: position.Line, ByteColumn: test.start},
						End:   syntax.Position{Line: position.Line, ByteColumn: test.end},
					},
				}
			}

			if !slices.Equal(items, want) {
				t.Fatalf("Complete(%q) = %+v, want %+v", test.source, items, want)
			}

			// Apply returned edits as a client would. Valid input must remain valid;
			// unfinished containers can supply their closing delimiters for this check.
			var original any

			validateYAML := yaml.Unmarshal([]byte(source+test.finish), &original) == nil
			for _, item := range items {
				edited := applyCompletion(t, source, position, item)

				if validateYAML || test.wantValue != nil {
					var value any

					err := yaml.Unmarshal([]byte(edited+test.finish), &value)
					if err != nil {
						t.Fatalf("edit %+v produces invalid YAML %q: %v", item, edited+test.finish, err)
					}

					if test.wantValue != nil && item.Label == test.valueLabel &&
						!reflect.DeepEqual(value, test.wantValue) {
						t.Errorf("edit %+v produces %q decoded as %#v; want %#v", item, edited, value, test.wantValue)
					}
				}
			}

			if string(tree.Source()) != source {
				t.Fatal("completion changed the source")
			}
		})
	}
}

//nolint:paralleltest // Subtests share one syntax tree, which must be used serially.
func TestCompleteInvalidPosition(t *testing.T) {
	tree, _, _ := completionTree(t, []byte("{}"), "ключ: 🚀¦")

	for _, test := range []struct {
		name     string
		position syntax.Position
	}{
		{name: "line", position: syntax.Position{Line: 1}},
		{name: "column", position: syntax.Position{ByteColumn: 15}},
		{name: "inside UTF-8", position: syntax.Position{ByteColumn: 1}},
	} {
		t.Run(test.name, func(t *testing.T) {
			items, err := tree.Completion(test.position)
			if err == nil || len(items) != 0 {
				t.Fatalf("Complete(%+v) = %+v, %v; want no items and an error", test.position, items, err)
			}
		})
	}
}

func TestCompleteUnavailableResult(t *testing.T) {
	t.Parallel()

	closed, _, _ := completionTree(t, []byte("{}"), "¦")
	closed.Close()
	closed.Close()

	for name, tree := range map[string]*syntax.Tree{"nil": nil, "zero": {}, "closed": closed} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			items, err := tree.Completion(syntax.Position{})
			if err == nil || len(items) != 0 {
				t.Fatalf("Complete() = %+v, %v; want no items and an error", items, err)
			}
		})
	}
}

func TestCompleteIndependentResults(t *testing.T) {
	t.Parallel()

	// Each tree is used by one goroutine. Only the schema (and its lazy regex
	// caches) is shared, so callers obey the per-parser and per-tree contract.
	for range 8 {
		parser, err := syntax.NewBuilder().
			WithJSONSchema([]byte(`{"properties":{"mode":{"enum":["prod"],"pattern":"^prod$"}}}`)).
			Build(context.Background())
		if err != nil {
			t.Fatal(err)
		}

		trees := make([]*syntax.Tree, 8)
		for index := range trees {
			trees[index], err = parser.Parse(context.Background(), []byte("mode: "))
			if err != nil {
				parser.Close()
				t.Fatal(err)
			}

			t.Cleanup(trees[index].Close)
		}

		parser.Close()

		var workers sync.WaitGroup

		start := make(chan struct{})

		for _, tree := range trees {
			workers.Go(func() {
				<-start

				items, err := tree.Completion(syntax.Position{ByteColumn: 6})
				if err != nil || len(items) != 1 || items[0].Label != "prod" {
					t.Errorf("got %+v, %v", items, err)
				}
			})
		}

		close(start)
		workers.Wait()
	}
}

func FuzzComplete(f *testing.F) {
	schema := testdata.Schema

	for _, marked := range []string{
		"¦", "\r¦", "server: {tls: tr¦", "servers:\n- {ho¦", `{"mo¦de":"prod"}`,
		"mode: 'prod'¦# comment", "server: {tls: true\nmode: pr¦",
		"mode: \"пр🚀¦\"", "enabled: true\r\n¦",
		"servers: [¦", "servers: [{host: existing}, ¦",
		"mode: ¦# comment", "? server\n:\n  ¦",
	} {
		//nolint:gosec // Test fixtures and fuzz inputs fit in uint32.
		f.Add(strings.Replace(marked, "¦", "", 1), uint32(strings.Index(marked, "¦")))
	}

	f.Fuzz(func(t *testing.T, source string, cursor uint32) {
		if len(source) > 4096 || !utf8.ValidString(source) || strings.Contains(source, "¦") {
			t.Skip()
		}

		//nolint:gosec // The fuzz input is limited to 4096 bytes above.
		offset := int(cursor % uint32(len(source)+1))
		for offset > 0 && offset < len(source) && !utf8.RuneStart(source[offset]) {
			offset--
		}

		if offset > 0 && offset < len(source) && source[offset] == '\n' && source[offset-1] == '\r' {
			offset--
		}

		marked := source[:offset] + "¦" + source[offset:]
		tree, _, pos := completionTree(t, schema, marked)

		items, err := tree.Completion(pos)
		if err != nil {
			t.Fatal(err)
		}

		for _, item := range items {
			applyCompletion(t, source, pos, item)
		}

		if string(tree.Source()) != source {
			t.Fatal("completion changed source")
		}
	})
}

func applyCompletion(tb testing.TB, source string, cursor syntax.Position, item syntax.CompletionItem) string {
	tb.Helper()

	rng := item.Replace
	if rng.Start.Line != cursor.Line || rng.End.Line != cursor.Line ||
		rng.Start.ByteColumn > cursor.ByteColumn || rng.End.ByteColumn < cursor.ByteColumn {
		tb.Fatalf("range %+v must be single-line and contain cursor %+v", rng, cursor)
	}

	lines := strings.Split(source, "\n")

	line := lines[cursor.Line]
	if int(cursor.Line)+1 < len(lines) {
		line = strings.TrimSuffix(line, "\r")
	}

	start, end := int(rng.Start.ByteColumn), int(rng.End.ByteColumn)
	if end > len(line) || !utf8.ValidString(line[:start]) || !utf8.ValidString(line[:end]) {
		tb.Fatalf("invalid byte range %+v in %q", rng, line)
	}

	lines[cursor.Line] = lines[cursor.Line][:start] + item.InsertText + lines[cursor.Line][end:]

	return strings.Join(lines, "\n")
}

// completionTree parses a fixture with a cursor marker and registers cleanup.
func completionTree(tb testing.TB, schema []byte, marked string) (*syntax.Tree, string, syntax.Position) {
	tb.Helper()

	offset := strings.Index(marked, "¦")
	if offset < 0 {
		tb.Fatal("missing cursor marker")
	}

	source := strings.Replace(marked, "¦", "", 1)
	position := syntax.Position{
		//nolint:gosec // Test fixtures and fuzz inputs fit in uint32.
		Line: uint32(strings.Count(source[:offset], "\n")),
		//nolint:gosec // Test fixtures and fuzz inputs fit in uint32.
		ByteColumn: uint32(offset - strings.LastIndex(source[:offset], "\n") - 1),
	}

	parser, err := syntax.NewBuilder().WithJSONSchema(schema).Build(context.Background())
	if err != nil {
		tb.Fatal(err)
	}

	tb.Cleanup(parser.Close)

	tree, err := parser.Parse(context.Background(), []byte(source))
	if err != nil {
		tb.Fatal(err)
	}

	tb.Cleanup(tree.Close)

	return tree, source, position
}
