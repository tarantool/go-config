package validator

// Range describes a range in source file for highlighting.
type Range struct {
	Start Position
	End   Position
}

// NewTODORange creates a placeholder Range.
//
// To be replaced with real implementation when position tracking is available.
func NewTODORange() Range {
	return Range{
		Start: Position{Line: 0, Column: 0},
		End:   Position{Line: 0, Column: 0},
	}
}
