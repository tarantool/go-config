package tarantool

import (
	"strconv"
	"strings"
)

// compareSemver compares two "X.Y.Z" version strings numerically.
// Returns negative if a < b, zero if a == b, positive if a > b.
// If either string fails to parse as X.Y.Z integers, that string sorts last
// (returns > the parseable one). Two non-parseable strings fall back to
// strings.Compare.
//
//nolint:varnamelen // standard comparator parameter names
func compareSemver(a, b string) int {
	aParts, aOK := parseSemver(a)
	bParts, bOK := parseSemver(b)

	switch {
	case !aOK && !bOK:
		return strings.Compare(a, b)
	case !aOK:
		return 1 // a is non-parseable → sorts last → a > b.
	case !bOK:
		return -1 // b is non-parseable → sorts last → a < b.
	}

	for i := range 3 {
		if aParts[i] != bParts[i] {
			return aParts[i] - bParts[i]
		}
	}

	return 0
}

// parseSemver splits a "X.Y.Z" string into three integer components.
// Returns false if the string does not have exactly three dot-separated
// integer segments.
func parseSemver(v string) ([3]int, bool) {
	const semverParts = 3

	parts := strings.SplitN(v, ".", semverParts+1)
	if len(parts) != semverParts {
		return [3]int{}, false
	}

	var out [3]int

	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return [3]int{}, false
		}

		out[i] = n
	}

	return out, true
}
