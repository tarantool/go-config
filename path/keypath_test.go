package path_test

import (
	"strings"
	"testing"

	"github.com/shoenig/test"

	"github.com/tarantool/go-config/path"
)

func formatTestName(in string) string {
	return strings.ReplaceAll(in, "/", "_")
}

func TestNewKeyPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected path.KeyPath
	}{
		{"", path.KeyPath{}},
		{"a", path.KeyPath{"a"}},
		{"a/b", path.KeyPath{"a", "b"}},
		{"a/b/c", path.KeyPath{"a", "b", "c"}},
		{"/a/b", path.KeyPath{"", "a", "b"}},
		{"a//c", path.KeyPath{"a", "", "c"}},
		{"a/", path.KeyPath{"a", ""}},
		{"/", path.KeyPath{"", ""}},
		{"//a//b//", path.KeyPath{"", "", "a", "", "b", "", ""}},
	}

	for _, tt := range tests {
		t.Run(formatTestName(tt.input), func(t *testing.T) {
			t.Parallel()

			test.Eq(t, tt.expected, path.NewKeyPath(tt.input))
			// Equal method works the same way as eq comparison.
			test.True(t, path.NewKeyPath(tt.input).Equals(tt.expected))
		})
	}
}

func TestNewKeyPathWithDelim(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		delim    string
		expected path.KeyPath
	}{
		{"a.b.c", ".", path.KeyPath{"a", "b", "c"}},
		{"a..c", ".", path.KeyPath{"a", "", "c"}},
		{"a|b", "|", path.KeyPath{"a", "b"}},
		{"", ".", path.KeyPath{}},
		{".a.b.", ".", path.KeyPath{"", "a", "b", ""}},
		{"..a..b..", ".", path.KeyPath{"", "", "a", "", "b", "", ""}},
		{"|a||b|", "|", path.KeyPath{"", "a", "", "b", ""}},
	}

	for _, tt := range tests {
		t.Run(formatTestName(tt.input), func(t *testing.T) {
			t.Parallel()

			test.Eq(t, tt.expected, path.NewKeyPathWithDelim(tt.input, tt.delim))
			// Equal method works the same way as eq comparison.
			test.True(t, path.NewKeyPathWithDelim(tt.input, tt.delim).Equals(tt.expected))
		})
	}
}

func TestString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path     path.KeyPath
		expected string
	}{
		{path.KeyPath{}, ""},
		{path.KeyPath{"a"}, "a"},
		{path.KeyPath{"a", "b"}, "a/b"},
		{path.KeyPath{"", "a", "b"}, "/a/b"},
		{path.KeyPath{"a", "", "c"}, "a//c"},
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

	kp := path.KeyPath{"a", "b", "c"}
	test.Eq(t, "a.b.c", kp.MakeString("."))
	test.Eq(t, "a|b|c", kp.MakeString("|"))
}

func TestParent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path     path.KeyPath
		expected path.KeyPath
	}{
		{path.KeyPath{}, nil},
		{path.KeyPath{"a"}, nil},
		{path.KeyPath{"a", "b"}, path.KeyPath{"a"}},
		{path.KeyPath{"a", "b", "c"}, path.KeyPath{"a", "b"}},
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
		path     path.KeyPath
		expected string
	}{
		{path.KeyPath{}, ""},
		{path.KeyPath{"a"}, "a"},
		{path.KeyPath{"a", "b"}, "b"},
		{path.KeyPath{"a", "b", "c"}, "c"},
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

	kp := path.KeyPath{"a", "b"}
	got := kp.Append("c", "d")

	test.Eq(t, path.KeyPath{"a", "b", "c", "d"}, got)
	// Original KeyPath is unchanged.
	test.Eq(t, path.KeyPath{"a", "b"}, kp)
}

