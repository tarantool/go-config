package syntax_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/tarantool/go-config/v2/syntax"
	"github.com/tarantool/go-config/v2/tarantool"
	"github.com/tarantool/go-config/v2/testdata"
)

func TestHover(t *testing.T) {
	t.Parallel()

	config := &syntax.Metadata{Types: []string{"object"}, Description: "Configuration options."}
	etcd := &syntax.Metadata{Types: []string{"object"}, Description: "Etcd connection settings."}
	mode := &syntax.Metadata{Types: []string{"string"}, Description: "Application mode.", Enum: []any{"dev", "prod"}}
	region := &syntax.Metadata{Types: []string{"string"}, Description: "Region name.", Enum: []any{"east", "west"}}
	something := &syntax.Metadata{
		Description: "Select a mode.",
		AnyOf: []syntax.Metadata{
			{Types: []string{"integer"}, Description: "Numeric mode.", Enum: []any{json.Number("1"), json.Number("2")},
				Default: json.Number("1")},
			{Types: []string{"string"}, Description: "Named mode.", Enum: []any{"auto", "off"},
				Default: "auto"},
		},
	}
	choice := &syntax.Metadata{OneOf: []syntax.Metadata{
		{Types: []string{"object"}, Description: "Structured choice."},
		{Types: []string{"string"}, Description: "Text choice."},
	}}

	for _, test := range []struct {
		name, schema, source string
		line, start, end     uint32
		want                 *syntax.Metadata
	}{
		{
			name: "parent key", schema: "nested", source: "con¦fig:\n  etcd:\n    something: 1", end: 6,
			want: config,
		},
		{
			name: "nested parent key", schema: "nested", source: "config:\n  et¦cd:\n    something: 1",
			line: 1, start: 2, end: 6, want: etcd,
		},
		{
			name: "leaf key alternatives", schema: "nested", source: "config:\n  etcd:\n    some¦thing: 1",
			line: 2, start: 4, end: 13, want: something,
		},
		{
			name: "leaf value keeps other alternatives", schema: "nested",
			source: "config:\n  etcd:\n    something: ¦1",
			line:   2, start: 15, end: 16, want: something,
		},
		{
			name: "flow parent", schema: "nested", source: "config: {et¦cd: {something: 1}}",
			start: 9, end: 13, want: etcd,
		},
		{
			name: "bare nested flow key", schema: "nested", source: "config: {et¦cd}",
			start: 9, end: 13, want: etcd,
		},
		{
			name: "flow key with null value", schema: "nested", source: "config: {et¦cd:}",
			start: 9, end: 13, want: etcd,
		},
		{
			name: "bare flow key before another pair", schema: "nested", source: "config: {et¦cd, unknown: 1}",
			start: 9, end: 13, want: etcd,
		},
		{
			name: "bare flow key inside sequence", source: "servers: [{ho¦st}]", start: 11, end: 15,
			want: &syntax.Metadata{Types: []string{"string"}, Description: "Server host."},
		},
		{
			name: "flow leaf", schema: "nested", source: "config: {etcd: {some¦thing: 1}}",
			start: 16, end: 25, want: something,
		},
		{name: "single quoted key", source: "'mo¦de': prod", end: 6, want: mode},
		{name: "escaped key", source: `"\u006do¦de": prod`, end: 11, want: mode},
		{name: "quoted value", source: "mode: 'pr¦od'", start: 6, end: 12, want: mode},
		{name: "invalid value still documented", source: "mode: ¦123", start: 6, end: 9, want: mode},
		{name: "empty container key", schema: "nested", source: "conf¦ig: {}", end: 6, want: config},
		{name: "empty value key", source: "mod¦e:", end: 4, want: mode},
		{name: "bare root key", source: "mod¦e", end: 4, want: mode},
		{
			name: "dynamic object key", schema: "dynamic_properties", source: "groups:\n  tea¦m:\n    enabled: true",
			line: 1, start: 2, end: 6,
			want: &syntax.Metadata{Types: []string{"object"}, Description: "A named group."},
		},
		{
			name: "dynamic field", schema: "dynamic_properties", source: "groups:\n  team:\n    ena¦bled: true",
			line: 2, start: 4, end: 11,
			want: &syntax.Metadata{Types: []string{"boolean"}, Description: "Enable this group."},
		},
		{
			name: "pattern property", schema: "pattern_properties", source: "custom_fo¦o: 1", end: 10,
			want: &syntax.Metadata{Types: []string{"integer"}, Description: "Custom option."},
		},
		{
			name: "array key", source: "reg¦ions: [east]", end: 7,
			want: &syntax.Metadata{Types: []string{"array"}, Description: "Available regions."},
		},
		{name: "block item", source: "regions:\n  - ea¦st", line: 1, start: 4, end: 8, want: region},
		{name: "flow item", source: "regions: [east, we¦st]", start: 16, end: 20, want: region},
		{
			name: "tuple item", schema: "tuple", source: "tuple:\n  - first\n  # comment\n  - se¦cond",
			line: 3, start: 4, end: 10, want: &syntax.Metadata{Description: "Second item.", Enum: []any{"second"}},
		},
		{
			name: "mixed array scalar", schema: "mixed_items", source: "mixed: [tex¦t]", start: 8, end: 12,
			want: &syntax.Metadata{AnyOf: []syntax.Metadata{
				{Types: []string{"object"}, Description: "Structured item."},
				{Types: []string{"string"}, Description: "Text item."},
			}},
		},
		{
			name: "multiline mixed value", schema: "object_or_string", source: "choice:\n  tex¦t",
			line: 1, start: 2, end: 6, want: choice,
		},
		{
			name: "object alternatives on parent", schema: "object_or_string",
			source: "cho¦ice: {}", end: 6, want: choice,
		},
		{
			name: "literal dotted key", schema: "literal_keys", source: "'a¦.b': text", end: 5,
			want: &syntax.Metadata{Types: []string{"string"}, Description: "A dotted property."},
		},
		{
			name: "unicode coordinates", schema: "literal_keys", source: "ключ: '🚀¦text'", start: 10, end: 20,
			want: &syntax.Metadata{Types: []string{"string"}, Description: "Unicode property."},
		},
		{
			name: "numeric property", schema: "literal_keys", source: "¦0: text", end: 1,
			want: &syntax.Metadata{Types: []string{"string"}, Description: "Numeric property name."},
		},
		{
			name: "empty property", schema: "empty_key", source: "'¦': text", end: 2,
			want: &syntax.Metadata{Description: "Empty property name.", Enum: []any{"prod"}},
		},
		{
			name: "CRLF", schema: "nested", source: "config:\r\n  et¦cd:\r\n    something: 1\r\n",
			line: 1, start: 2, end: 6, want: etcd,
		},
		{name: "bare key in unclosed mapping", source: "{mod¦e", start: 1, end: 5, want: mode},
		{name: "key before unfinished value", source: "mod¦e: [", end: 4, want: mode},
		{name: "scalar in unclosed sequence", source: "regions: [ea¦st", start: 10, end: 14, want: region},
		{name: "unrelated syntax error", source: "mod¦e: prod\nbroken: [\n", end: 4, want: mode},
		{
			name: "recovered flow key", schema: "nested", source: "config: {etcd: {some¦thing: 1",
			start: 16, end: 25, want: something,
		},
		{
			name: "recovered flow value", schema: "nested", source: "config: {etcd: {something: a¦u",
			start: 27, end: 29, want: something,
		},
		{name: "later document", source: "mode: dev\n---\nmod¦e: prod", line: 2, end: 4, want: mode},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			parser := newParser(t, test.schema)
			tree, position := markedTree(t, parser, test.source)

			item, err := tree.Hover(position)
			if err != nil {
				t.Fatal(err)
			}

			if item.Metadata == nil {
				t.Fatal("Hover returned no description")
			}

			wantRange := syntax.Range{
				Start: syntax.Position{Line: test.line, ByteColumn: test.start},
				End:   syntax.Position{Line: test.line, ByteColumn: test.end},
			}
			if item.Range != wantRange {
				t.Errorf("range = %+v, want %+v", item.Range, wantRange)
			}

			if !reflect.DeepEqual(item.Metadata, test.want) {
				t.Errorf("metadata = %+v, want %+v", item.Metadata, test.want)
			}
		})
	}
}

