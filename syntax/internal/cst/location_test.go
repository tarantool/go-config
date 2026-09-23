package cst //nolint:testpackage // Tests share raw CST fixtures with context tests.

import (
	"strings"
	"testing"

	sitter "github.com/smacker/go-tree-sitter"
	yamlgrammar "github.com/smacker/go-tree-sitter/yaml"
)

// expectedNode describes the CST anchor independently of completion policy.
type expectedNode struct {
	nodeType   string
	text       string
	parentType string
}

// These tests describe the CST anchor at the cursor, not the completion kind
// or the future path through JSON Schema.
func TestLocateNode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		source   string
		position sitter.Point
		want     expectedNode
	}{
		{
			name: "empty document",
			want: expectedNode{nodeType: "stream"},
		},
		{
			name:   "inside a key",
			source: "mode: dev", position: sitter.Point{Row: 0, Column: 2},
			want: expectedNode{nodeType: "string_scalar", text: "mode", parentType: "plain_scalar"},
		},
		{
			name:   "cursor at end of partial key",
			source: "mo", position: sitter.Point{Row: 0, Column: 2},
			want: expectedNode{nodeType: "string_scalar", text: "mo", parentType: "plain_scalar"},
		},
		{
			name:   "empty value after colon",
			source: "mode: ", position: sitter.Point{Row: 0, Column: 6},
			want: expectedNode{nodeType: "block_mapping_pair", text: "mode:", parentType: "block_mapping"},
		},
		{
			name:   "empty line after completed value",
			source: "enabled: true\n", position: sitter.Point{Row: 1, Column: 0},
			want: expectedNode{nodeType: "boolean_scalar", text: "true", parentType: "plain_scalar"},
		},
		{
			name:   "empty line after CRLF",
			source: "enabled: true\r\n", position: sitter.Point{Row: 1, Column: 0},
			want: expectedNode{nodeType: "boolean_scalar", text: "true", parentType: "plain_scalar"},
		},
		{
			name:   "empty indented line after key",
			source: "server:\n  ", position: sitter.Point{Row: 1, Column: 2},
			want: expectedNode{nodeType: "block_mapping_pair", text: "server:", parentType: "block_mapping"},
		},
		{
			name:   "empty line after nested sibling",
			source: "server:\n  host: localhost\n  ", position: sitter.Point{Row: 2, Column: 2},
			want: expectedNode{nodeType: "string_scalar", text: "localhost", parentType: "plain_scalar"},
		},
		{
			name:   "empty line before next sibling uses previous value",
			source: "server:\n  host: localhost\n\n  port: 1", position: sitter.Point{Row: 2, Column: 0},
			want: expectedNode{nodeType: "string_scalar", text: "localhost", parentType: "plain_scalar"},
		},
		{
			name:   "empty flow mapping",
			source: "server: {}", position: sitter.Point{Row: 0, Column: 9},
			want: expectedNode{nodeType: "flow_mapping", text: "{}", parentType: "flow_node"},
		},
		{
			name:   "space before first flow key",
			source: "server: { host: localhost}", position: sitter.Point{Row: 0, Column: 9},
			want: expectedNode{nodeType: "flow_mapping", text: "{ host: localhost}", parentType: "flow_node"},
		},
		{
			name:   "flow mapping after comma",
			source: "server: {host: localhost, }", position: sitter.Point{Row: 0, Column: 26},
			want: expectedNode{nodeType: "flow_mapping", text: "{host: localhost, }", parentType: "flow_node"},
		},
		{
			name:   "empty flow sequence",
			source: "regions: []", position: sitter.Point{Row: 0, Column: 10},
			want: expectedNode{nodeType: "flow_sequence", text: "[]", parentType: "flow_node"},
		},
		{
			name:   "partial item in incomplete flow sequence",
			source: "regions: [ea", position: sitter.Point{Row: 0, Column: 12},
			want: expectedNode{nodeType: "string_scalar", text: "ea", parentType: "plain_scalar"},
		},
		{
			name:   "partial key in incomplete flow mapping",
			source: "server: {ho", position: sitter.Point{Row: 0, Column: 11},
			want: expectedNode{nodeType: "string_scalar", text: "ho", parentType: "plain_scalar"},
		},
		{
			name:   "inside comment",
			source: "# mode: pr", position: sitter.Point{Row: 0, Column: 5},
			want: expectedNode{nodeType: "comment", text: "# mode: pr", parentType: "stream"},
		},
		{
			name:   "blank before first node has no previous content",
			source: "\nmode: dev", position: sitter.Point{Row: 0, Column: 0},
			want: expectedNode{nodeType: "stream", text: "mode: dev"},
		},
		{
			name:   "UTF-8 cursor column is measured in bytes",
			source: "ключ: 🚀", position: sitter.Point{Row: 0, Column: 14},
			want: expectedNode{nodeType: "string_scalar", text: "🚀", parentType: "plain_scalar"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			content := []byte(tt.source)

			tree := parseTree(t, content)

			location, err := Locate(tree.RootNode(), content, LineStarts(content), tt.position)
			if err != nil {
				t.Fatal(err)
			}

			got := location.Node
			if got == nil {
				t.Fatal("locateNode() returned no node")
			}

			parentType := ""
			if parent := got.Parent(); parent != nil {
				parentType = parent.Type()
			}

			actual := expectedNode{
				nodeType: got.Type(), text: got.Content(content),
				parentType: parentType,
			}
			if actual != tt.want {
				t.Errorf("node = %+v, want %+v", actual, tt.want)
			}
		})
	}
}

