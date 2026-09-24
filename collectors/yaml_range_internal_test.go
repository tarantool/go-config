package collectors

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"

	"github.com/tarantool/go-config/v2/tree"
)

// The text a node's range covers must parse back to the node's value.
func TestYamlRanges_SliceReparses(t *testing.T) {
	t.Parallel()

	files, err := filepath.Glob("testdata/*.yaml")
	require.NoError(t, err)

	nested, err := filepath.Glob("testdata/configdir/*.yaml")
	require.NoError(t, err)

	files = append(files, nested...)

	corpus := map[string]string{}

	for _, file := range files {
		data, err := os.ReadFile(filepath.Clean(file))
		require.NoError(t, err)

		corpus[file] = string(data)
		corpus[file+" (BOM)"] = "\xef\xbb\xbf" + string(data)
		corpus[file+" (CRLF)"] = strings.ReplaceAll(string(data), "\n", "\r\n")
		corpus[file+" (CR)"] = strings.ReplaceAll(string(data), "\n", "\r")
		corpus[file+" (NEL)"] = strings.ReplaceAll(string(data), "\n", "\xc2\x85")
	}

	checked := 0

	for name, src := range corpus {
		var doc yaml.Node

		if yaml.Unmarshal([]byte(src), &doc) != nil {
			continue
		}

		ranges := newYamlRanges([]byte(src), &doc)

		walkYaml(&doc, func(node *yaml.Node) {
			if node.Kind == yaml.DocumentNode || node.Line == 0 {
				return
			}

			span, ok := ranges.spans[node]
			require.True(t, ok, "%s: node at %d:%d not measured", name, node.Line, node.Column)

			rng := span.rng

			// An implicit null is moved next to its key.
			if !span.null {
				assert.Equal(t, tree.Position{Line: node.Line, Column: node.Column}, rng.Start)
			}

			if hasAlias(node) || hasKeptBlankLines(node) || hasExplicitIndent(node, ranges) {
				return
			}

			covered := sliceRange(string(ranges.src), rng)
			raw := strings.TrimLeft(covered, " ")

			// Trailing spaces of a block scalar line are content.
			if !endsInBlockScalar(node) {
				assert.Equal(t, strings.TrimRight(raw, " \t\n"), raw,
					"%s: range %v of node at %d:%d ends in blanks", name, rng, node.Line, node.Column)
			}

			// Clip chomping keeps a final line break the range leaves out.
			text := covered + "\n"

			var want, got any

			require.NoError(t, node.Decode(&want))

			// `? a : b` outside a flow sequence is a different mapping.
			if node.Kind == yaml.MappingNode && node.Style&yaml.FlowStyle != 0 && !strings.HasPrefix(raw, "{") {
				text = "[" + raw + "]\n"
				want = []any{want}
			}

			require.NoError(t, yaml.Unmarshal([]byte(text), &got),
				"%s: range %v of node at %d:%d covers %q", name, rng, node.Line, node.Column, text)
			assert.Equal(t, want, got,
				"%s: range %v of node at %d:%d covers %q", name, rng, node.Line, node.Column, text)

			checked++
		})
	}

	assert.Greater(t, checked, 200)
}

func TestYamlRanges_Alias(t *testing.T) {
	t.Parallel()

	src := "a: &x {k: v}\nb: *x\n"

	var doc yaml.Node

	require.NoError(t, yaml.Unmarshal([]byte(src), &doc))

	ranges := newYamlRanges([]byte(src), &doc)
	alias := doc.Content[0].Content[3]

	require.Equal(t, yaml.AliasNode, alias.Kind)
	assert.Equal(t, tree.NewRange(2, 4, 2, 6), ranges.get(alias))
	assert.Equal(t, tree.NewRange(1, 4, 1, 13), ranges.get(alias.Alias))
}

// An explicit indentation indicator depends on the enclosing collection, so
// these scalars do not reparse out of context.
func TestYamlRanges_ExplicitIndent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		src   string
		path  []int
		value string
		want  tree.Range
	}{
		{"k: |2\n    x\n  y\nn: 1\n", []int{0, 1}, "  x\ny\n", tree.NewRange(1, 4, 3, 4)},
		{"- |1\n   x\n  y\n", []int{0, 0}, "  x\n y\n", tree.NewRange(1, 3, 3, 4)},
		{"k:\n  - |1\n     a\n    b\nn: 1\n", []int{0, 1, 0}, "  a\n b\n", tree.NewRange(2, 5, 4, 6)},
		{"|1\n a\n# tail\n", []int{0}, "a\n", tree.NewRange(1, 1, 2, 3)},
	}

	for _, tt := range tests {
		var doc yaml.Node

		require.NoError(t, yaml.Unmarshal([]byte(tt.src), &doc), tt.src)

		node := &doc
		for _, i := range tt.path {
			node = node.Content[i]
		}

		require.Equal(t, tt.value, node.Value, tt.src)
		assert.Equal(t, tt.want, newYamlRanges([]byte(tt.src), &doc).get(node), tt.src)
	}
}