func TestHoverWithoutTarget(t *testing.T) {
	t.Parallel()

	parser := newParser(t, "")

	for _, source := range []string{
		"¦", "  ¦  ", "mode: prod\n¦", "mod¦: prod", "unknown: tex¦t",
		"mode¦: prod", "mode:¦ prod", "mode: prod¦", "mode: prod ¦ ",
		"mode: prod ¦# comment", "mode: prod # ¦comment", "¦# comment",
		"server: ¦{}", "server: {¦}", "server: { ¦ }", "regions: [east¦, west]",
		"regions: ¦[east]", "regions:\n  ¦- east", "mode: &an¦chor prod", "mode: *al¦ias",
		"mode: !!st¦r prod", "¦---\nmode: prod", "mode: prod\n¦...",
		"server: {unkn¦own}", "servers: [{unkn¦own}]",
	} {
		t.Run(source, func(t *testing.T) {
			t.Parallel()

			tree, position := markedTree(t, parser, source)

			item, err := tree.Hover(position)
			if err != nil || item != (syntax.HoverItem{}) {
				t.Fatalf("Hover = %+v, %v; want no tooltip", item, err)
			}
		})
	}
}

func TestHoverInvalidPositionAndClosedTree(t *testing.T) {
	t.Parallel()

	parser := newParser(t, "")
	tree, _ := markedTree(t, parser, "ключ: 🚀¦")

	for _, position := range []syntax.Position{
		{Line: 1}, {ByteColumn: 50}, {ByteColumn: 1}, {ByteColumn: 11},
	} {
		_, err := tree.Hover(position)
		if err == nil {
			t.Errorf("Hover(%+v) should reject an invalid position", position)
		}
	}

	tree.Close()

	_, err := tree.Hover(syntax.Position{})
	if err == nil {
		t.Error("Hover should reject a closed tree")
	}

	_, err = (*syntax.Tree)(nil).Hover(syntax.Position{})
	if err == nil {
		t.Error("Hover should reject a nil tree")
	}
}

