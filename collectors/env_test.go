package collectors_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tarantool/go-config"
	"github.com/tarantool/go-config/collectors"
	"github.com/tarantool/go-config/internal/testutil"
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

		return config.NewKeyPathWithDelim(strings.Join(parts, "/"), "/")
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
