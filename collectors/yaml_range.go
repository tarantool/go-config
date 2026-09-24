package collectors

import (
	"bytes"
	"encoding/binary"
	"sort"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"

	"github.com/tarantool/go-config/v2/tree"
)

// yamlRanges finds where YAML nodes end, which yaml.v3 does not expose.
// Ranges are half-open and include the node's anchor and tag; a node starts
// where yaml.v3 says it does.
type yamlRanges struct {
	src    []byte
	lines  []int
	spans  map[*yaml.Node]yamlSpan
	cursor yamlCursor
}

type yamlSpan struct {
	start, end int
	rng        tree.Range
	// null marks an implicit null: a scalar written as nothing at all.
	null bool
}

// yamlCursor remembers the last converted position, so that walking a long
// line in document order does not recount it from the start every time.
type yamlCursor struct {
	off, line, column int
}

func newYamlRanges(src []byte, doc *yaml.Node) *yamlRanges {
	src = normalizeYamlSource(src)

	ranges := &yamlRanges{
		src:    src,
		lines:  lineStarts(src),
		spans:  make(map[*yaml.Node]yamlSpan, countYamlNodes(doc)),
		cursor: yamlCursor{off: 0, line: 1, column: 1},
	}

	ranges.measure(doc, nil, -1, false)

	return ranges
}

func countYamlNodes(node *yaml.Node) int {
	count := 1
	for _, child := range node.Content {
		count += countYamlNodes(child)
	}

	return count
}

func (r *yamlRanges) get(node *yaml.Node) tree.Range {
	span, ok := r.spans[node]
	if !ok {
		return tree.NewZeroRange()
	}

	return span.rng
}

// next is the node that follows node's subtree in the document, nil at the
// end; indent is the enclosing block collection's indentation, -1 at the
// document level.
func (r *yamlRanges) measure(node, next *yaml.Node, indent int, flow bool) int {
	if node.Line < 1 {
		return 0
	}

	start := r.offset(node.Line, node.Column)

	var end int

	switch node.Kind {
	case yaml.DocumentNode:
		end = start
		for _, child := range node.Content {
			end = r.measure(child, nil, -1, false)
		}
	case yaml.MappingNode, yaml.SequenceNode:
		end = r.collectionEnd(node, next, start, indent, flow)
	case yaml.ScalarNode:
		// yaml.v3 does not mark the non-specific tags "!" and "!<!>" as tags,
		// but a node it starts where the next one does has no text there.
		if isImplicitNull(node) && (!r.at(start, '!') || startsAt(node, next)) {
			// Nothing to measure: yaml.v3 places it on the next token, and
			// its parent moves it next to its key or "-".
			r.setNullSpan(node, start)

			return start
		}

		end = r.scalarEnd(node, next, start, indent, flow)
	case yaml.AliasNode:
		end = r.anchorEnd(start + 1)
	default:
		return start
	}

	end = max(end, start)

	nodeStart := tree.Position{Line: node.Line, Column: node.Column}

	nodeEnd := nodeStart
	if end > start {
		nodeEnd = r.position(end)
	}

	r.spans[node] = yamlSpan{start: start, end: end, rng: tree.Range{Start: nodeStart, End: nodeEnd}, null: false}

	return end
}

// startsAt reports whether node starts where other does.
func startsAt(node, other *yaml.Node) bool {
	return other != nil && node.Line == other.Line && node.Column == other.Column
}

// following returns the node after the i-th child of node, next after the
// last one.
func following(node *yaml.Node, i int, next *yaml.Node) *yaml.Node {
	if i+1 < len(node.Content) {
		return node.Content[i+1]
	}

	return next
}

// setNullSpan records an implicit null with an empty range at off.
func (r *yamlRanges) setNullSpan(node *yaml.Node, off int) {
	pos := r.position(off)

	r.spans[node] = yamlSpan{start: off, end: off, rng: tree.Range{Start: pos, End: pos}, null: true}
}

// isImplicitNull reports whether node looks like a null written as nothing
// at all: no content, anchor or tag that yaml.v3 records.
func isImplicitNull(node *yaml.Node) bool {
	return node.Kind == yaml.ScalarNode && node.Value == "" && node.Style == 0 && node.Anchor == ""
}