func TestHoverLifetime(t *testing.T) {
	t.Parallel()

	parser := newParser(t, "")
	tree, position := markedTree(t, parser, "mod¦e: prod")
	parser.Close()

	item, err := tree.Hover(position)
	if err != nil || item.Metadata == nil {
		t.Fatalf("Hover after Parser.Close = %+v, %v", item, err)
	}

	tree.Close()

	if item.Metadata.Description != "Application mode." || item.Range.End.ByteColumn != 4 {
		t.Fatalf("unexpected result after Tree.Close: %+v", item)
	}
}

func TestHoverMultilineScalar(t *testing.T) {
	t.Parallel()

	parser := newParser(t, "")

	for _, source := range []string{
		"mode: |\n  hell¦o", "mode: >\n  hell¦o", "mode: |\r\n  hell¦o",
		"mode: 'hello\n  wor¦ld'", "mode: hello\n  wor¦ld",
	} {
		t.Run(source, func(t *testing.T) {
			t.Parallel()

			tree, position := markedTree(t, parser, source)

			item, err := tree.Hover(position)
			if err != nil || item.Metadata == nil {
				t.Fatalf("Hover = %+v, %v", item, err)
			}

			if item.Metadata.Description != "Application mode." || item.Range.Start.Line != 0 ||
				item.Range.Start.ByteColumn != 6 || item.Range.End.Line != 1 {
				t.Fatalf("unexpected multiline hover: %+v", item)
			}

			checkRange(t, tree.Source(), position, item.Range)
		})
	}
}

