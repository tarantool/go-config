package cst //nolint:testpackage // Tests share raw CST fixtures with coordinate tests.

import (
	"slices"
	"strings"
	"testing"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/tarantool/go-config/v2/syntax/internal/path"
	"github.com/tarantool/go-config/v2/syntax/internal/schema"
	"github.com/tarantool/go-config/v2/testdata"
)

func TestNodeContext(t *testing.T) {
	t.Parallel()

	schemaSource := testdata.Schema

	compiled := schemaForTest(t, schemaSource)

	tests := []struct {
		name     string
		source   string
		isKey    bool
		path     []path.Step
		flow     bool
		itemType string
	}{
		{
			name:     "property key",
			source:   "server:\n  ho¦st: localhost",
			isKey:    true,
			path:     []path.Step{path.Property("server")},
			itemType: "string",
		},
		{
			name:     "quoted property key",
			source:   "'server':\n  'ho¦st': localhost",
			isKey:    true,
			path:     []path.Step{path.Property("server")},
			itemType: "string",
		},
		{
			name:     "property value",
			source:   "server:\n  tls: tr¦ue",
			isKey:    false,
			path:     []path.Step{path.Property("server"), path.Property("tls")},
			itemType: "boolean",
		},
		{
			name:     "flow value",
			source:   "server: {tls: tr¦ue}",
			isKey:    false,
			path:     []path.Step{path.Property("server"), path.Property("tls")},
			flow:     true,
			itemType: "boolean",
		},
		{
			name:     "incomplete flow value",
			source:   "server: {tls: tr¦",
			isKey:    false,
			path:     []path.Step{path.Property("server"), path.Property("tls")},
			flow:     true,
			itemType: "boolean",
		},
		{
			name:   "block array index ignores comments",
			source: "servers:\n  - host: localhost\n  # comment\n  - tls: tr¦ue",
			isKey:  false,
			path: []path.Step{
				path.Property("servers"), path.Index(1), path.Property("tls"),
			},
			itemType: "boolean",
		},
		{
			name:   "flow array index",
			source: "servers: [{host: localhost}, {tls: tr¦ue}]",
			isKey:  false,
			path: []path.Step{
				path.Property("servers"), path.Index(1), path.Property("tls"),
			},
			flow:     true,
			itemType: "boolean",
		},
		{
			name:     "multiline value remains available for inspection",
			source:   "mode: pro\n  d¦",
			isKey:    false,
			path:     []path.Step{path.Property("mode")},
			itemType: "string",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			offset := strings.Index(test.source, "¦")
			source := strings.Replace(test.source, "¦", "", 1)
			content := []byte(source)
			tree := parseTree(t, content)

			position := sitter.Point{
				//nolint:gosec // Test fixtures and fuzz inputs fit in uint32.
				Row: uint32(strings.Count(source[:offset], "\n")),
				//nolint:gosec // Test fixtures and fuzz inputs fit in uint32.
				Column: uint32(offset - strings.LastIndex(source[:offset], "\n") - 1),
			}

			location, err := Locate(tree.RootNode(), content, LineStarts(content), position)
			if err != nil {
				t.Fatal(err)
			}

			context := NodeContext(content, compiled, location.Node)
			if context.IsKey != test.isKey || context.Flow != test.flow || !slices.Equal(context.Path, test.path) {
				t.Fatalf("context = %+v; want isKey %v, flow %v, path %v", context, test.isKey, test.flow, test.path)
			}

			path := FieldPath(content, context)
			if itemType := schema.Type(compiled.Lookup(path)); itemType != test.itemType {
				t.Fatalf("item type = %q, want %q", itemType, test.itemType)
			}

			if string(content) != source {
				t.Fatal("context Lookup changed the source")
			}
		})
	}
}

func TestErrorContext(t *testing.T) {
	t.Parallel()

	schemaSource := testdata.Schema

	compiled := schemaForTest(t, schemaSource)

	tests := []struct {
		name   string
		source string
		isKey  bool
		path   []path.Step
		flow   bool
		token  string
	}{
		{
			name:   "unclosed mapping",
			source: "server: {",
			isKey:  true,
			path:   []path.Step{path.Property("server")},
			flow:   true,
		},
		{
			name:   "unclosed sequence",
			source: "regions: [",
			isKey:  false,
			path:   []path.Step{path.Property("regions"), path.Index(0)},
			flow:   true,
		},
		{
			name:   "unclosed mapping in block sequence",
			source: "servers:\n  - {",
			isKey:  true,
			path:   []path.Step{path.Property("servers"), path.Index(0)},
			flow:   true,
		},
		{
			name:   "unclosed mapping in indentless sequence",
			source: "servers:\n- {",
			isKey:  true,
			path:   []path.Step{path.Property("servers"), path.Index(0)},
			flow:   true,
		},
		{
			name:   "unclosed quote",
			source: "mode: \"pr",
			isKey:  false,
			path:   []path.Step{path.Property("mode")},
			token:  "\"",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			content := []byte(test.source)

			tree := parseTree(t, content)

			position := sitter.Point{
				//nolint:gosec // Test fixtures and fuzz inputs fit in uint32.
				Row: uint32(strings.Count(test.source, "\n")),
				//nolint:gosec // Test fixtures and fuzz inputs fit in uint32.
				Column: uint32(len(test.source) - strings.LastIndex(test.source, "\n") - 1),
			}

			location, err := Locate(tree.RootNode(), content, LineStarts(content), position)
			if err != nil {
				t.Fatal(err)
			}

			node, offset := location.Node, location.Offset
			for node != nil && !node.IsError() {
				node = node.Parent()
			}

			if node == nil {
				t.Fatal("expected an ERROR ancestor")
			}

			context, token := ErrorContext(
				content, compiled, node, NodeContext(content, compiled, node), position, offset,
			)
			if context.IsKey != test.isKey || context.Flow != test.flow || !slices.Equal(context.Path, test.path) {
				t.Fatalf("context = %+v; want isKey %v, flow %v, path %v", context, test.isKey, test.flow, test.path)
			}

			tokenType := ""
			if token != nil {
				tokenType = token.Type()
			}

			if tokenType != test.token {
				t.Fatalf("token = %q, want %q", tokenType, test.token)
			}
		})
	}
}

