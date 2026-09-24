package jsonschema

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tarantool/go-config/v2/keypath"
	"github.com/tarantool/go-config/v2/validator"
)

func TestDropSummaries(t *testing.T) {
	t.Parallel()

	errAt := func(path, code string) validator.ValidationError {
		return validator.ValidationError{
			Path:    keypath.NewKeyPath(path),
			Range:   validator.NewEmptyRange(),
			Code:    code,
			Message: "",
		}
	}

	codes := func(errs []validator.ValidationError) []string {
		out := make([]string, 0, len(errs))
		for _, err := range errs {
			out = append(out, err.Path.String()+" "+err.Code)
		}

		return out
	}

	tests := []struct {
		name string
		errs []validator.ValidationError
		want []string
	}{
		{
			name: "a failure below explains every summary above it",
			errs: []validator.ValidationError{errAt("", "properties"), errAt("a", "$ref"), errAt("a", "minimum")},
			want: []string{"a minimum"},
		},
		{
			name: "a summary explains the summaries above it",
			errs: []validator.ValidationError{errAt("", "properties"), errAt("a", "$ref")},
			want: []string{"a $ref"},
		},
		{
			name: "summaries at one path do not explain each other",
			errs: []validator.ValidationError{errAt("a", "$ref"), errAt("a", "allOf")},
			want: []string{"a $ref", "a allOf"},
		},
		{
			name: "other keywords stay",
			errs: []validator.ValidationError{errAt("a", "required"), errAt("a/b", "type")},
			want: []string{"a required", "a/b type"},
		},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.want, codes(dropSummaries(tt.errs)), tt.name)
	}
}
