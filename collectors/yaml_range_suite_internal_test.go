package collectors

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
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

const yamlTestSuiteDir = "testdata/yaml-test-suite"

// Every range of every yaml-test-suite case, in every encoding and line break
// yaml.v3 reads, must nest inside its parent, follow its previous sibling and
// read back as the node's value.
func TestYamlRanges_Suite(t *testing.T) {
	t.Parallel()

	checked := 0

	for name, src := range yamlTestSuite(t) {
		for variant, data := range yamlVariants(src) {
			for _, doc := range decodeYamlStream(data) {
				ranges := newYamlRanges(data, doc)

				checkYamlRangeTree(t, name+variant, ranges, doc)

				walkYaml(doc, func(node *yaml.Node) {
					if checkYamlRangeValue(t, name+variant, ranges, node) {
						checked++
					}
				})
			}
		}
	}

	assert.Greater(t, checked, 5000)
}

// Every range must match the node tree-sitter-yaml finds at the same position.
func TestYamlRanges_TreeSitter(t *testing.T) {
	t.Parallel()

	// yaml.v3 reads these differently from the specification, and so from
	// tree-sitter.
	skip := map[string]string{
		"4ABK.yaml":    `"omitted value:," is a key ending with ":", not a key and an empty value`,
		"652Z.yaml":    `"?foo :," and ":bar" are plain scalars, not a key and a value`,
		"HM87-01.yaml": `"[:x]" holds a mapping with an empty key`,
	}

	oracle := treeSitterRanges(t)
	compared := 0

	for name, src := range yamlTestSuite(t) {
		if _, ok := skip[name]; ok {
			continue
		}

		for variant, data := range yamlVariants(src) {
			pending := map[oracleKey][]tree.Range{}
			for _, node := range oracle[name] {
				pending[node.key] = append(pending[node.key], node.rng)
			}

			for _, doc := range decodeYamlStream(data) {
				ranges := newYamlRanges(data, doc)

				walkYaml(doc, func(node *yaml.Node) {
					if node.Kind == yaml.DocumentNode || node.Line == 0 || isImplicitNull(node) {
						return
					}

					start := tree.Position{Line: node.Line, Column: node.Column}
					key := oracleKey{kind: yamlNodeKind(node), start: start}

					want, ok := pending[key]
					if !assert.True(t, ok && len(want) > 0, "%s: tree-sitter has no %s at %d:%d",
						name+variant, key.kind, node.Line, node.Column) {
						return
					}

					pending[key] = want[1:]

					got := ranges.get(node)

					if endsWithBlockScalar(node) {
						assert.True(t, endsInBlanksPast(ranges, want[0], got),
							"%s: %s with a block scalar at %d:%d: tree-sitter %v, got %v",
							name+variant, key.kind, node.Line, node.Column, want[0], got)
					} else {
						assert.Equal(t, want[0], got, "%s: %s at %d:%d", name+variant, key.kind, node.Line, node.Column)
					}

					compared++
				})
			}
		}
	}

	assert.Greater(t, compared, 5000)
}

type oracleKey struct {
	kind  string
	start tree.Position
}

type oracleNode struct {
	key oracleKey
	rng tree.Range
}

func treeSitterRanges(t *testing.T) map[string][]oracleNode {
	t.Helper()

	file, err := os.Open(filepath.Join(yamlTestSuiteDir, "tree-sitter.txt"))
	require.NoError(t, err)

	defer func() { _ = file.Close() }()

	nodes := map[string][]oracleNode{}
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		var (
			name string
			node oracleNode
		)

		_, err := fmt.Sscanf(scanner.Text(), "%s %s %d:%d %d:%d", &name, &node.key.kind,
			&node.rng.Start.Line, &node.rng.Start.Column, &node.rng.End.Line, &node.rng.End.Column)
		require.NoError(t, err, scanner.Text())

		node.key.start = node.rng.Start
		nodes[name] = append(nodes[name], node)
	}

	require.NoError(t, scanner.Err())

	return nodes
}

func yamlTestSuite(tb testing.TB) map[string][]byte {
	tb.Helper()

	files, err := filepath.Glob(filepath.Join(yamlTestSuiteDir, "*.yaml"))
	require.NoError(tb, err)
	require.NotEmpty(tb, files)

	suite := make(map[string][]byte, len(files))

	for _, file := range files {
		data, err := os.ReadFile(filepath.Clean(file))
		require.NoError(tb, err)

		suite[filepath.Base(file)] = data
	}

	return suite
}