func TestHoverTarantool(t *testing.T) {
	t.Parallel()

	source, err := tarantool.Schema("3.7.1")
	if err != nil {
		t.Fatal(err)
	}

	parser, err := syntax.NewBuilder().WithJSONSchema(source).Build(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(parser.Close)

	for _, test := range []struct {
		source, description string
		itemType            string
		defaultValue        any
	}{
		{"con¦fig:\n  etcd:\n    username: user", "centralized configuration", "object", nil},
		{"config:\n  et¦cd:\n    username: user", "connection settings", "object", nil},
		{"replication:\n  fail¦over: off", "failover", "string", "off"},
	} {
		t.Run(test.source, func(t *testing.T) {
			t.Parallel()

			tree, position := markedTree(t, parser, test.source)

			item, err := tree.Hover(position)
			if err != nil || item.Metadata == nil {
				t.Fatalf("Hover = %+v, %v", item, err)
			}

			metadata := item.Metadata
			if !strings.Contains(metadata.Description, test.description) ||
				!reflect.DeepEqual(metadata.Types, []string{test.itemType}) ||
				metadata.Default != test.defaultValue ||
				len(metadata.AllOf)+len(metadata.AnyOf)+len(metadata.OneOf) != 0 {
				t.Errorf("unexpected metadata: %+v", metadata)
			}
		})
	}
}

func FuzzHover(f *testing.F) {
	parser := newParser(f, "")

	for _, marked := range []string{
		"¦", "mod¦e: prod", "config: {etcd: {something: a¦u", "regions: [ea¦st]",
		"mode: 'пр🚀¦'", "mode: prod ¦# comment", "config:\r\n  et¦cd:\r\n",
		"message: |\n  hell¦o\n", "message: >\r\n  hell¦o\r\n", "mode: !!str pr¦od",
		"mixed: [tex¦t]", "choice:\n  tex¦t", "? mode\n: pr¦od", "mode: &ref pr¦od",
	} {
		f.Add(strings.Replace(marked, "¦", "", 1), strings.Index(marked, "¦"))
	}

	f.Fuzz(func(t *testing.T, source string, cursor int) {
		if len(source) > 8192 || !utf8.ValidString(source) || strings.Contains(source, "¦") {
			t.Skip()
		}

		offset := cursor % (len(source) + 1)
		if offset < 0 {
			offset += len(source) + 1
		}

		for offset > 0 && offset < len(source) && !utf8.RuneStart(source[offset]) {
			offset--
		}

		if offset > 0 && offset < len(source) && source[offset] == '\n' && source[offset-1] == '\r' {
			offset--
		}

		tree, position := markedTree(t, parser, source[:offset]+"¦"+source[offset:])

		item, err := tree.Hover(position)
		if err != nil {
			t.Fatal(err)
		}

		if item.Metadata != nil {
			checkRange(t, tree.Source(), position, item.Range)
		} else if item != (syntax.HoverItem{}) {
			t.Fatalf("Hover without metadata returned a nonzero result: %+v", item)
		}

		if string(tree.Source()) != source {
			t.Fatal("Hover changed the source")
		}
	})
}

func newParser(tb testing.TB, definition string) *syntax.Parser {
	tb.Helper()

	source := testdata.Schema

	if definition != "" {
		var fixture struct {
			Defs json.RawMessage `json:"$defs"`
		}

		err := json.Unmarshal(source, &fixture)
		if err != nil {
			tb.Fatal(err)
		}

		source, err = json.Marshal(map[string]any{"$defs": fixture.Defs, "$ref": "#/$defs/" + definition})
		if err != nil {
			tb.Fatal(err)
		}
	}

	parser, err := syntax.NewBuilder().WithJSONSchema(source).Build(tb.Context())
	if err != nil {
		tb.Fatal(err)
	}

	tb.Cleanup(parser.Close)

	return parser
}

func markedTree(t *testing.T, parser *syntax.Parser, marked string) (*syntax.Tree, syntax.Position) {
	t.Helper()

	offset := strings.Index(marked, "¦")
	if offset < 0 {
		t.Fatal("missing cursor marker")
	}

	source := strings.Replace(marked, "¦", "", 1)

	tree, err := parser.Parse(t.Context(), []byte(source))
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(tree.Close)

	var position syntax.Position

	for _, character := range []byte(source[:offset]) {
		if character == '\n' {
			position.Line++

			position.ByteColumn = 0
		} else {
			position.ByteColumn++
		}
	}

	return tree, position
}

func checkRange(t *testing.T, source []byte, position syntax.Position, span syntax.Range) {
	t.Helper()

	lines := strings.Split(string(source), "\n")
	for _, point := range []syntax.Position{span.Start, span.End, position} {
		if int(point.Line) >= len(lines) {
			t.Fatalf("position %+v is outside the source", point)
		}

		line := lines[point.Line]
		if int(point.Line)+1 < len(lines) {
			line = strings.TrimSuffix(line, "\r")
		}

		if int(point.ByteColumn) > len(line) || !utf8.ValidString(line[:point.ByteColumn]) {
			t.Fatalf("invalid byte position %+v in %q", point, line)
		}
	}

	before := func(first, second syntax.Position) bool {
		return first.Line < second.Line || first.Line == second.Line && first.ByteColumn < second.ByteColumn
	}

	if before(position, span.Start) || !before(position, span.End) {
		t.Fatalf("hover range [%v, %v) does not contain cursor %v", span.Start, span.End, position)
	}
}