// A reparse cannot see a range that swallows a comment or loses syntax that
// does not change the value.
func TestYamlRanges_Exact(t *testing.T) {
	t.Parallel()

	tests := []struct {
		src  string
		path []int
		want tree.Range
	}{
		{"\xef\xbb\xbfk: [x, y]\n", []int{0, 1, 0}, tree.NewRange(1, 5, 1, 6)},
		{"k: x\rn: y\r", []int{0, 3}, tree.NewRange(2, 4, 2, 5)},
		{"k: x\xe2\x80\xa8n: y\n", []int{0, 3}, tree.NewRange(2, 4, 2, 5)},
		{"k: |\n  x  \nn: y\n", []int{0, 1}, tree.NewRange(1, 4, 2, 6)},
		{"k: [a,# ]\n b]\n", []int{0, 1}, tree.NewRange(1, 4, 2, 4)},
		{"[a:]\n", []int{0, 0}, tree.NewRange(1, 2, 1, 4)},
		{"|\n# hi\n", []int{0}, tree.NewRange(1, 1, 1, 2)},
		{"k: [a: , b]\n", []int{0, 1, 0}, tree.NewRange(1, 5, 1, 7)},
		{"k: >2-\n  x\n    \nn: y\n", []int{0, 1}, tree.NewRange(1, 4, 3, 5)},
		{"k: [\"a\"# ]\n ,b]\n", []int{0, 1}, tree.NewRange(1, 4, 2, 5)},
		{"k: [[]# ]\n ,b]\n", []int{0, 1}, tree.NewRange(1, 4, 2, 5)},
		{"k: |2-\n  \t\nn: y\n", []int{0, 1}, tree.NewRange(1, 4, 2, 4)},
		{"k: |\n  x\n  \t\nn: y\n", []int{0, 1}, tree.NewRange(1, 4, 3, 4)},
		{"k: |-\n  x\n  \nn: 1\n", []int{0, 1}, tree.NewRange(1, 4, 2, 4)},
		{"k: |-\n  x\n   \nn: 1\n", []int{0, 1}, tree.NewRange(1, 4, 3, 4)},
		{"script: |\n    \n  # TODO: fill in\nnext: 1\n", []int{0, 1}, tree.NewRange(1, 9, 1, 10)},
		{"k: a # c\n", []int{0, 1}, tree.NewRange(1, 4, 1, 5)},
		{"k: x\xe2\x80\xa9n: y\n", []int{0, 3}, tree.NewRange(2, 4, 2, 5)},
		{"!!str key: |\n  text\nother: 1\n", []int{0, 1}, tree.NewRange(1, 12, 2, 7)},
		{"&k key: long\n  text\nother: 1\n", []int{0, 1}, tree.NewRange(1, 9, 2, 7)},
		{"k: !!str\n&a n: 1\n", []int{0, 1}, tree.NewRange(1, 4, 1, 9)},
		{"[a]: 1\nb: 2\n", []int{0}, tree.NewRange(1, 1, 2, 5)},
		{"[a]: 1\nb: 2\n", []int{0, 1}, tree.NewRange(1, 6, 1, 7)},
		{"players: !!set\n  ? Mark\n  ? Sammy\n\n# Next\n\nnext: 1\n", []int{0, 1}, tree.NewRange(1, 10, 3, 10)},
		{"[\n  primary: ,\n  secondary\n]\n", []int{0, 0}, tree.NewRange(2, 3, 2, 11)},
		{"--- [\n  y: ]\n", []int{0, 0}, tree.NewRange(2, 3, 2, 5)},
		{"- a\n-\n", []int{0}, tree.NewRange(1, 1, 2, 2)},
		{"k: [?'x]', y]\n", []int{0, 1}, tree.NewRange(1, 4, 1, 14)},
		{"a: &x k\nb: {*x:\"}\"}\n", []int{0, 3}, tree.NewRange(2, 4, 2, 12)},
		{"k: &a:b\nx: 1\n", []int{0, 1}, tree.NewRange(1, 4, 1, 8)},
		{"k: [!!str, a]\n", []int{0, 1, 0}, tree.NewRange(1, 5, 1, 13)},
		{"---", []int{0}, tree.NewRange(1, 4, 1, 4)},
		{"players: !!set\n  ? Mark\n  ? Sammy\n\n# Next\n\nnext: 1\n", []int{0, 1, 3}, tree.NewRange(3, 10, 3, 10)},
		{"[\n0: ]\n", []int{0, 0}, tree.NewRange(2, 1, 2, 3)},
		{"[\n0: ]\n", []int{0, 0, 1}, tree.NewRange(2, 3, 2, 3)},
		{"? b\n&anchor c: 3\n", []int{0, 1}, tree.NewRange(1, 4, 1, 4)},
		{"k:\nn: 1\n", []int{0, 1}, tree.NewRange(1, 3, 1, 3)},
		{"k: &a\nn: 1\n", []int{0, 1}, tree.NewRange(1, 4, 1, 6)},
		{"k: &a\n!!str n: 1\n", []int{0, 1}, tree.NewRange(1, 4, 1, 6)},
		{"k: &a\n  !!str\n  \"a # b\"\nn: 1\n", []int{0, 1}, tree.NewRange(1, 4, 3, 10)},
		{"k: &a\n  !!str\n  |\n    # not a comment\n    text\nn: 1\n", []int{0, 1}, tree.NewRange(1, 4, 5, 9)},
		{"k: &a\n  !!map\n  {x: 1, y: 2}\nn: 1\n", []int{0, 1}, tree.NewRange(1, 4, 3, 15)},
		{"k: &a\n  !!map\n  {x: 1, y: 2}\nn: 1\n", []int{0, 1, 1}, tree.NewRange(3, 7, 3, 8)},
		{"&a1\n!!seq\n[a, b]\n", []int{0}, tree.NewRange(1, 1, 3, 7)},
		{"&a1\n!!seq\n[a, b]\n", []int{0, 0}, tree.NewRange(3, 2, 3, 3)},
		{"k: ! 'x # y'\n", []int{0, 1}, tree.NewRange(1, 4, 1, 13)},
		{"- !\n", []int{0}, tree.NewRange(1, 1, 1, 4)},
		{"- !\n", []int{0, 0}, tree.NewRange(1, 3, 1, 4)},
		{"k: !\nn: 1\n", []int{0, 1}, tree.NewRange(1, 4, 1, 5)},
		{"k: !<!> \"a # b\"\nn: 1\n", []int{0, 1}, tree.NewRange(1, 4, 1, 16)},
		{"k: !<!> |\n  # x\n  y\nn: 1\n", []int{0, 1}, tree.NewRange(1, 4, 3, 4)},
		{"k: !<!> [a, b]\nn: 1\n", []int{0, 1, 0}, tree.NewRange(1, 10, 1, 11)},
		{"? \n::", []int{0, 1}, tree.NewRange(1, 2, 1, 2)},
		{"k: !\n! n: 1\n", []int{0, 1}, tree.NewRange(1, 4, 1, 5)},
		{"k: !!null\n! n: 1\n", []int{0, 1}, tree.NewRange(1, 4, 1, 10)},
		{"k: &a\n! n: 1\n", []int{0, 1}, tree.NewRange(1, 4, 1, 6)},
		{"0: &0\n! :", []int{0, 1}, tree.NewRange(1, 4, 1, 6)},
		{"!00 0: \n- &0\n! :", []int{0, 1, 0}, tree.NewRange(2, 3, 2, 5)},
		{"k: &a\n&b n: 1\n", []int{0, 1}, tree.NewRange(1, 4, 1, 6)},
		{"? a\n! b: 1\n", []int{0, 1}, tree.NewRange(1, 4, 1, 4)},
		{"? a\n!t b: 1\n", []int{0, 1}, tree.NewRange(1, 4, 1, 4)},
		{"? a\n&x b: 1\n", []int{0, 1}, tree.NewRange(1, 4, 1, 4)},
		{"? \n! 0:", []int{0, 1}, tree.NewRange(1, 2, 1, 2)},
		{"? - ? a\n: v\n", []int{0, 0}, tree.NewRange(1, 3, 1, 8)},
		{"? - ? a\n: v\n", []int{0, 0, 0}, tree.NewRange(1, 5, 1, 8)},
		{"? a\n:\nb: 1\n", []int{0, 1}, tree.NewRange(2, 2, 2, 2)},
		{"k:\n  - ? a\nn: 1\n", []int{0, 1, 0}, tree.NewRange(2, 5, 2, 8)},
		{"[&m\n{a: 1}]\n", []int{0, 0}, tree.NewRange(1, 2, 2, 7)},
		{"k: [a, # c ]\n]\n", []int{0, 1}, tree.NewRange(1, 4, 2, 2)},
		{"{? }\n", []int{0}, tree.NewRange(1, 1, 1, 5)},
		{"{? : }\n", []int{0}, tree.NewRange(1, 1, 1, 7)},
		{"{a: , b: c}\n", []int{0}, tree.NewRange(1, 1, 1, 12)},
		{"[[a]: b]\n", []int{0, 0}, tree.NewRange(1, 2, 1, 8)},
		{"[{a: b}: c]\n", []int{0, 0}, tree.NewRange(1, 2, 1, 11)},
	}

	for _, tt := range tests {
		var doc yaml.Node

		require.NoError(t, yaml.Unmarshal([]byte(tt.src), &doc), tt.src)

		node := &doc
		for _, i := range tt.path {
			node = node.Content[i]
		}

		assert.Equal(t, tt.want, newYamlRanges([]byte(tt.src), &doc).get(node), "%q", tt.src)
	}
}