func yamlVariants(src []byte) map[string][]byte {
	text := string(src)

	return map[string][]byte{
		"":            src,
		" (BOM)":      []byte("\xef\xbb\xbf" + text),
		" (CRLF)":     []byte(strings.ReplaceAll(text, "\n", "\r\n")),
		" (CR)":       []byte(strings.ReplaceAll(text, "\n", "\r")),
		" (NEL)":      []byte(strings.ReplaceAll(text, "\n", "\xc2\x85")),
		" (UTF-16LE)": encodeUTF16(text, binary.LittleEndian),
		" (UTF-16BE)": encodeUTF16(text, binary.BigEndian),
	}
}

func encodeUTF16(text string, order binary.AppendByteOrder) []byte {
	out := order.AppendUint16(nil, 0xfeff)
	for _, unit := range utf16.Encode([]rune(text)) {
		out = order.AppendUint16(out, unit)
	}

	return out
}

// decodeYamlStream returns every document of src, or none if yaml.v3 rejects it.
func decodeYamlStream(src []byte) []*yaml.Node {
	decoder := yaml.NewDecoder(bytes.NewReader(src))

	var docs []*yaml.Node

	for {
		doc := &yaml.Node{}

		err := decoder.Decode(doc)
		if errors.Is(err, io.EOF) {
			return docs
		}

		if err != nil {
			return nil
		}

		docs = append(docs, doc)
	}
}

// checkYamlRangeTree checks that node's range starts where yaml.v3 says the
// node does, that it holds the ranges of the node's children and that those
// follow one another.
func checkYamlRangeTree(t *testing.T, name string, ranges *yamlRanges, node *yaml.Node) {
	t.Helper()

	if node.Line == 0 {
		return
	}

	span, ok := ranges.spans[node]
	if !assert.True(t, ok, "%s: node at %d:%d not measured", name, node.Line, node.Column) {
		return
	}

	rng := span.rng

	// yaml.v3 puts an implicit null on the next token; its range sits by its key.
	if !isImplicitNull(node) {
		assert.Equal(t, tree.Position{Line: node.Line, Column: node.Column}, rng.Start, "%s: start", name)
	}

	assert.False(t, positionBefore(rng.End, rng.Start), "%s: range %v of node at %d:%d is reversed",
		name, rng, node.Line, node.Column)

	previous := rng.Start

	for _, child := range node.Content {
		checkYamlRangeTree(t, name, ranges, child)

		childSpan, ok := ranges.spans[child]
		if !ok {
			continue
		}

		childRange := childSpan.rng

		assert.False(t, positionBefore(childRange.Start, previous),
			"%s: child range %v of node at %d:%d starts before %v", name, childRange, node.Line, node.Column, previous)
		assert.False(t, positionBefore(rng.End, childRange.End),
			"%s: child range %v sticks out of %v of node at %d:%d", name, childRange, rng, node.Line, node.Column)

		previous = childRange.End
	}
}

// checkYamlRangeValue checks that the text node's range covers reads back as
// node's value and reports whether the node could be checked this way.
func checkYamlRangeValue(t *testing.T, name string, ranges *yamlRanges, node *yaml.Node) bool {
	t.Helper()

	// A scalar with no content has nothing to read but its properties, and
	// their meaning depends on where the node sits.
	emptyScalar := node.Kind == yaml.ScalarNode && node.Value == "" &&
		node.Style&(yaml.DoubleQuotedStyle|yaml.SingleQuotedStyle|yaml.LiteralStyle|yaml.FoldedStyle) == 0

	// yaml.v3 keeps LS and PS line breaks in a value, which the normalized
	// source no longer tells apart from line feeds.
	if node.Kind == yaml.DocumentNode || node.Line == 0 || emptyScalar || hasLineSeparator(node) ||
		hasAlias(node) || hasKeptBlankLines(node) || hasExplicitIndent(node, ranges) ||
		hasKeepChomping(node, ranges) || bytes.Contains(ranges.src, []byte("%TAG")) {
		return false
	}

	rng := ranges.get(node)

	// Out of context a plain scalar may read differently ("-", "a:"), so fold
	// its text instead of parsing it.
	if node.Kind == yaml.ScalarNode && node.Style&^yaml.TaggedStyle == 0 {
		_, content := ranges.skipProperties(ranges.offset(node.Line, node.Column), node, nil)
		end := ranges.offset(rng.End.Line, rng.End.Column)

		assert.Equal(t, node.Value, foldPlain(string(ranges.src[min(content, end):end])),
			"%s: range %v of plain scalar at %d:%d", name, rng, node.Line, node.Column)

		return true
	}

	var want, got any

	if node.Decode(&want) != nil {
		return false
	}

	covered := sliceRange(string(ranges.src), rng)
	raw := strings.TrimLeft(covered, " ")

	if !endsWithBlockScalar(node) {
		assert.Equal(t, strings.TrimRight(raw, " \t\n"), raw,
			"%s: range %v of node at %d:%d ends in blanks", name, rng, node.Line, node.Column)
	}

	end := ranges.offset(rng.End.Line, rng.End.Column)

	text := covered
	if ranges.at(end, '\n') {
		text += "\n"
	}

	// A mapping without braces reads as one only inside a flow sequence, and
	// a blank after it decides how yaml.v3 reads a trailing ":" ("[a: ]" and
	// "[a:]" differ).
	_, content := ranges.skipProperties(ranges.offset(node.Line, node.Column), node, nil)
	if node.Kind == yaml.MappingNode && node.Style&yaml.FlowStyle != 0 && !ranges.at(content, '{') {
		closing := "]\n"
		if ranges.at(end, ' ') || ranges.at(end, '\t') || ranges.at(end, '\n') {
			closing = " ]\n"
		}

		text = "[" + raw + closing
		want = []any{want}
	}

	if assert.NoError(t, yaml.Unmarshal([]byte(text), &got),
		"%s: range %v of node at %d:%d covers %q", name, rng, node.Line, node.Column, text) {
		assert.Equal(t, want, got, "%s: range %v of node at %d:%d covers %q", name, rng, node.Line, node.Column, text)
	}

	return true
}