func TestEquals(t *testing.T) {
	t.Parallel()

	tests := []struct {
		a        path.KeyPath
		b        path.KeyPath
		expected bool
	}{
		{path.KeyPath{}, path.KeyPath{}, true},
		{path.KeyPath{"a"}, path.KeyPath{"a"}, true},
		{path.KeyPath{"a", "b"}, path.KeyPath{"a", "b"}, true},
		{path.KeyPath{"a"}, path.KeyPath{"b"}, false},
		{path.KeyPath{"a", "b"}, path.KeyPath{"a"}, false},
		{path.KeyPath{"a", "b"}, path.KeyPath{"a", "c"}, false},
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
		path     path.KeyPath
		pattern  path.KeyPath
		expected bool
	}{
		// Exact matches.
		{path.KeyPath{"a", "b", "c"}, path.KeyPath{"a", "b", "c"}, true},
		{path.KeyPath{"a", "b", "c"}, path.KeyPath{"a", "b", "d"}, false},
		// Wildcard as "*".
		{path.KeyPath{"a", "b", "c"}, path.KeyPath{"a", "*", "c"}, true},
		{path.KeyPath{"a", "x", "c"}, path.KeyPath{"a", "*", "c"}, true},
		{path.KeyPath{"a", "b", "c"}, path.KeyPath{"*", "b", "c"}, true},
		{path.KeyPath{"a", "b", "c"}, path.KeyPath{"a", "b", "*"}, true},
		{path.KeyPath{"a", "b", "c"}, path.KeyPath{"*", "*", "*"}, true},
		// Empty segment is NOT a wildcard.
		{path.KeyPath{"a", "b", "c"}, path.KeyPath{"a", "", "c"}, false},
		{path.KeyPath{"a", "x", "c"}, path.KeyPath{"a", "", "c"}, false},
		{path.KeyPath{"a", "b", "c"}, path.KeyPath{"", "b", "c"}, false},
		{path.KeyPath{"a", "b", "c"}, path.KeyPath{"a", "b", ""}, false},
		{path.KeyPath{"a", "b", "c"}, path.KeyPath{"", "", ""}, false},
		// Length mismatch: pattern shorter than path (prefix match).
		{path.KeyPath{"a", "b", "c"}, path.KeyPath{"a", "b"}, true},
		{path.KeyPath{"a", "b", "c", "d"}, path.KeyPath{"a", "b"}, true},
		{path.KeyPath{"a", "b", "c", "d"}, path.KeyPath{"a", "*"}, true},
		{path.KeyPath{"a", "b", "c", "d"}, path.KeyPath{"*", "b"}, true},
		{path.KeyPath{"a", "b", "c", "d"}, path.KeyPath{"a", "*", "c"}, true},
		// Length mismatch: pattern longer than path.
		{path.KeyPath{"a", "b"}, path.KeyPath{"a", "b", "c"}, false},
		{path.KeyPath{"a", "b"}, path.KeyPath{"a", "b", "*"}, false},
		// Multiple wildcards.
		{path.KeyPath{"a", "b", "c", "d"}, path.KeyPath{"a", "*", "c", "*"}, true},
		{path.KeyPath{"a", "x", "c", "y"}, path.KeyPath{"a", "*", "c", "*"}, true},
		{path.KeyPath{"a", "x", "c", "y"}, path.KeyPath{"a", "*", "d", "*"}, false},
		// Empty path and pattern.
		{path.KeyPath{}, path.KeyPath{}, true},
		{path.KeyPath{}, path.KeyPath{""}, false},
		{path.KeyPath{""}, path.KeyPath{}, true},
		{path.KeyPath{"a"}, path.KeyPath{}, true},
		// RFC example: pattern "/groups/*/replicasets/*/instances" should match path with extra segments.
		{path.KeyPath{"groups", "storages", "replicasets", "storage-001", "instances", "storage-001-a"},
			path.KeyPath{"groups", "*", "replicasets", "*", "instances"}, true},
		{path.KeyPath{"groups", "storages", "replicasets", "storage-001", "instances"},
			path.KeyPath{"groups", "*", "replicasets", "*", "instances"}, true},
		{path.KeyPath{"groups", "storages", "replicasets", "storage-001"},
			path.KeyPath{"groups", "*", "replicasets", "*", "instances"}, false},
		// Wildcard * should match exactly one segment.
		{path.KeyPath{"a", "c", "d", "b"}, path.KeyPath{"a", "*", "b"}, false},
		{path.KeyPath{"a", "b"}, path.KeyPath{"a", "*", "b"}, false},
		{path.KeyPath{"a"}, path.KeyPath{"a", "*"}, false},
		{path.KeyPath{"a", "b"}, path.KeyPath{"*", "b"}, true},
		{path.KeyPath{"a", "b", "c"}, path.KeyPath{"*", "b", "*"}, true},
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
		path     path.KeyPath
		expected bool
	}{
		{path.KeyPath{}, false},
		{path.KeyPath{"a"}, false},
		{path.KeyPath{"a", "b", "c"}, false},
		{path.KeyPath{"a", "", "c"}, true},
		{path.KeyPath{"", "a", "b"}, true},
		{path.KeyPath{"a", "b", ""}, true},
		{path.KeyPath{"", "", ""}, true},
		{path.KeyPath{"", "a", "", "b", ""}, true},
		{path.NewKeyPath("a//c"), true},
		{path.NewKeyPath("/a/b"), true},
		{path.NewKeyPath("a/"), true},
		{path.NewKeyPath("//a//b//"), true},
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
		path     path.KeyPath
		pattern  path.KeyPath
		expected bool
	}{
		{path.KeyPath{"a", "b"}, path.KeyPath{"a", "**", "b"}, true},
		{path.KeyPath{"a", "x", "b"}, path.KeyPath{"a", "**", "b"}, true},
		{path.KeyPath{"a", "x", "y", "z", "b"}, path.KeyPath{"a", "**", "b"}, true},
		{path.KeyPath{"a", "b", "c"}, path.KeyPath{"a", "**"}, true},
		{path.KeyPath{"a", "b", "c"}, path.KeyPath{"**", "c"}, true},
		{path.KeyPath{"a", "b", "c", "d"}, path.KeyPath{"a", "*", "**", "d"}, true},
		{path.KeyPath{"a", "b", "c", "d", "e"}, path.KeyPath{"a", "**", "c", "**", "e"}, true},
		{path.KeyPath{"a", "b"}, path.KeyPath{"a", "**", "c"}, false},
	}

	for _, tt := range tests {
		t.Run(formatTestName(tt.path.String()+"_"+tt.pattern.String()), func(t *testing.T) {
			t.Parallel()
			test.Eq(t, tt.expected, tt.path.Match(tt.pattern))
		})
	}
}
