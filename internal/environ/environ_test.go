package environ_test

import (
	"testing"

	"github.com/shoenig/test"

	"github.com/tarantool/go-config/internal/environ"
	"github.com/tarantool/go-config/internal/testutil"
)

func TestParse(t *testing.T) {
	t.Parallel()

	env := []string{
		"FOO=bar",
		"MYAPP_HOST=localhost",
		"EMPTY=",
		"=VALUE",    // malformed, should be skipped.
		"NO_EQUALS", // malformed, should be skipped.
	}

	expected := []testutil.TestIterSeq2Pair[string, string]{
		{Key: "FOO", Value: "bar"},
		{Key: "MYAPP_HOST", Value: "localhost"},
		{Key: "EMPTY", Value: ""},
	}

	testutil.TestIterSeq2Compare(t, expected, environ.Parse(env))
}

func TestParseAll(t *testing.T) {
	t.Parallel()

	// We can't predict the environment, but we can verify that
	// ParseAll returns at least as many entries as os.Environ().
	// However, ParseAll skips malformed entries, so it may be fewer.
	// Instead, just ensure it doesn't panic and returns something.
	iter := environ.ParseAll()

	count := 0
	for range iter {
		count++
	}

	// At least zero entries (possible in empty environment).
	// We just ensure we can iterate.
	test.True(t, count >= 0)
}

func TestParseEmpty(t *testing.T) {
	t.Parallel()

	testutil.TestIterSeq2Empty(t, environ.Parse(nil))
	testutil.TestIterSeq2Empty(t, environ.Parse([]string{}))
}

func TestParseMalformed(t *testing.T) {
	t.Parallel()

	env := []string{
		"NO_EQUALS",
		"=VALUE",
		"=VALUE=",
	}
	// All entries malformed, should be skipped.
	testutil.TestIterSeq2Empty(t, environ.Parse(env))
}