func TestYamlRanges_UTF16(t *testing.T) {
	t.Parallel()

	// A surrogate pair before x, and a CRLF inside the sequence.
	units := utf16.Encode([]rune("k: [" + string(rune(0x1f600)) + ", x,\r\n y]\n"))

	for name, order := range map[string]binary.AppendByteOrder{"LE": binary.LittleEndian, "BE": binary.BigEndian} {
		src := order.AppendUint16(nil, 0xfeff)
		for _, unit := range units {
			src = order.AppendUint16(src, unit)
		}

		var doc yaml.Node

		require.NoError(t, yaml.Unmarshal(src, &doc), name)

		ranges := newYamlRanges(src, &doc)
		items := doc.Content[0].Content[1].Content

		require.Equal(t, "x", items[1].Value, name)
		assert.Equal(t, tree.NewRange(1, 8, 1, 9), ranges.get(items[1]), name)
		assert.Equal(t, tree.NewRange(2, 2, 2, 3), ranges.get(items[2]), name)
	}
}

func walkYaml(node *yaml.Node, visit func(*yaml.Node)) {
	visit(node)

	for _, child := range node.Content {
		walkYaml(child, visit)
	}
}

func hasAlias(node *yaml.Node) bool {
	return node.Kind == yaml.AliasNode || slices.ContainsFunc(node.Content, hasAlias)
}

