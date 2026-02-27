package environ_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tarantool/go-config/internal/environ"
	"github.com/tarantool/go-config/internal/testutil"
)

func TestParse(t *testing.T) {
	t.Parallel()

	env := []string{
		"FOO=bar",
		"MYAPP_HOST=localhost",
		"EMPTY=",
		"=VALUE",
		"NO_EQUALS",
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

	iter := environ.ParseAll()

	count := 0
	for range iter {
		count++
	}

	assert.GreaterOrEqual(t, count, 0)
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
	testutil.TestIterSeq2Empty(t, environ.Parse(env))
}
