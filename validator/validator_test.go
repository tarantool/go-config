package validator_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tarantool/go-config/validator"
)

var _ error = (*validator.ValidationError)(nil)

func TestValidationError_Error(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		path    []string
		code    string
		message string
		expect  string
	}{
		{
			name:    "no path",
			path:    nil,
			code:    "type",
			message: "expected string, got number",
			expect:  "[type] expected string, got number",
		},
		{
			name:    "empty path",
			path:    []string{},
			code:    "required",
			message: "missing field 'name'",
			expect:  "[required] missing field 'name'",
		},
		{
			name:    "single segment path",
			path:    []string{"server"},
			code:    "minimum",
			message: "value must be >= 0",
			expect:  "server [minimum] value must be >= 0",
		},
		{
			name:    "multi segment path",
			path:    []string{"server", "port"},
			code:    "type",
			message: "expected integer, got string",
			expect:  "server/port [type] expected integer, got string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := &validator.ValidationError{
				Path:    tt.path,
				Range:   validator.NewEmptyRange(),
				Code:    tt.code,
				Message: tt.message,
			}
			assert.Equal(t, tt.expect, err.Error())
		})
	}
}

func TestValidationError_ErrorEdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		path    []string
		code    string
		message string
		expect  string
	}{
		{
			name:    "empty code",
			path:    []string{"field"},
			code:    "",
			message: "some message",
			expect:  "field [] some message",
		},
		{
			name:    "empty message",
			path:    []string{"field"},
			code:    "code",
			message: "",
			expect:  "field [code] ",
		},
		{
			name:    "both empty",
			path:    []string{"field"},
			code:    "",
			message: "",
			expect:  "field [] ",
		},
		{
			name:    "path with empty segment",
			path:    []string{"a", "", "b"},
			code:    "type",
			message: "error",
			expect:  "a//b [type] error",
		},
		{
			name:    "path with slash in segment",
			path:    []string{"a/b", "c"},
			code:    "type",
			message: "error",
			expect:  "a/b/c [type] error",
		},
		{
			name:    "no path with empty code and message",
			path:    nil,
			code:    "",
			message: "",
			expect:  "[] ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := &validator.ValidationError{
				Path:    tt.path,
				Range:   validator.NewEmptyRange(),
				Code:    tt.code,
				Message: tt.message,
			}
			assert.Equal(t, tt.expect, err.Error())
		})
	}
}

func TestValidationError_ImplementsError(t *testing.T) {
	t.Parallel()

	var err error = &validator.ValidationError{
		Path:    nil,
		Range:   validator.NewEmptyRange(),
		Code:    "",
		Message: "",
	}

	_ = err // Just ensure assignment compiles.
	require.Error(t, err)
}

func TestPosition_ZeroValues(t *testing.T) {
	t.Parallel()

	pos := validator.Position{Line: 0, Column: 0}
	assert.Equal(t, 0, pos.Line)
	assert.Equal(t, 0, pos.Column)
}

func TestRange_ZeroValues(t *testing.T) {
	t.Parallel()

	r := validator.Range{
		Start: validator.Position{Line: 0, Column: 0},
		End:   validator.Position{Line: 0, Column: 0},
	}
	assert.Equal(t, validator.Position{Line: 0, Column: 0}, r.Start)
	assert.Equal(t, validator.Position{Line: 0, Column: 0}, r.End)
}

func TestNewEmptyRange(t *testing.T) {
	t.Parallel()

	rng := validator.NewEmptyRange()
	assert.Equal(t, validator.Position{Line: 0, Column: 0}, rng.Start)
	assert.Equal(t, validator.Position{Line: 0, Column: 0}, rng.End)
}