func hasKeptBlankLines(node *yaml.Node) bool {
	if node.Style&(yaml.LiteralStyle|yaml.FoldedStyle) != 0 && strings.HasSuffix(node.Value, "\n\n") {
		return true
	}

	return slices.ContainsFunc(node.Content, hasKeptBlankLines)
}

func endsInBlockScalar(node *yaml.Node) bool {
	if len(node.Content) > 0 {
		return endsInBlockScalar(node.Content[len(node.Content)-1])
	}

	return node.Style&(yaml.LiteralStyle|yaml.FoldedStyle) != 0
}

func hasExplicitIndent(node *yaml.Node, ranges *yamlRanges) bool {
	if node.Style&(yaml.LiteralStyle|yaml.FoldedStyle) != 0 {
		_, pos := ranges.skipProperties(ranges.offset(node.Line, node.Column), node, nil)
		header := ranges.src[pos:ranges.lineEnd(pos)]

		if bytes.ContainsAny(header[:min(len(header), 3)], "123456789") {
			return true
		}
	}

	return slices.ContainsFunc(node.Content, func(child *yaml.Node) bool {
		return hasExplicitIndent(child, ranges)
	})
}

// sliceRange keeps the original column so block nodes keep their structure.
func sliceRange(src string, rng tree.Range) string {
	lines := strings.SplitAfter(src, "\n")

	var out strings.Builder

	out.WriteString(strings.Repeat(" ", rng.Start.Column-1))

	for line := rng.Start.Line; line <= rng.End.Line; line++ {
		runes := []rune(lines[line-1])

		first, last := 0, len(runes)
		if line == rng.Start.Line {
			first = rng.Start.Column - 1
		}

		if line == rng.End.Line {
			last = rng.End.Column - 1
		}

		out.WriteString(string(runes[first:last]))
	}

	return out.String()
}
