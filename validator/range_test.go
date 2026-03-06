package validator_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tarantool/go-config/tree"
	"github.com/tarantool/go-config/validator"
)

func TestRangeFromTree_Basic(t *testing.T) {
	t.Parallel()

	treeRange := tree.NewRange(10, 5, 20, 15)

	vRange := validator.RangeFromTree(treeRange)

	assert.Equal(t, 10, vRange.Start.Line)
	assert.Equal(t, 5, vRange.Start.Column)
	assert.Equal(t, 20, vRange.End.Line)
	assert.Equal(t, 15, vRange.End.Column)
}

func TestRangeFromTree_ZeroRange(t *testing.T) {
	t.Parallel()

	treeRange := tree.NewZeroRange()

	vRange := validator.RangeFromTree(treeRange)

	assert.Equal(t, validator.Range{
		Start: validator.Position{Line: 0, Column: 0},
		End:   validator.Position{Line: 0, Column: 0},
	}, vRange)
}

func TestRangeFromTree_VariousPositions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		startLine int
		startCol  int
		endLine   int
		endCol    int
	}{
		{"positive positions", 10, 5, 20, 15},
		{"zero positions", 0, 0, 0, 0},
		{"same line", 5, 1, 5, 100},
		{"same position", 42, 10, 42, 10},
		{"large values", 1000, 500, 2000, 1000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			treeRange := tree.NewRange(tt.startLine, tt.startCol, tt.endLine, tt.endCol)

			vRange := validator.RangeFromTree(treeRange)

			assert.Equal(t, tt.startLine, vRange.Start.Line)
			assert.Equal(t, tt.startCol, vRange.Start.Column)
			assert.Equal(t, tt.endLine, vRange.End.Line)
			assert.Equal(t, tt.endCol, vRange.End.Column)
		})
	}
}

func TestPosition_Structure(t *testing.T) {
	t.Parallel()

	pos := validator.Position{
		Line:   100,
		Column: 50,
	}

	assert.Equal(t, 100, pos.Line)
	assert.Equal(t, 50, pos.Column)
}

func TestRange_Structure(t *testing.T) {
	t.Parallel()

	validatorRange := validator.Range{
		Start: validator.Position{Line: 5, Column: 10},
		End:   validator.Position{Line: 15, Column: 20},
	}

	assert.Equal(t, 5, validatorRange.Start.Line)
	assert.Equal(t, 10, validatorRange.Start.Column)
	assert.Equal(t, 15, validatorRange.End.Line)
	assert.Equal(t, 20, validatorRange.End.Column)
}