// foldPlain joins the lines of a plain scalar the way YAML does: a line break
// between text becomes a space, and each empty line a line feed.
func foldPlain(text string) string {
	lines := strings.Split(text, "\n")

	var out strings.Builder

	out.WriteString(strings.TrimRight(lines[0], " \t"))

	empty := 0

	for _, line := range lines[1:] {
		line = strings.Trim(line, " \t")
		if line == "" {
			empty++

			continue
		}

		if empty == 0 {
			out.WriteByte(' ')
		} else {
			out.WriteString(strings.Repeat("\n", empty))
		}

		empty = 0

		out.WriteString(line)
	}

	return out.String()
}

// endsWithBlockScalar reports whether the text of node ends with a block
// scalar: an implicit null after it adds no text.
func endsWithBlockScalar(node *yaml.Node) bool {
	for i := len(node.Content) - 1; i >= 0; i-- {
		if !isImplicitNull(node.Content[i]) {
			return endsWithBlockScalar(node.Content[i])
		}
	}

	return node.Style&(yaml.LiteralStyle|yaml.FoldedStyle) != 0
}

func hasLineSeparator(node *yaml.Node) bool {
	const lineSeparators = "\xe2\x80\xa8\xe2\x80\xa9" // LS and PS.

	return strings.ContainsAny(node.Value, lineSeparators) || slices.ContainsFunc(node.Content, hasLineSeparator)
}

func hasKeepChomping(node *yaml.Node, ranges *yamlRanges) bool {
	if node.Style&(yaml.LiteralStyle|yaml.FoldedStyle) != 0 {
		_, pos := ranges.skipProperties(ranges.offset(node.Line, node.Column), node, nil)
		header := ranges.src[pos:ranges.lineEnd(pos)]

		if bytes.Contains(header[:min(len(header), 3)], []byte("+")) {
			return true
		}
	}

	return slices.ContainsFunc(node.Content, func(child *yaml.Node) bool {
		return hasKeepChomping(child, ranges)
	})
}

func yamlNodeKind(node *yaml.Node) string {
	switch node.Kind {
	case yaml.MappingNode:
		return "mapping"
	case yaml.SequenceNode:
		return "sequence"
	case yaml.AliasNode:
		return "alias"
	case yaml.ScalarNode:
		return "scalar"
	case yaml.DocumentNode:
		return "document"
	}

	return ""
}

// tree-sitter ends a block scalar at its last non-blank character, while its
// trailing spaces are content and belong to the range.
func endsInBlanksPast(ranges *yamlRanges, want, got tree.Range) bool {
	if want.Start != got.Start {
		return false
	}

	wantEnd := ranges.offset(want.End.Line, want.End.Column)
	gotEnd := ranges.offset(got.End.Line, got.End.Column)

	return wantEnd <= gotEnd && strings.Trim(string(ranges.src[wantEnd:gotEnd]), " \t\n") == ""
}

func positionBefore(a, b tree.Position) bool {
	return a.Line < b.Line || a.Line == b.Line && a.Column < b.Column
}
