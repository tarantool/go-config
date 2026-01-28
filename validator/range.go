package validator

import (
	"github.com/tarantool/go-config/tree"
)

// Range describes a range in source file for highlighting.
type Range struct {
	Start Position
	End   Position
}

// NewEmptyRange creates a placeholder Range.
func NewEmptyRange() Range {
	return Range{
		Start: Position{Line: 0, Column: 0},
		End:   Position{Line: 0, Column: 0},
	}
}

// RangeFromTree converts a tree.Range to a validator.Range.
func RangeFromTree(r tree.Range) Range {
	return Range{
		Start: Position{Line: r.Start.Line, Column: r.Start.Column},
		End:   Position{Line: r.End.Line, Column: r.End.Column},
	}
}
