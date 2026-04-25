package tarantool_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tarantool/go-config"
	"github.com/tarantool/go-config/tarantool"
)

func TestBuild_EnvIgnore_ExactMatch(t *testing.T) {
	t.Setenv("TT_FOO", "x")

	ctx := context.Background()

	cfg, err := tarantool.New().
		WithoutSchema().
		WithEnvIgnore("TT_FOO").
		Build(ctx)
	require.NoError(t, err)

	_, ok := cfg.Lookup(config.NewKeyPath("foo"))
	assert.False(t, ok, "exact-name ignore should drop the var")
}

func TestBuild_EnvIgnore_GlobMatch(t *testing.T) {
	t.Setenv("TT_CLI_REPO_ROCKS", "x")

	ctx := context.Background()

	cfg, err := tarantool.New().
		WithoutSchema().
		WithEnvIgnore("TT_CLI_*").
		Build(ctx)
	require.NoError(t, err)

	_, ok := cfg.Lookup(config.NewKeyPath("cli/repo/rocks"))
	assert.False(t, ok, "glob ignore should drop matching vars")
}

func TestBuild_EnvIgnore_AppliesToDefaultSuffix(t *testing.T) {
	t.Setenv("TT_FOO_DEFAULT", "x")

	ctx := context.Background()

	cfg, err := tarantool.New().
		WithoutSchema().
		WithEnvIgnore("TT_FOO_DEFAULT").
		Build(ctx)
	require.NoError(t, err)

	_, ok := cfg.Lookup(config.NewKeyPath("foo"))
	assert.False(t, ok, "ignore should also filter _DEFAULT vars")
}

func TestBuild_EnvIgnore_MultiplePatterns(t *testing.T) {
	t.Setenv("TT_A", "1")
	t.Setenv("TT_B_INNER", "2")
	t.Setenv("TT_C", "3")

	ctx := context.Background()

	cfg, err := tarantool.New().
		WithoutSchema().
		WithEnvIgnore("TT_A", "TT_B_*").
		Build(ctx)
	require.NoError(t, err)

	_, ok := cfg.Lookup(config.NewKeyPath("a"))
	assert.False(t, ok, "TT_A ignored")

	_, ok = cfg.Lookup(config.NewKeyPath("b/inner"))
	assert.False(t, ok, "TT_B_INNER ignored")

	var c string

	_, err = cfg.Get(config.NewKeyPath("c"), &c)
	require.NoError(t, err)
	assert.Equal(t, "3", c, "TT_C survives")
}

func TestBuild_EnvIgnore_AccumulatesAcrossCalls(t *testing.T) {
	t.Setenv("TT_A", "1")
	t.Setenv("TT_B", "2")

	ctx := context.Background()

	cfg, err := tarantool.New().
		WithoutSchema().
		WithEnvIgnore("TT_A").
		WithEnvIgnore("TT_B").
		Build(ctx)
	require.NoError(t, err)

	_, ok := cfg.Lookup(config.NewKeyPath("a"))
	assert.False(t, ok)

	_, ok = cfg.Lookup(config.NewKeyPath("b"))
	assert.False(t, ok)
}

func TestBuild_EnvIgnore_BadPattern(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	_, err := tarantool.New().
		WithoutSchema().
		WithEnvIgnore("TT_[").
		Build(ctx)
	require.ErrorIs(t, err, tarantool.ErrBadEnvIgnorePattern)
}

func TestBuild_EnvIgnore_EmptyIsNoOp(t *testing.T) {
	t.Setenv("TT_FOO", "value")

	ctx := context.Background()

	cfg, err := tarantool.New().
		WithoutSchema().
		Build(ctx)
	require.NoError(t, err)

	var foo string

	_, err = cfg.Get(config.NewKeyPath("foo"), &foo)
	require.NoError(t, err)
	assert.Equal(t, "value", foo, "no ignore patterns ⇒ unchanged behaviour")
}

// TestBuild_EnvIgnore_TTCLIPollution checks that WithEnvIgnore("TT_CLI_*")
// drops tt-CLI env vars from the config tree while leaving other TT_* vars intact.
func TestBuild_EnvIgnore_TTCLIPollution(t *testing.T) {
	t.Setenv("TT_CLI_REPO_ROCKS", "/sdk/rocks")
	t.Setenv("TT_CLI_TARANTOOL_PREFIX", "/sdk/3.5.0")
	t.Setenv("TT_USER_FIELD", "expected-value")

	ctx := context.Background()

	dirty, err := tarantool.New().
		WithoutSchema().
		Build(ctx)
	require.NoError(t, err)

	_, polluted := dirty.Lookup(config.NewKeyPath("cli/repo/rocks"))
	assert.True(t, polluted, "baseline: TT_CLI_REPO_ROCKS leaks into the tree without an ignore list")

	clean, err := tarantool.New().
		WithoutSchema().
		WithEnvIgnore("TT_CLI_*").
		Build(ctx)
	require.NoError(t, err)

	_, polluted = clean.Lookup(config.NewKeyPath("cli/repo/rocks"))
	assert.False(t, polluted, "TT_CLI_* should be gone after WithEnvIgnore")

	var userField string

	_, err = clean.Get(config.NewKeyPath("user/field"), &userField)
	require.NoError(t, err)
	assert.Equal(t, "expected-value", userField, "unrelated TT_* vars survive")
}
