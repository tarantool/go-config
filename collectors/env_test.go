package collectors_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tarantool/go-config/v2"
	"github.com/tarantool/go-config/v2/collectors"
	"github.com/tarantool/go-config/v2/internal/testutil"
)

func TestNewEnv(t *testing.T) {
	t.Parallel()

	ec := collectors.NewEnv()
	require.NotNil(t, ec)
	assert.Equal(t, "env", ec.Name())
	assert.Equal(t, config.EnvSource, ec.Source())
	assert.Equal(t, config.RevisionType(""), ec.Revision())
	assert.False(t, ec.KeepOrder())
}

func TestEnv_WithName(t *testing.T) {
	t.Parallel()

	ec := collectors.NewEnv().WithName("custom")
	assert.Equal(t, "custom", ec.Name())
}

func TestEnv_WithSourceType(t *testing.T) {
	t.Parallel()

	ec := collectors.NewEnv().WithSourceType(config.FileSource)
	assert.Equal(t, config.FileSource, ec.Source())
}

func TestEnv_WithRevision(t *testing.T) {
	t.Parallel()

	ec := collectors.NewEnv().WithRevision("v1.0.0")
	assert.Equal(t, config.RevisionType("v1.0.0"), ec.Revision())
}

func TestEnv_WithKeepOrder(t *testing.T) {
	t.Parallel()

	ec := collectors.NewEnv().WithKeepOrder(true)
	assert.True(t, ec.KeepOrder())
}

func TestEnv_Read_Basic(t *testing.T) {
	ctx := context.Background()

	t.Setenv("MYAPP_FOO", "bar")
	t.Setenv("MYAPP_NESTED_KEY", "42")

	ec := collectors.NewEnv().
		WithPrefix("MYAPP_").
		WithDelimiter("_")
	ch := ec.Read(ctx)

	values := make([]config.Value, 0, 2)
	for val := range ch {
		values = append(values, val)
	}

	assert.Len(t, values, 2)

	for _, val := range values {
		var dest any

		err := val.Get(&dest)
		require.NoError(t, err)
	}
}

func TestEnv_Read_Transformation(t *testing.T) {
	ctx := context.Background()

	t.Setenv("MYAPP_DB_HOST", "localhost")
	t.Setenv("MYAPP_DB_PORT", "5432")

	ec := collectors.NewEnv().
		WithPrefix("MYAPP_").
		WithDelimiter("_")
	ch := ec.Read(ctx)

	got := make(map[string]any)

	for val := range ch {
		var dest any

		err := val.Get(&dest)
		require.NoError(t, err)

		meta := val.Meta()
		key := meta.Key.String()

		got[key] = dest
	}

	expected := map[string]any{
		"db/host": "localhost",
		"db/port": "5432",
	}
	assert.Equal(t, expected, got)
}

func TestEnv_Read_CustomTransform(t *testing.T) {
	ctx := context.Background()

	t.Setenv("MYAPP_FOO_BAR", "value")

	transform := func(key string) config.KeyPath {
		parts := strings.Split(key, "_")
		for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
			parts[i], parts[j] = parts[j], parts[i]
		}

		return config.NewKeyPathFromSegments(parts)
	}

	ec := collectors.NewEnv().
		WithPrefix("MYAPP_").
		WithTransform(transform)
	ch := ec.Read(ctx)

	got := make(map[string]any)

	for val := range ch {
		var dest any

		err := val.Get(&dest)
		require.NoError(t, err)

		meta := val.Meta()

		got[meta.Key.String()] = dest
	}

	expected := map[string]any{
		"BAR/FOO": "value",
	}
	assert.Equal(t, expected, got)
}

func TestEnv_Read_Cancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	t.Setenv("MYAPP_A", "1")
	t.Setenv("MYAPP_B", "2")

	ec := collectors.NewEnv().WithPrefix("MYAPP_")
	valueCh := ec.Read(ctx)

	val, ok := <-valueCh
	require.True(t, ok)

	var dest int

	err := val.Get(&dest)
	require.NoError(t, err)

	cancel()

	testutil.Drain(t, valueCh)
}

// TestEnv_Read_IndexKeysMakeArrays checks that a number in a variable name
// is still read as an array index: the names do not tell an index from a map
// key, and Env keeps taking them for indices.
func TestEnv_Read_IndexKeysMakeArrays(t *testing.T) {
	t.Setenv("MYAPP_ROLES_0", "a")
	t.Setenv("MYAPP_ROLES_1", "b")
	t.Setenv("MYAPP_LISTEN_0_URI", "127.0.0.1:3301")

	builder := config.NewBuilder()

	builder = builder.AddCollector(collectors.NewEnv().WithPrefix("MYAPP_"))

	cfg, errs := builder.Build(t.Context())
	require.Empty(t, errs)

	var roles any

	_, err := cfg.Get(config.NewKeyPath("roles"), &roles)
	require.NoError(t, err)
	assert.Equal(t, []any{"a", "b"}, roles)

	var listen any

	_, err = cfg.Get(config.NewKeyPath("listen"), &listen)
	require.NoError(t, err)
	assert.Equal(t, []any{map[string]any{"uri": "127.0.0.1:3301"}}, listen)
}

func TestEnv_Read_NoMatchNoEmit(t *testing.T) {
	ctx := context.Background()

	t.Setenv("OTHERAPP_FOO", "bar")
	t.Setenv("OTHERAPP_BAZ", "qux")

	ec := collectors.NewEnv().WithPrefix("MYAPP_")
	ch := ec.Read(ctx)

	var values []config.Value
	for val := range ch {
		values = append(values, val)
	}

	assert.Empty(t, values)
}
