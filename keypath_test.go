package config_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tarantool/go-config"
)

func formatTestName(in string) string {
	return strings.ReplaceAll(in, "/", "_")
}

func TestNewKeyPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected config.KeyPath
	}{
		{"", config.KeyPath{}},
		{"a", config.KeyPath{"a"}},
		{"a/b", config.KeyPath{"a", "b"}},
		{"a/b/c", config.KeyPath{"a", "b", "c"}},
		{"/a/b", config.KeyPath{"", "a", "b"}},
		{"a//c", config.KeyPath{"a", "", "c"}},
		{"a/", config.KeyPath{"a", ""}},
		{"/", config.KeyPath{"", ""}},
		{"//a//b//", config.KeyPath{"", "", "a", "", "b", "", ""}},
	}

	for _, tt := range tests {
		t.Run(formatTestName(tt.input), func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.expected, config.NewKeyPath(tt.input))
			assert.True(t, config.NewKeyPath(tt.input).Equals(tt.expected))
		})
	}
}

func TestNewKeyPathWithDelim(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		delim    string
		expected config.KeyPath
	}{
		{"a.b.c", ".", config.KeyPath{"a", "b", "c"}},
		{"a..c", ".", config.KeyPath{"a", "", "c"}},
		{"a|b", "|", config.KeyPath{"a", "b"}},
		{"", ".", config.KeyPath{}},
		{".a.b.", ".", config.KeyPath{"", "a", "b", ""}},
		{"..a..b..", ".", config.KeyPath{"", "", "a", "", "b", "", ""}},
		{"|a||b|", "|", config.KeyPath{"", "a", "", "b", ""}},
	}

	for _, tt := range tests {
		t.Run(formatTestName(tt.input), func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.expected, config.NewKeyPathWithDelim(tt.input, tt.delim))
			assert.True(t, config.NewKeyPathWithDelim(tt.input, tt.delim).Equals(tt.expected))
		})
	}
}

func TestString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path     config.KeyPath
		expected string
	}{
		{config.KeyPath{}, ""},
		{config.KeyPath{"a"}, "a"},
		{config.KeyPath{"a", "b"}, "a/b"},
		{config.KeyPath{"", "a", "b"}, "/a/b"},
		{config.KeyPath{"a", "", "c"}, "a//c"},
	}

	for _, tt := range tests {
		t.Run(formatTestName(tt.path.String()), func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.expected, tt.path.String())
		})
	}
}

func TestMakeString(t *testing.T) {
	t.Parallel()

	keypath := config.KeyPath{"a", "b", "c"}
	assert.Equal(t, "a.b.c", keypath.MakeString("."))
	assert.Equal(t, "a|b|c", keypath.MakeString("|"))
}

func TestParent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path     config.KeyPath
		expected config.KeyPath
	}{
		{config.KeyPath{}, nil},
		{config.KeyPath{"a"}, nil},
		{config.KeyPath{"a", "b"}, config.KeyPath{"a"}},
		{config.KeyPath{"a", "b", "c"}, config.KeyPath{"a", "b"}},
	}

	for _, tt := range tests {
		t.Run(formatTestName(tt.path.String()), func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.expected, tt.path.Parent())
		})
	}
}

func TestLeaf(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path     config.KeyPath
		expected string
	}{
		{config.KeyPath{}, ""},
		{config.KeyPath{"a"}, "a"},
		{config.KeyPath{"a", "b"}, "b"},
		{config.KeyPath{"a", "b", "c"}, "c"},
	}

	for _, tt := range tests {
		t.Run(tt.path.String(), func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.expected, tt.path.Leaf())
		})
	}
}

func TestAppend(t *testing.T) {
	t.Parallel()

	path := config.KeyPath{"a", "b"}
	got := path.Append("c", "d")

	assert.Equal(t, config.KeyPath{"a", "b", "c", "d"}, got)
	assert.Equal(t, config.KeyPath{"a", "b"}, path)
}

func TestEquals(t *testing.T) {
	t.Parallel()

	tests := []struct {
		a        config.KeyPath
		b        config.KeyPath
		expected bool
	}{
		{config.KeyPath{}, config.KeyPath{}, true},
		{config.KeyPath{"a"}, config.KeyPath{"a"}, true},
		{config.KeyPath{"a", "b"}, config.KeyPath{"a", "b"}, true},
		{config.KeyPath{"a"}, config.KeyPath{"b"}, false},
		{config.KeyPath{"a", "b"}, config.KeyPath{"a"}, false},
		{config.KeyPath{"a", "b"}, config.KeyPath{"a", "c"}, false},
	}

	for _, tt := range tests {
		t.Run(formatTestName(tt.a.String()+"_"+tt.b.String()), func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.expected, tt.a.Equals(tt.b))
		})
	}
}

