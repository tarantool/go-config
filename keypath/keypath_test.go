package keypath_test

import (
	"strings"
	"testing"

	"github.com/shoenig/test"

	"github.com/tarantool/go-config/keypath"
)

func formatTestName(in string) string {
	return strings.ReplaceAll(in, "/", "_")
}

func TestNewKeyPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected keypath.KeyPath
	}{
		{"", keypath.KeyPath{}},
		{"a", keypath.KeyPath{"a"}},
		{"a/b", keypath.KeyPath{"a", "b"}},
		{"a/b/c", keypath.KeyPath{"a", "b", "c"}},
		{"/a/b", keypath.KeyPath{"", "a", "b"}},
		{"a//c", keypath.KeyPath{"a", "", "c"}},
		{"a/", keypath.KeyPath{"a", ""}},
		{"/", keypath.KeyPath{"", ""}},
		{"//a//b//", keypath.KeyPath{"", "", "a", "", "b", "", ""}},
	}

	for _, tt := range tests {
		t.Run(formatTestName(tt.input), func(t *testing.T) {
			t.Parallel()

			test.Eq(t, tt.expected, keypath.NewKeyPath(tt.input))
			// Equal method works the same way as eq comparison.
			test.True(t, keypath.NewKeyPath(tt.input).Equals(tt.expected))
		})
	}
}

func TestNewKeyPathWithDelim(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		delim    string
		expected keypath.KeyPath
	}{
		{"a.b.c", ".", keypath.KeyPath{"a", "b", "c"}},
		{"a..c", ".", keypath.KeyPath{"a", "", "c"}},
		{"a|b", "|", keypath.KeyPath{"a", "b"}},
		{"", ".", keypath.KeyPath{}},
		{".a.b.", ".", keypath.KeyPath{"", "a", "b", ""}},
		{"..a..b..", ".", keypath.KeyPath{"", "", "a", "", "b", "", ""}},
		{"|a||b|", "|", keypath.KeyPath{"", "a", "", "b", ""}},
	}

	for _, tt := range tests {
		t.Run(formatTestName(tt.input), func(t *testing.T) {
			t.Parallel()

			test.Eq(t, tt.expected, keypath.NewKeyPathWithDelim(tt.input, tt.delim))
			// Equal method works the same way as eq comparison.
			test.True(t, keypath.NewKeyPathWithDelim(tt.input, tt.delim).Equals(tt.expected))
		})
	}
}

func TestString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path     keypath.KeyPath
		expected string
	}{
		{keypath.KeyPath{}, ""},
		{keypath.KeyPath{"a"}, "a"},
		{keypath.KeyPath{"a", "b"}, "a/b"},
		{keypath.KeyPath{"", "a", "b"}, "/a/b"},
		{keypath.KeyPath{"a", "", "c"}, "a//c"},
	}

	for _, tt := range tests {
		t.Run(formatTestName(tt.path.String()), func(t *testing.T) {
			t.Parallel()

			test.Eq(t, tt.expected, tt.path.String())
		})
	}
}

func TestMakeString(t *testing.T) {
	t.Parallel()

	kp := keypath.KeyPath{"a", "b", "c"}
	test.Eq(t, "a.b.c", kp.MakeString("."))
	test.Eq(t, "a|b|c", kp.MakeString("|"))
}

func TestParent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path     keypath.KeyPath
		expected keypath.KeyPath
	}{
		{keypath.KeyPath{}, nil},
		{keypath.KeyPath{"a"}, nil},
		{keypath.KeyPath{"a", "b"}, keypath.KeyPath{"a"}},
		{keypath.KeyPath{"a", "b", "c"}, keypath.KeyPath{"a", "b"}},
	}

	for _, tt := range tests {
		t.Run(formatTestName(tt.path.String()), func(t *testing.T) {
			t.Parallel()

			test.Eq(t, tt.expected, tt.path.Parent())
		})
	}
}

func TestLeaf(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path     keypath.KeyPath
		expected string
	}{
		{keypath.KeyPath{}, ""},
		{keypath.KeyPath{"a"}, "a"},
		{keypath.KeyPath{"a", "b"}, "b"},
		{keypath.KeyPath{"a", "b", "c"}, "c"},
	}

	for _, tt := range tests {
		t.Run(tt.path.String(), func(t *testing.T) {
			t.Parallel()

			test.Eq(t, tt.expected, tt.path.Leaf())
		})
	}
}

