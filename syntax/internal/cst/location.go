package cst

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"unicode/utf8"

	sitter "github.com/smacker/go-tree-sitter"
)

// Position is a zero-based location in the original UTF-8 YAML source.
// ByteColumn counts bytes, not Unicode code points.
type Position struct {
	Line       uint32
	ByteColumn uint32
}

// Range is a half-open interval in the original source.
type Range struct {
	Start Position
	End   Position
}

// ErrInvalidPosition identifies a position outside the source or inside a UTF-8 rune.
var ErrInvalidPosition = errors.New("syntax: invalid position")

// LineStarts indexes the byte after each LF, including a trailing empty line.
func LineStarts(source []byte) []int {
	starts := []int{0}

	for offset, character := range source {
		if character == '\n' {
			starts = append(starts, offset+1)
		}
	}

	return starts
}

// Location identifies a CST anchor and the cursor offset in the original source.
// An anchor may precede the cursor. Contains distinguishes positions inside
// a node from insertion points at its end or in surrounding whitespace.
// The node is valid only while its tree is alive.
type Location struct {
	Node   *sitter.Node
	Offset int
}

// Contains reports whether the cursor lies in the node's half-open byte range.
// Completion can additionally accept the end boundary as an insertion point.
func (l Location) Contains(node *sitter.Node) bool {
	return node != nil && l.Offset >= int(node.StartByte()) && l.Offset < int(node.EndByte())
}

// isSpace recognizes YAML separator bytes, not arbitrary Unicode whitespace.
func isSpace(character byte) bool {
	return character == ' ' || character == '\t' || character == '\r' || character == '\n'
}

// Locate validates pos and returns its CST anchor and byte offset.
// In whitespace it may select preceding content; callers must still check
// the cursor line and indentation before treating that anchor as an edit target.
func Locate(root *sitter.Node, source []byte, starts []int, pos sitter.Point) (Location, error) {
	offset, err := SourceOffset(source, starts, pos)
	if err != nil {
		return Location{}, err
	}

	node := root.NamedDescendantForPointRange(pos, pos)

	if node == nil {
		node = root
	}

	for current := node; current != nil; current = current.Parent() {
		// The position before '#' is still an insertion point in YAML.
		if current.Type() == nodeComment && offset == int(current.StartByte()) {
			break
		}

		if IsScalar(current) || current.Type() == nodeComment || current.Type() == "alias" {
			return Location{Node: current, Offset: offset}, nil
		}
	}

	if previous, ok := previousContentPoint(source, starts, offset); ok {
		nearby := root.NamedDescendantForPointRange(previous, previous)
		if nearby != nil && (node.Equal(root) || node.IsError() ||
			node.Type() == nodeComment && offset == int(node.StartByte()) ||
			offset == len(source) || isSpace(source[offset]) || IsScalar(nearby) &&
			int(nearby.EndByte()) == offset) {
			return Location{Node: nearby, Offset: offset}, nil
		}
	}

	return Location{Node: node, Offset: offset}, nil
}

// SourceOffset converts a zero-based byte position to a source offset.
// It rejects out-of-bounds coordinates and split UTF-8 runes.
func SourceOffset(source []byte, starts []int, pos sitter.Point) (int, error) {
	if uint64(pos.Row) >= uint64(len(starts)) {
		return 0, fmt.Errorf("%w: line %d is outside the source", ErrInvalidPosition, pos.Row)
	}

	start := starts[pos.Row]
	if int64(pos.Column) > int64(LineEnd(source, starts, pos.Row)-start) {
		return 0, fmt.Errorf("%w: byte column %d is outside line %d", ErrInvalidPosition, pos.Column, pos.Row)
	}

	offset := start + int(pos.Column)
	if offset < len(source) && !utf8.RuneStart(source[offset]) {
		return 0, fmt.Errorf("%w: byte column %d splits a UTF-8 code point", ErrInvalidPosition, pos.Column)
	}

	return offset, nil
}

// LineEnd returns the byte offset before the LF or CRLF terminator.
// The caller must supply a line already validated by SourceOffset.
func LineEnd(source []byte, starts []int, line uint32) int {
	end := len(source)
	if int(line)+1 < len(starts) {
		end = starts[line+1] - 1
		if end > starts[line] && source[end-1] == '\r' {
			end--
		}
	}

	return end
}

// previousContentPoint finds the preceding non-whitespace rune for CST lookup.
// It returns false when the cursor has no preceding content.
func previousContentPoint(source []byte, starts []int, offset int) (sitter.Point, bool) {
	for offset > 0 {
		_, size := utf8.DecodeLastRune(source[:offset])

		offset -= size

		if isSpace(source[offset]) {
			continue
		}

		line := sort.Search(len(starts), func(index int) bool {
			return starts[index] > offset
		}) - 1

		// Tree-sitter represents source offsets and coordinates with uint32.
		//nolint:gosec // Both coordinates refer to an offset in the syntax tree.
		return sitter.Point{Row: uint32(line), Column: uint32(offset - starts[line])}, true
	}

	return sitter.Point{Row: 0, Column: 0}, false
}

// Newline preserves CRLF when present in the source and otherwise uses LF.
func Newline(source []byte) string {
	if bytes.Contains(source, []byte("\r\n")) {
		return "\r\n"
	}

	return "\n"
}
