package collectors_test

import (
	"context"
	"strings"
	"testing"

	"github.com/shoenig/test"
	"github.com/shoenig/test/must"

	"github.com/tarantool/go-config"
	"github.com/tarantool/go-config/collectors"
	"github.com/tarantool/go-config/internal/testutil"
)

func TestNewEnv(t *testing.T) {
	t.Parallel()

	ec := collectors.NewEnv()
	must.NotNil(t, ec)
	test.Eq(t, "env", ec.Name())
	test.Eq(t, config.EnvSource, ec.Source())
	test.Eq(t, "", ec.Revision())
	test.False(t, ec.KeepOrder())
}

func TestEnv_WithName(t *testing.T) {
	t.Parallel()

	ec := collectors.NewEnv().WithName("custom")
	test.Eq(t, "custom", ec.Name())
}

func TestEnv_WithSourceType(t *testing.T) {
	t.Parallel()

	ec := collectors.NewEnv().WithSourceType(config.FileSource)
	test.Eq(t, config.FileSource, ec.Source())
}

func TestEnv_WithRevision(t *testing.T) {
	t.Parallel()

	ec := collectors.NewEnv().WithRevision("v1.0.0")
	test.Eq(t, "v1.0.0", ec.Revision())
}

func TestEnv_WithKeepOrder(t *testing.T) {
	t.Parallel()

	ec := collectors.NewEnv().WithKeepOrder(true)
	test.True(t, ec.KeepOrder())
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

	test.Len(t, 2, values)

	// Verify values can be extracted.
	for _, val := range values {
		var dest any

		err := val.Get(&dest)
		must.NoError(t, err)
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

	// Collect values into a map for easier assertion.
	got := make(map[string]any)

	for val := range ch {
		var dest any

		err := val.Get(&dest)
		must.NoError(t, err)
		// Get key path from meta.
		meta := val.Meta()
		key := meta.Key.String()

		got[key] = dest
	}

	expected := map[string]any{
		"db/host": "localhost",
		"db/port": "5432",
	}
	test.Eq(t, expected, got)
}

func TestEnv_Read_CustomTransform(t *testing.T) {
	ctx := context.Background()

	t.Setenv("MYAPP_FOO_BAR", "value")

	transform := func(key string) config.KeyPath {
		// Simply split by underscore and reverse order.
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
		must.NoError(t, err)

		meta := val.Meta()

		got[meta.Key.String()] = dest
	}

	expected := map[string]any{
		"BAR/FOO": "value",
	}
	test.Eq(t, expected, got)
}

func TestEnv_Read_Cancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	t.Setenv("MYAPP_A", "1")
	t.Setenv("MYAPP_B", "2")

	ec := collectors.NewEnv().WithPrefix("MYAPP_")
	valueCh := ec.Read(ctx)

	// Read first value.
	val, ok := <-valueCh
	must.True(t, ok)

	var dest int

	err := val.Get(&dest)
	must.NoError(t, err)

	// Cancel context.
	cancel()

	testutil.Drain(t, valueCh)
}