func TestAppend(t *testing.T) {
	t.Parallel()

	kp := keypath.KeyPath{"a", "b"}
	got := kp.Append("c", "d")

	test.Eq(t, keypath.KeyPath{"a", "b", "c", "d"}, got)
	// Original KeyPath is unchanged.
	test.Eq(t, keypath.KeyPath{"a", "b"}, kp)
}

func TestEquals(t *testing.T) {
	t.Parallel()

	tests := []struct {
		a        keypath.KeyPath
		b        keypath.KeyPath
		expected bool
	}{
		{keypath.KeyPath{}, keypath.KeyPath{}, true},
		{keypath.KeyPath{"a"}, keypath.KeyPath{"a"}, true},
		{keypath.KeyPath{"a", "b"}, keypath.KeyPath{"a", "b"}, true},
		{keypath.KeyPath{"a"}, keypath.KeyPath{"b"}, false},
		{keypath.KeyPath{"a", "b"}, keypath.KeyPath{"a"}, false},
		{keypath.KeyPath{"a", "b"}, keypath.KeyPath{"a", "c"}, false},
	}

	for _, tt := range tests {
		t.Run(formatTestName(tt.a.String()+"_"+tt.b.String()), func(t *testing.T) {
			t.Parallel()

			test.Eq(t, tt.expected, tt.a.Equals(tt.b))
		})
	}
}

func TestMatch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path     keypath.KeyPath
		pattern  keypath.KeyPath
		expected bool
	}{
		// Exact matches.
		{keypath.KeyPath{"a", "b", "c"}, keypath.KeyPath{"a", "b", "c"}, true},
		{keypath.KeyPath{"a", "b", "c"}, keypath.KeyPath{"a", "b", "d"}, false},
		// Wildcard as "*".
		{keypath.KeyPath{"a", "b", "c"}, keypath.KeyPath{"a", "*", "c"}, true},
		{keypath.KeyPath{"a", "x", "c"}, keypath.KeyPath{"a", "*", "c"}, true},
		{keypath.KeyPath{"a", "b", "c"}, keypath.KeyPath{"*", "b", "c"}, true},
		{keypath.KeyPath{"a", "b", "c"}, keypath.KeyPath{"a", "b", "*"}, true},
		{keypath.KeyPath{"a", "b", "c"}, keypath.KeyPath{"*", "*", "*"}, true},
		// Empty segment is NOT a wildcard.
		{keypath.KeyPath{"a", "b", "c"}, keypath.KeyPath{"a", "", "c"}, false},
		{keypath.KeyPath{"a", "x", "c"}, keypath.KeyPath{"a", "", "c"}, false},
		{keypath.KeyPath{"a", "b", "c"}, keypath.KeyPath{"", "b", "c"}, false},
		{keypath.KeyPath{"a", "b", "c"}, keypath.KeyPath{"a", "b", ""}, false},
		{keypath.KeyPath{"a", "b", "c"}, keypath.KeyPath{"", "", ""}, false},
		// Length mismatch: pattern shorter than path (prefix match).
		{keypath.KeyPath{"a", "b", "c"}, keypath.KeyPath{"a", "b"}, true},
		{keypath.KeyPath{"a", "b", "c", "d"}, keypath.KeyPath{"a", "b"}, true},
		{keypath.KeyPath{"a", "b", "c", "d"}, keypath.KeyPath{"a", "*"}, true},
		{keypath.KeyPath{"a", "b", "c", "d"}, keypath.KeyPath{"*", "b"}, true},
		{keypath.KeyPath{"a", "b", "c", "d"}, keypath.KeyPath{"a", "*", "c"}, true},
		// Length mismatch: pattern longer than keypath.
		{keypath.KeyPath{"a", "b"}, keypath.KeyPath{"a", "b", "c"}, false},
		{keypath.KeyPath{"a", "b"}, keypath.KeyPath{"a", "b", "*"}, false},
		// Multiple wildcards.
		{keypath.KeyPath{"a", "b", "c", "d"}, keypath.KeyPath{"a", "*", "c", "*"}, true},
		{keypath.KeyPath{"a", "x", "c", "y"}, keypath.KeyPath{"a", "*", "c", "*"}, true},
		{keypath.KeyPath{"a", "x", "c", "y"}, keypath.KeyPath{"a", "*", "d", "*"}, false},
		// Empty path and pattern.
		{keypath.KeyPath{}, keypath.KeyPath{}, true},
		{keypath.KeyPath{}, keypath.KeyPath{""}, false},
		{keypath.KeyPath{""}, keypath.KeyPath{}, true},
		{keypath.KeyPath{"a"}, keypath.KeyPath{}, true},
		// RFC example: pattern "/groups/*/replicasets/*/instances" should match path with extra segments.
		{keypath.KeyPath{"groups", "storages", "replicasets", "storage-001", "instances", "storage-001-a"},
			keypath.KeyPath{"groups", "*", "replicasets", "*", "instances"}, true},
		{keypath.KeyPath{"groups", "storages", "replicasets", "storage-001", "instances"},
			keypath.KeyPath{"groups", "*", "replicasets", "*", "instances"}, true},
		{keypath.KeyPath{"groups", "storages", "replicasets", "storage-001"},
			keypath.KeyPath{"groups", "*", "replicasets", "*", "instances"}, false},
		// Wildcard * should match exactly one segment.
		{keypath.KeyPath{"a", "c", "d", "b"}, keypath.KeyPath{"a", "*", "b"}, false},
		{keypath.KeyPath{"a", "b"}, keypath.KeyPath{"a", "*", "b"}, false},
		{keypath.KeyPath{"a"}, keypath.KeyPath{"a", "*"}, false},
		{keypath.KeyPath{"a", "b"}, keypath.KeyPath{"*", "b"}, true},
		{keypath.KeyPath{"a", "b", "c"}, keypath.KeyPath{"*", "b", "*"}, true},
	}

	for _, tt := range tests {
		t.Run(formatTestName(tt.path.String()+"_"+tt.pattern.String()), func(t *testing.T) {
			t.Parallel()

			test.Eq(t, tt.expected, tt.path.Match(tt.pattern))
		})
	}
}