func TestMatch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path     config.KeyPath
		pattern  config.KeyPath
		expected bool
	}{
		{config.KeyPath{"a", "b", "c"}, config.KeyPath{"a", "b", "c"}, true},
		{config.KeyPath{"a", "b", "c"}, config.KeyPath{"a", "b", "d"}, false},
		{config.KeyPath{"a", "b", "c"}, config.KeyPath{"a", "*", "c"}, true},
		{config.KeyPath{"a", "x", "c"}, config.KeyPath{"a", "*", "c"}, true},
		{config.KeyPath{"a", "b", "c"}, config.KeyPath{"*", "b", "c"}, true},
		{config.KeyPath{"a", "b", "c"}, config.KeyPath{"a", "b", "*"}, true},
		{config.KeyPath{"a", "b", "c"}, config.KeyPath{"*", "*", "*"}, true},
		{config.KeyPath{"a", "b", "c"}, config.KeyPath{"a", "", "c"}, false},
		{config.KeyPath{"a", "x", "c"}, config.KeyPath{"a", "", "c"}, false},
		{config.KeyPath{"a", "b", "c"}, config.KeyPath{"", "b", "c"}, false},
		{config.KeyPath{"a", "b", "c"}, config.KeyPath{"a", "b", ""}, false},
		{config.KeyPath{"a", "b", "c"}, config.KeyPath{"", "", ""}, false},
		{config.KeyPath{"a", "b", "c"}, config.KeyPath{"a", "b"}, true},
		{config.KeyPath{"a", "b", "c", "d"}, config.KeyPath{"a", "b"}, true},
		{config.KeyPath{"a", "b", "c", "d"}, config.KeyPath{"a", "*"}, true},
		{config.KeyPath{"a", "b", "c", "d"}, config.KeyPath{"*", "b"}, true},
		{config.KeyPath{"a", "b", "c", "d"}, config.KeyPath{"a", "*", "c"}, true},
		{config.KeyPath{"a", "b"}, config.KeyPath{"a", "b", "c"}, false},
		{config.KeyPath{"a", "b"}, config.KeyPath{"a", "b", "*"}, false},
		{config.KeyPath{"a", "b", "c", "d"}, config.KeyPath{"a", "*", "c", "*"}, true},
		{config.KeyPath{"a", "x", "c", "y"}, config.KeyPath{"a", "*", "c", "*"}, true},
		{config.KeyPath{"a", "x", "c", "y"}, config.KeyPath{"a", "*", "d", "*"}, false},
		{config.KeyPath{}, config.KeyPath{}, true},
		{config.KeyPath{}, config.KeyPath{""}, false},
		{config.KeyPath{""}, config.KeyPath{}, true},
		{config.KeyPath{"a"}, config.KeyPath{}, true},
		{config.KeyPath{"groups", "storages", "replicasets", "storage-001", "instances", "storage-001-a"},
			config.KeyPath{"groups", "*", "replicasets", "*", "instances"}, true},
		{config.KeyPath{"groups", "storages", "replicasets", "storage-001", "instances"},
			config.KeyPath{"groups", "*", "replicasets", "*", "instances"}, true},
		{config.KeyPath{"groups", "storages", "replicasets", "storage-001"},
			config.KeyPath{"groups", "*", "replicasets", "*", "instances"}, false},
		{config.KeyPath{"a", "c", "d", "b"}, config.KeyPath{"a", "*", "b"}, false},
		{config.KeyPath{"a", "b"}, config.KeyPath{"a", "*", "b"}, false},
		{config.KeyPath{"a"}, config.KeyPath{"a", "*"}, false},
		{config.KeyPath{"a", "b"}, config.KeyPath{"*", "b"}, true},
		{config.KeyPath{"a", "b", "c"}, config.KeyPath{"*", "b", "*"}, true},
	}

	for _, tt := range tests {
		t.Run(formatTestName(tt.path.String()+"_"+tt.pattern.String()), func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.expected, tt.path.Match(tt.pattern))
		})
	}
}

func TestHasEmptySegment(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path     config.KeyPath
		expected bool
	}{
		{config.KeyPath{}, false},
		{config.KeyPath{"a"}, false},
		{config.KeyPath{"a", "b", "c"}, false},
		{config.KeyPath{"a", "", "c"}, true},
		{config.KeyPath{"", "a", "b"}, true},
		{config.KeyPath{"a", "b", ""}, true},
		{config.KeyPath{"", "", ""}, true},
		{config.KeyPath{"", "a", "", "b", ""}, true},
		{config.NewKeyPath("a//c"), true},
		{config.NewKeyPath("/a/b"), true},
		{config.NewKeyPath("a/"), true},
		{config.NewKeyPath("//a//b//"), true},
	}

	for _, tt := range tests {
		t.Run(formatTestName(tt.path.String()), func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.expected, tt.path.HasEmptySegment())
		})
	}
}

func TestMatchDoubleStar(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path     config.KeyPath
		pattern  config.KeyPath
		expected bool
	}{
		{config.KeyPath{"a", "b"}, config.KeyPath{"a", "**", "b"}, true},
		{config.KeyPath{"a", "x", "b"}, config.KeyPath{"a", "**", "b"}, true},
		{config.KeyPath{"a", "x", "y", "z", "b"}, config.KeyPath{"a", "**", "b"}, true},
		{config.KeyPath{"a", "b", "c"}, config.KeyPath{"a", "**"}, true},
		{config.KeyPath{"a", "b", "c"}, config.KeyPath{"**", "c"}, true},
		{config.KeyPath{"a", "b", "c", "d"}, config.KeyPath{"a", "*", "**", "d"}, true},
		{config.KeyPath{"a", "b", "c", "d", "e"}, config.KeyPath{"a", "**", "c", "**", "e"}, true},
		{config.KeyPath{"a", "b"}, config.KeyPath{"a", "**", "c"}, false},
	}

	for _, tt := range tests {
		t.Run(formatTestName(tt.path.String()+"_"+tt.pattern.String()), func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, tt.path.Match(tt.pattern))
		})
	}
}
