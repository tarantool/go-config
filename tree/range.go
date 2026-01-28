package tree

// Position describes a position in source file.
type Position struct {
	Line   int // Line number (1-based), 0 if unknown.
	Column int // Column number (1-based), 0 if unknown.
}

// Range describes a range in source file for highlighting.
type Range struct {
	Start Position
	End   Position
}

// NewRange creates a Range with given start and end positions.
func NewRange(startLine, startCol, endLine, endCol int) Range {
	return Range{
		Start: Position{Line: startLine, Column: startCol},
		End:   Position{Line: endLine, Column: endCol},
	}
}

// NewZeroRange creates a zero Range (unknown position).
func NewZeroRange() Range {
	return Range{Start: Position{Line: 0, Column: 0}, End: Position{Line: 0, Column: 0}}
}
