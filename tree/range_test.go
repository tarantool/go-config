package tree_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tarantool/go-config/tree"
)

func TestPosition_Structure(t *testing.T) {
	t.Parallel()

	pos := tree.Position{
		Line:   10,
		Column: 5,
	}

	assert.Equal(t, 10, pos.Line)
	assert.Equal(t, 5, pos.Column)
}

func TestRange_Structure(t *testing.T) {
	t.Parallel()

	treeRange := tree.Range{
		Start: tree.Position{Line: 1, Column: 1},
		End:   tree.Position{Line: 10, Column: 20},
	}

	assert.Equal(t, 1, treeRange.Start.Line)
	assert.Equal(t, 1, treeRange.Start.Column)
	assert.Equal(t, 10, treeRange.End.Line)
	assert.Equal(t, 20, treeRange.End.Column)
}

func TestNewRange(t *testing.T) {
	t.Parallel()

	r := tree.NewRange(1, 2, 3, 4)

	assert.Equal(t, 1, r.Start.Line)
	assert.Equal(t, 2, r.Start.Column)
	assert.Equal(t, 3, r.End.Line)
	assert.Equal(t, 4, r.End.Column)
}

func TestNewRange_ValidPositions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		startLine int
		startCol  int
		endLine   int
		endCol    int
	}{
		{"positive values", 10, 5, 20, 10},
		{"zero values", 0, 0, 0, 0},
		{"single line", 5, 1, 5, 100},
		{"same position", 42, 10, 42, 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := tree.NewRange(tt.startLine, tt.startCol, tt.endLine, tt.endCol)

			assert.Equal(t, tt.startLine, r.Start.Line)
			assert.Equal(t, tt.startCol, r.Start.Column)
			assert.Equal(t, tt.endLine, r.End.Line)
			assert.Equal(t, tt.endCol, r.End.Column)
		})
	}
}

func TestNewZeroRange(t *testing.T) {
	t.Parallel()

	r := tree.NewZeroRange()

	assert.Equal(t, tree.Range{
		Start: tree.Position{Line: 0, Column: 0},
		End:   tree.Position{Line: 0, Column: 0},
	}, r)
}

func TestNewRange_Equality(t *testing.T) {
	t.Parallel()

	r1 := tree.NewRange(1, 2, 3, 4)
	r2 := tree.NewRange(1, 2, 3, 4)

	assert.Equal(t, r1, r2)
}

func TestNewRange_Inequality(t *testing.T) {
	t.Parallel()

	r1 := tree.NewRange(1, 2, 3, 4)
	r2 := tree.NewRange(5, 6, 7, 8)

	assert.NotEqual(t, r1, r2)
}
