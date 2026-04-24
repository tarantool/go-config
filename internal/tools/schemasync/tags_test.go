package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFilterStableTags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    []string
		minMajor int
		want     []string
	}{
		{
			name:     "v-prefixed stable tag is kept and stripped",
			input:    []string{"v3.0.0"},
			minMajor: 3,
			want:     []string{"3.0.0"},
		},
		{
			name:     "plain semver without v prefix is kept as-is",
			input:    []string{"3.7.1"},
			minMajor: 3,
			want:     []string{"3.7.1"},
		},
		{
			name:     "pre-release suffix rc1 is dropped",
			input:    []string{"v3.0.0-rc1"},
			minMajor: 3,
			want:     []string{},
		},
		{
			name:     "entrypoint suffix is dropped",
			input:    []string{"v3.0.0-entrypoint"},
			minMajor: 3,
			want:     []string{},
		},
		{
			name:     "major below minMajor is dropped",
			input:    []string{"v2.11.4"},
			minMajor: 3,
			want:     []string{},
		},
		{
			name:     "major above minMajor is kept",
			input:    []string{"v4.0.0"},
			minMajor: 3,
			want:     []string{"4.0.0"},
		},
		{
			name:     "non-semver tag is dropped",
			input:    []string{"release-3.0"},
			minMajor: 3,
			want:     []string{},
		},
		{
			name:     "empty string is dropped",
			input:    []string{""},
			minMajor: 3,
			want:     []string{},
		},
		{
			name:     "mixed input preserves input order",
			input:    []string{"v3.0.0", "v2.11.4", "3.7.1", "v3.0.0-rc1", "v4.0.0", "release-3.0", ""},
			minMajor: 3,
			want:     []string{"3.0.0", "3.7.1", "4.0.0"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := filterStableTags(tc.input, tc.minMajor)
			assert.Equal(t, tc.want, got)
		})
	}
}