func TestFieldPath(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name   string
		source string
		want   []path.Step
	}{
		{
			name: "nested key", source: "server:\n  ho¦st: localhost",
			want: []path.Step{path.Property("server"), path.Property("host")},
		},
		{
			name: "bare root key", source: "mo¦de",
			want: []path.Step{path.Property("mode")},
		},
		{
			name: "bare flow key", source: "server: {ho¦st}",
			want: []path.Step{path.Property("server"), path.Property("host")},
		},
		{
			name: "recovered flow key", source: "server: {ho¦st",
			want: []path.Step{path.Property("server"), path.Property("host")},
		},
		{
			name: "quoted empty key", source: "'¦': value",
			want: []path.Step{path.Property("")},
		},
		{
			name: "numeric property", source: "'0¦': value",
			want: []path.Step{path.Property("0")},
		},
		{
			name: "sequence item", source: "regions: [east, we¦st]",
			want: []path.Step{path.Property("regions"), path.Index(1)},
		},
		{
			name: "value in recovered mapping", source: "server: {tls: tr¦",
			want: []path.Step{path.Property("server"), path.Property("tls")},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			offset := strings.Index(test.source, "¦")
			source := strings.Replace(test.source, "¦", "", 1)
			content := []byte(source)
			tree := parseTree(t, content)

			compiled := schemaForTest(t, []byte(`{"type":"object"}`))

			var position sitter.Point

			for _, character := range []byte(source[:offset]) {
				if character == '\n' {
					position.Row++

					position.Column = 0
				} else {
					position.Column++
				}
			}

			location, err := Locate(tree.RootNode(), content, LineStarts(content), position)
			if err != nil {
				t.Fatal(err)
			}

			context := NodeContext(content, compiled, location.Node)
			if got := FieldPath(content, context); !slices.Equal(got, test.want) {
				t.Fatalf("FieldPath() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestRecoveryAtCursor(t *testing.T) {
	t.Parallel()

	compiled := schemaForTest(t, testdata.Schema)

	server := []path.Step{path.Property("server")}
	host := []path.Step{path.Property("server"), path.Property("host")}
	firstServer := []path.Step{path.Property("servers"), path.Index(0)}
	secondServer := []path.Step{path.Property("servers"), path.Index(1)}
	secondRegion := []path.Step{path.Property("regions"), path.Index(1)}

	for _, test := range []struct {
		name, source     string
		path             []path.Step
		key, flow        bool
		token, separator string
	}{
		{"before a later child", "server: {¦ host: x", server, true, true, "", ""},
		{"after a complete pair", "server: {host: x ¦", host, false, true, "", ""},
		{"inside a complete pair", "server: {ho¦st: x", server, true, true, "", ""},
		{"next key after comma", "server: {host: x, ¦", server, true, true, "", ""},
		{"bare key", "server: {ho¦st", server, true, true, "string_scalar", ""},
		{"block colon", "server:¦ {", server, false, false, "", ":"},
		{"flow colon", "server: {host:¦ [", host, false, true, "", ":"},
		{"comment start", "server: {host: x,\n  ¦# comment\n", server, true, true, "", ""},
		{"inside comment", "server: {host: x,\n  # ¦comment\n", server, true, true, "comment", ""},
		{"after comment", "server: {host: x,\n  # comment\n  ¦", server, true, true, "", ""},
		{"next scalar item", "regions: [east, ¦", secondRegion, false, true, "", ""},
		{"completed nested sequence", "regions: [[east], ¦", secondRegion, false, true, "", ""},
		{"next object in flow sequence", "servers: [{host: x}, {¦", secondServer, true, true, "", ""},
		{"next object in block sequence", "servers:\n  - {}\n  - {¦", secondServer, true, true, "", ""},
		{"indentless sequence", "servers:\n- host: x\n- {¦", secondServer, true, true, "", ""},
		{"inside complete block item", "servers:\n  - {¦}\n  - {", firstServer, true, false, "", ""},
		{"before next block item", "servers:\n  - {}\n  ¦\n  - {", firstServer, true, false, "", ""},
		{"after completed block sibling", "servers:\n  - {}\nserver: {¦", server, true, true, "", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			source, position := cursorPosition(test.source)
			content := []byte(source)
			tree := parseTree(t, content)

			location, err := Locate(tree.RootNode(), content, LineStarts(content), position)
			if err != nil {
				t.Fatal(err)
			}

			node := ancestor(t, location.Node, "ERROR")

			context, token := ErrorContext(
				content, compiled, node, NodeContext(content, compiled, node), position, location.Offset,
			)
			if context.IsKey != test.key || context.Flow != test.flow || !slices.Equal(context.Path, test.path) {
				t.Errorf("context = %+v, want key=%v flow=%v path=%v", context, test.key, test.flow, test.path)
			}

			if got := nodeType(token); got != test.token {
				t.Errorf("token = %q, want %q", got, test.token)
			}

			if got := nodeType(context.Separator); got != test.separator {
				t.Errorf("separator = %q, want %q", got, test.separator)
			}

			if got := string(content); got != source {
				t.Error("recovery modified the original source")
			}
		})
	}
}

func TestContentNodes(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		source, content, scalar string
	}{
		{"mode: &name prod # comment", "string_scalar", "prod"},
		{"mode: &name # comment", "", ""},
		{"mode: !!str prod", "", ""},
		{"mode: [prod]", "flow_sequence", ""},
		{"mode: {nested: prod}", "flow_mapping", ""},
	} {
		t.Run(test.source, func(t *testing.T) {
			t.Parallel()

			content := []byte(test.source)

			tree := parseTree(t, content)

			location, err := Locate(tree.RootNode(), content, LineStarts(content), sitter.Point{})
			if err != nil {
				t.Fatal(err)
			}

			pair := ancestor(t, location.Node, "block_mapping_pair")

			value := pair.ChildByFieldName("value")
			if got := nodeType(ContentNode(value)); got != test.content {
				t.Errorf("ContentNode = %q, want %q", got, test.content)
			}

			scalar := ScalarNode(value)

			var text string

			if scalar != nil {
				text = scalar.Content(content)
			}

			if text != test.scalar {
				t.Errorf("ScalarNode = %q, want %q", text, test.scalar)
			}

			colon := ChildOfType(pair, ":")
			if colon == nil || colon.StartByte() != 4 || colon.EndByte() != 5 {
				t.Errorf("ChildOfType did not find the key's colon: %v", colon)
			}

			if got := ChildOfType(pair, "-"); got != nil {
				t.Errorf("ChildOfType found an absent dash: %v", got)
			}
		})
	}
}

func TestPairAndContainerPaths(t *testing.T) {
	t.Parallel()

	compiled := schemaForTest(t, []byte(`{"type":"object"}`))
	content := []byte("server: {host: prod}")
	tree := parseTree(t, content)

	location, err := Locate(tree.RootNode(), content, LineStarts(content), sitter.Point{Column: 10})
	if err != nil {
		t.Fatal(err)
	}

	pair := ancestor(t, location.Node, "flow_pair")
	context := NodeContext(content, compiled, pair)

	want := []path.Step{path.Property("server"), path.Property("host")}
	if !context.IsKey || !slices.Equal(FieldPath(content, context), want) {
		t.Fatalf("pair lost its field path: %+v", context)
	}

	context.Current = pair.Parent()
	if got := FieldPath(content, context); !slices.Equal(got, context.Path) {
		t.Errorf("container changed its path: %v, want %v", got, context.Path)
	}

	if got := KeyText(content, nil); got != "" {
		t.Errorf("absent key = %q", got)
	}

	complexContent := []byte("? [a, b]\n: value")
	complexKey := parseTree(t, complexContent)

	location, err = Locate(
		complexKey.RootNode(), complexContent, LineStarts(complexContent), sitter.Point{Row: 1, Column: 3},
	)
	if err != nil {
		t.Fatal(err)
	}

	pair = ancestor(t, location.Node, "block_mapping_pair")
	if got := KeyText(complexContent, pair.ChildByFieldName("key")); got != "[a, b]" {
		t.Errorf("non-string key should preserve its source, got %q", got)
	}
}

func ancestor(t *testing.T, node *sitter.Node, kind string) *sitter.Node {
	t.Helper()

	for current := node; current != nil; current = current.Parent() {
		if current.Type() == kind {
			return current
		}
	}

	t.Fatalf("%s has no %s ancestor", node.String(), kind)

	return nil
}

func nodeType(node *sitter.Node) string {
	if node == nil {
		return ""
	}

	return node.Type()
}

// schemaForTest compiles a schema for CST context resolution.
func schemaForTest(t *testing.T, source []byte) *schema.Schema {
	t.Helper()

	compiled, err := schema.Compile(source)
	if err != nil {
		t.Fatal(err)
	}

	return compiled
}