func TestLocateNodeRejectsInvalidPosition(t *testing.T) {
	t.Parallel()

	content := []byte("ключ: 🚀")
	tree := parseTree(t, content)

	for _, pos := range []sitter.Point{
		{Row: 1, Column: 0},
		{Row: 0, Column: 15},
		{Row: 0, Column: 1}, {Row: 0, Column: 1}, // middle of a UTF-8 code point.
	} {
		_, err := Locate(tree.RootNode(), content, LineStarts(content), pos)
		if err == nil {
			t.Errorf("locateNode(%v) should reject invalid position", pos)
		}
	}
}

func TestLocationContainsAnchor(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		source   string
		position sitter.Point
		inside   bool
	}{
		{
			name: "inside key", source: "mode: prod",
			position: sitter.Point{Row: 0, Column: 2}, inside: true,
		},
		{
			name: "inside value", source: "mode: prod",
			position: sitter.Point{Row: 0, Column: 8}, inside: true,
		},
		{
			name: "end of key", source: "mode",
			position: sitter.Point{Row: 0, Column: 4}, inside: false,
		},
		{
			name: "space after value", source: "mode: prod  ",
			position: sitter.Point{Row: 0, Column: 11}, inside: false,
		},
		{
			name: "empty next line", source: "mode: prod\n",
			position: sitter.Point{Row: 1, Column: 0}, inside: false,
		},
		{
			name: "empty input", source: "",
			position: sitter.Point{Row: 0, Column: 0}, inside: false,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			content := []byte(test.source)

			tree := parseTree(t, content)

			location, err := Locate(tree.RootNode(), content, LineStarts(content), test.position)
			if err != nil {
				t.Fatal(err)
			}

			if got := location.Contains(location.Node); got != test.inside {
				t.Errorf("Contains(anchor) = %v, want %v", got, test.inside)
			}
		})
	}
}

func TestLineBoundaries(t *testing.T) {
	t.Parallel()

	type lineBounds struct {
		start, end int
		column     uint32
	}

	for _, test := range []struct {
		name, source string
		lines        []lineBounds
	}{
		{"empty", "", []lineBounds{{0, 0, 0}}},
		{"LF", "a\nb\n", []lineBounds{{0, 1, 1}, {2, 3, 1}, {4, 4, 0}}},
		{"CRLF and Unicode", "ключ: 🚀\r\n\r\nx", []lineBounds{{0, 14, 14}, {16, 16, 0}, {18, 19, 1}}},
		{"trailing CRLF", "a\r\n", []lineBounds{{0, 1, 1}, {3, 3, 0}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			content := []byte(test.source)
			starts := LineStarts(content)

			var line uint32

			for _, bounds := range test.lines {
				start, end, column := bounds.start, bounds.end, bounds.column
				if starts[line] != start || LineEnd(content, starts, line) != end {
					t.Fatalf("line %d = [%d, %d], want [%d, %d]",
						line, starts[line], LineEnd(content, starts, line), start, end)
				}

				for _, boundary := range []struct {
					column uint32
					offset int
				}{{0, start}, {column, end}} {
					offset, err := SourceOffset(content, starts, sitter.Point{Row: line, Column: boundary.column})
					if err != nil || offset != boundary.offset {
						t.Errorf("sourceOffset(%d, %d) = %d, %v; want %d",
							line, boundary.column, offset, err, boundary.offset)
					}
				}

				_, err := SourceOffset(content, starts, sitter.Point{Row: line, Column: column + 1})
				if err == nil {
					t.Errorf("line %d accepted a column beyond its content", line)
				}

				line++
			}

			_, err := SourceOffset(content, starts, sitter.Point{Row: line})
			if err == nil {
				t.Error("accepted a line beyond the source")
			}
		})
	}
}

func TestLocateBeforeComment(t *testing.T) {
	t.Parallel()

	content := []byte("mode: prod # comment")
	tree := parseTree(t, content)

	location, err := Locate(tree.RootNode(), content, LineStarts(content), sitter.Point{Column: 11})
	if err != nil {
		t.Fatal(err)
	}

	if location.Node.Content(content) != "prod" || location.Contains(location.Node) {
		t.Fatalf("comment start should anchor preceding content without containing the cursor: %+v", location)
	}
}

// parseTree builds a raw YAML CST and registers its cleanup.
func parseTree(t *testing.T, source []byte) *sitter.Tree {
	t.Helper()

	parser := sitter.NewParser()
	parser.SetLanguage(yamlgrammar.GetLanguage())
	t.Cleanup(parser.Close)

	tree, err := parser.ParseCtx(t.Context(), nil, source)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(tree.Close)

	return tree
}

func cursorPosition(marked string) (string, sitter.Point) {
	prefix, suffix, _ := strings.Cut(marked, "¦")

	var position sitter.Point

	for _, character := range []byte(prefix) {
		if character == '\n' {
			position.Row++

			position.Column = 0
		} else {
			position.Column++
		}
	}

	return prefix + suffix, position
}