func TestHasEmptySegment(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path     keypath.KeyPath
		expected bool
	}{
		{keypath.KeyPath{}, false},
		{keypath.KeyPath{"a"}, false},
		{keypath.KeyPath{"a", "b", "c"}, false},
		{keypath.KeyPath{"a", "", "c"}, true},
		{keypath.KeyPath{"", "a", "b"}, true},
		{keypath.KeyPath{"a", "b", ""}, true},
		{keypath.KeyPath{"", "", ""}, true},
		{keypath.KeyPath{"", "a", "", "b", ""}, true},
		{keypath.NewKeyPath("a//c"), true},
		{keypath.NewKeyPath("/a/b"), true},
		{keypath.NewKeyPath("a/"), true},
		{keypath.NewKeyPath("//a//b//"), true},
	}

	for _, tt := range tests {
		t.Run(formatTestName(tt.path.String()), func(t *testing.T) {
			t.Parallel()

			test.Eq(t, tt.expected, tt.path.HasEmptySegment())
		})
	}
}

func TestMatchDoubleStar(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path     keypath.KeyPath
		pattern  keypath.KeyPath
		expected bool
	}{
		{keypath.KeyPath{"a", "b"}, keypath.KeyPath{"a", "**", "b"}, true},
		{keypath.KeyPath{"a", "x", "b"}, keypath.KeyPath{"a", "**", "b"}, true},
		{keypath.KeyPath{"a", "x", "y", "z", "b"}, keypath.KeyPath{"a", "**", "b"}, true},
		{keypath.KeyPath{"a", "b", "c"}, keypath.KeyPath{"a", "**"}, true},
		{keypath.KeyPath{"a", "b", "c"}, keypath.KeyPath{"**", "c"}, true},
		{keypath.KeyPath{"a", "b", "c", "d"}, keypath.KeyPath{"a", "*", "**", "d"}, true},
		{keypath.KeyPath{"a", "b", "c", "d", "e"}, keypath.KeyPath{"a", "**", "c", "**", "e"}, true},
		{keypath.KeyPath{"a", "b"}, keypath.KeyPath{"a", "**", "c"}, false},
	}

	for _, tt := range tests {
		t.Run(formatTestName(tt.path.String()+"_"+tt.pattern.String()), func(t *testing.T) {
			t.Parallel()
			test.Eq(t, tt.expected, tt.path.Match(tt.pattern))
		})
	}
}