func (r *yamlRanges) collectionEnd(node, next *yaml.Node, start, indent int, flow bool) int {
	open := start

	// A collection that starts where its first child does has no brackets or
	// properties of its own: a block mapping, or a mapping without braces in
	// a flow sequence (`[a: b]`).
	if len(node.Content) == 0 || !startsAt(node, node.Content[0]) {
		_, open = r.skipProperties(start, node, following(node, -1, next))

		if r.at(open, '[') || r.at(open, '{') {
			for i, child := range node.Content {
				r.measure(child, following(node, i, next), indent, true)
			}

			return r.flowEnd(r.contentEnd(node, open+1, -1))
		}
	}

	childIndent := r.position(open).Column - 1
	for i, child := range node.Content {
		r.measure(child, following(node, i, next), childIndent, flow)
	}

	if flow {
		return r.contentEnd(node, open, -1)
	}

	return r.contentEnd(node, open, childIndent)
}

// contentEnd returns where the last child of node ends, or end if that is
// later. An implicit null value is moved right after its key and ":", and one
// yaml.v3 did not place right after a "-" does not count. indent is the
// indentation of a block mapping, -1 for a flow one.
func (r *yamlRanges) contentEnd(node *yaml.Node, end, indent int) int {
	for i, child := range node.Content {
		childEnd := r.spans[child].end

		if r.spans[child].null {
			switch start := r.spans[child].start; {
			case node.Kind == yaml.MappingNode && i%2 == 1:
				limit := len(r.src)
				if i+1 < len(node.Content) {
					limit = r.spans[node.Content[i+1]].start
				}

				childEnd = r.colonEnd(r.spans[node.Content[i-1]].end, limit, indent)
				r.setNullSpan(child, childEnd)
			case r.dashBefore(start):
				childEnd = start
			default:
				continue
			}
		}

		end = max(end, childEnd)
	}

	return end
}

// colonEnd returns the offset just past the ":" that follows pos across
// blanks, line breaks and comments, before limit, or pos when another token
// comes first. In a block mapping at indent, a ":" on a later line belongs to
// the mapping only at that indentation; elsewhere it is an enclosing one's.
func (r *yamlRanges) colonEnd(pos, limit, indent int) int {
	next := r.skipSpace(pos)
	if next >= limit || !r.at(next, ':') {
		return pos
	}

	lineStart := bytes.LastIndexByte(r.src[:next], '\n') + 1
	if indent >= 0 && lineStart > pos && next-lineStart != indent {
		return pos
	}

	return next + 1
}

// dashBefore reports whether the nearest character before pos on its line is
// a sequence entry indicator.
func (r *yamlRanges) dashBefore(pos int) bool {
	i := pos - 1
	for i >= 0 && isYamlSpace(r.src[i]) {
		i--
	}

	return r.at(i, '-')
}

func (r *yamlRanges) scalarEnd(node, next *yaml.Node, start, indent int, flow bool) int {
	propsEnd, pos := r.skipProperties(start, node, next)

	switch {
	case node.Style&yaml.DoubleQuotedStyle != 0 && r.at(pos, '"'):
		return r.doubleQuotedEnd(pos)
	case node.Style&yaml.SingleQuotedStyle != 0 && r.at(pos, '\''):
		return r.singleQuotedEnd(pos)
	case node.Style&(yaml.LiteralStyle|yaml.FoldedStyle) != 0 && (r.at(pos, '|') || r.at(pos, '>')):
		return r.blockScalarEnd(pos, indent)
	case node.Value == "":
		return propsEnd
	default:
		return r.plainEnd(pos, indent, flow)
	}
}

// skipProperties returns where the anchor and tag of node, starting at pos,
// end and where its content begins. A node has at most one of each, possibly
// on separate lines, and none where next, the node after it, starts: yaml.v3
// starts a node at its own properties.
func (r *yamlRanges) skipProperties(pos int, node, next *yaml.Node) (int, int) {
	anchor := node.Anchor != ""
	tag := node.Style&yaml.TaggedStyle != 0

	propsEnd, content := pos, pos

	for {
		// Found only for a property past the node's start, which is rare, so
		// that long lines are not walked back to find next.
		if content > pos && next != nil && content >= r.offset(next.Line, next.Column) {
			return propsEnd, content
		}

		switch {
		case anchor && r.at(content, '&'):
			anchor = false
		case r.at(content, '!') && (tag || r.nonSpecificTag(content)):
			tag = false
		default:
			return propsEnd, content
		}

		propsEnd = r.propertyEnd(content)
		content = r.skipSpace(propsEnd)
	}
}

// nonSpecificTag reports whether the tag at pos is "!" or "!<!>", which
// yaml.v3 does not mark as tags.
func (r *yamlRanges) nonSpecificTag(pos int) bool {
	next := pos + 1

	return next == len(r.src) || isYamlSpace(r.src[next]) || r.src[next] == '\n' ||
		bytes.HasPrefix(r.src[pos:], []byte("!<!>"))
}

// propertyEnd returns the end of the anchor or tag at pos, using yaml.v3's
// character sets.
func (r *yamlRanges) propertyEnd(pos int) int {
	if r.at(pos, '&') {
		return r.anchorEnd(pos + 1)
	}

	if r.at(pos+1, '<') {
		if i := bytes.IndexByte(r.src[pos:], '>'); i >= 0 {
			return pos + i + 1
		}
	}

	i := pos + 1
	for i < len(r.src) && (isAnchorChar(r.src[i]) || strings.IndexByte(";/?:@&=+$,.!~*'()[]%", r.src[i]) >= 0) {
		i++
	}

	return i
}

func (r *yamlRanges) anchorEnd(pos int) int {
	for pos < len(r.src) && isAnchorChar(r.src[pos]) {
		pos++
	}

	return pos
}

func (r *yamlRanges) skipSpace(pos int) int {
	for pos < len(r.src) {
		switch char := r.src[pos]; {
		case isYamlSpace(char) || char == '\n':
			pos++
		case char == '#':
			pos = r.lineEnd(pos)
		default:
			return pos
		}
	}

	return pos
}

func (r *yamlRanges) doubleQuotedEnd(pos int) int {
	for i := pos + 1; i < len(r.src); i++ {
		switch r.src[i] {
		case '\\':
			i++
		case '"':
			return i + 1
		}
	}

	return len(r.src)
}

func (r *yamlRanges) singleQuotedEnd(pos int) int {
	for i := pos + 1; i < len(r.src); i++ {
		if r.src[i] != '\'' {
			continue
		}

		if r.at(i+1, '\'') {
			i++

			continue
		}

		return i + 1
	}

	return len(r.src)
}

func (r *yamlRanges) blockScalarEnd(pos, indent int) int {
	i := pos + 1
	explicit := 0

	for i < len(r.src) {
		char := r.src[i]

		if char == '+' || char == '-' {
			i++

			continue
		}

		if char >= '1' && char <= '9' {
			explicit = int(char - '0')
			i++

			continue
		}

		break
	}

	end := i
	first := r.nextLine(i)

	// Indentation as yaml.v3 computes it: a document-level scalar counts from
	// column 0, and without an indicator the widest leading line decides.
	parentIndent := max(indent, 0)
	contentIndent := parentIndent + explicit

	if explicit == 0 {
		contentIndent = parentIndent + 1

		for line := first; line < len(r.src); line = r.nextLine(line) {
			lineIndent, blank := r.indentAt(line)

			contentIndent = max(contentIndent, lineIndent)

			if !blank {
				break
			}
		}
	}

	for line := first; line < len(r.src); line = r.nextLine(line) {
		lineIndent, blank := r.indentAt(line)

		if blank {
			// Whitespace past the content indentation is content.
			if lineIndent >= contentIndent && r.lineEnd(line)-line > contentIndent {
				end = r.lineEnd(line)
			}

			continue
		}

		if lineIndent < contentIndent || r.isDocumentMarker(line) {
			break
		}

		end = r.lineEnd(line)
	}

	return end
}

func (r *yamlRanges) plainEnd(pos, indent int, flow bool) int {
	end, more := r.plainSegment(pos, flow)

	for line := r.nextLine(pos); more && line < len(r.src); line = r.nextLine(line) {
		lineIndent, blank := r.indentAt(line)
		if blank {
			continue
		}

		first := line + lineIndent
		for first < len(r.src) && isYamlSpace(r.src[first]) {
			first++
		}

		// yaml.v3 does not check the indentation of flow continuation lines.
		if !flow && lineIndent <= indent || r.src[first] == '#' || r.isDocumentMarker(line) {
			break
		}

		var segEnd int

		segEnd, more = r.plainSegment(first, flow)
		if segEnd == first {
			break
		}

		end = segEnd
	}

	return end
}

func (r *yamlRanges) plainSegment(pos int, flow bool) (int, bool) {
	end := pos

	for i := pos; i < len(r.src); i++ {
		char := r.src[i]

		switch {
		case char == '\n':
			return end, true
		case char == '#' && i > pos && isYamlSpace(r.src[i-1]):
			return end, false
		case char == ':' && (i+1 == len(r.src) || isYamlSpace(r.src[i+1]) || r.src[i+1] == '\n'):
			return end, false
		case flow && isFlowIndicator(char):
			return end, false
		case !isYamlSpace(char):
			end = i + 1
		}
	}

	return end, true
}

// flowEnd returns the offset just past the bracket that closes a flow
// collection whose last child ends at pos. Only blanks, commas and comments
// can stand in between; anything else leaves pos.
func (r *yamlRanges) flowEnd(pos int) int {
	for i := pos; i < len(r.src); i++ {
		switch char := r.src[i]; {
		case char == ']' || char == '}':
			return i + 1
		case char == '#':
			i = r.lineEnd(i) - 1
		case isYamlSpace(char) || char == '\n' || char == ',':
		default:
			return pos
		}
	}

	return pos
}

func (r *yamlRanges) indentAt(line int) (int, bool) {
	i := line
	for i < len(r.src) && r.src[i] == ' ' {
		i++
	}

	indent := i - line

	for i < len(r.src) && isYamlSpace(r.src[i]) {
		i++
	}

	return indent, i == len(r.src) || r.src[i] == '\n'
}

func (r *yamlRanges) isDocumentMarker(line int) bool {
	rest := r.src[line:]
	if !bytes.HasPrefix(rest, []byte("---")) && !bytes.HasPrefix(rest, []byte("...")) {
		return false
	}

	return len(rest) == 3 || isYamlSpace(rest[3]) || rest[3] == '\n'
}

func (r *yamlRanges) lineEnd(pos int) int {
	if i := bytes.IndexByte(r.src[pos:], '\n'); i >= 0 {
		return pos + i
	}

	return len(r.src)
}

func (r *yamlRanges) nextLine(pos int) int {
	return min(r.lineEnd(pos)+1, len(r.src))
}

func (r *yamlRanges) at(pos int, char byte) bool {
	return pos >= 0 && pos < len(r.src) && r.src[pos] == char
}

func (r *yamlRanges) offset(line, column int) int {
	if line > len(r.lines) {
		return len(r.src)
	}

	off, col := r.lines[line-1], 1
	if r.cursor.line == line && r.cursor.column <= column {
		off, col = r.cursor.off, r.cursor.column
	}

	for ; col < column && off < len(r.src) && r.src[off] != '\n'; col++ {
		_, size := utf8.DecodeRune(r.src[off:])

		off += size
	}

	r.cursor = yamlCursor{off: off, line: line, column: col}

	return off
}

func (r *yamlRanges) position(off int) tree.Position {
	line := sort.Search(len(r.lines), func(i int) bool { return r.lines[i] > off })

	from, col := r.lines[line-1], 1
	if r.cursor.line == line && r.cursor.off <= off {
		from, col = r.cursor.off, r.cursor.column
	}

	col += utf8.RuneCount(r.src[from:off])

	r.cursor = yamlCursor{off: off, line: line, column: col}

	return tree.Position{Line: line, Column: col}
}

func lineStarts(src []byte) []int {
	starts := []int{0}

	for i, char := range src {
		if char == '\n' {
			starts = append(starts, i+1)
		}
	}

	return starts
}

// normalizeYamlSource decodes the source to UTF-8 without a byte order mark
// and turns every line break yaml.v3 recognizes into "\n", so that lines and
// columns match yaml.v3's.
func normalizeYamlSource(src []byte) []byte {
	switch {
	case bytes.HasPrefix(src, []byte("\xff\xfe")):
		src = decodeUTF16(src[2:], binary.LittleEndian)
	case bytes.HasPrefix(src, []byte("\xfe\xff")):
		src = decodeUTF16(src[2:], binary.BigEndian)
	default:
		src = bytes.TrimPrefix(src, []byte("\xef\xbb\xbf"))
	}

	// CRLF, then CR, NEL, LS and PS.
	for _, lineBreak := range []string{"\r\n", "\r", "\xc2\x85", "\xe2\x80\xa8", "\xe2\x80\xa9"} {
		if bytes.Contains(src, []byte(lineBreak)) {
			src = bytes.ReplaceAll(src, []byte(lineBreak), []byte("\n"))
		}
	}

	return src
}

const utf16UnitSize = 2

func decodeUTF16(src []byte, order binary.ByteOrder) []byte {
	units := make([]uint16, len(src)/utf16UnitSize)
	for i := range units {
		units[i] = order.Uint16(src[utf16UnitSize*i:])
	}

	return []byte(string(utf16.Decode(units)))
}

func isAnchorChar(char byte) bool {
	return char >= '0' && char <= '9' || char >= 'A' && char <= 'Z' || char >= 'a' && char <= 'z' ||
		char == '_' || char == '-'
}

func isYamlSpace(char byte) bool {
	return char == ' ' || char == '\t'
}

func isFlowIndicator(char byte) bool {
	return char == ',' || char == '[' || char == ']' || char == '{' || char == '}'
}
