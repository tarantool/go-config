package collectors_test

import (
	"context"
	"testing"

	"github.com/shoenig/test"
	"github.com/shoenig/test/must"

	"github.com/tarantool/go-config"
	"github.com/tarantool/go-config/collectors"
	"github.com/tarantool/go-config/internal/testutil"
)

func TestNewMap(t *testing.T) {
	t.Parallel()

	data := map[string]any{
		"foo": "bar",
	}
	mc := collectors.NewMap(data)
	must.NotNil(t, mc)
	test.Eq(t, "map", mc.Name())
	test.Eq(t, config.UnknownSource, mc.Source())
	test.Eq(t, "", mc.Revision())
	test.False(t, mc.KeepOrder())
}

func TestMap_WithName(t *testing.T) {
	t.Parallel()

	data := map[string]any{}
	mc := collectors.NewMap(data).WithName("custom")
	test.Eq(t, "custom", mc.Name())
}

func TestMap_WithSourceType(t *testing.T) {
	t.Parallel()

	data := map[string]any{}
	mc := collectors.NewMap(data).WithSourceType(config.FileSource)
	test.Eq(t, config.FileSource, mc.Source())
}

func TestMap_WithRevision(t *testing.T) {
	t.Parallel()

	data := map[string]any{}
	mc := collectors.NewMap(data).WithRevision("v1.0.0")
	test.Eq(t, "v1.0.0", mc.Revision())
}

func TestMap_WithKeepOrder(t *testing.T) {
	t.Parallel()

	data := map[string]any{}
	mc := collectors.NewMap(data).WithKeepOrder(true)
	test.True(t, mc.KeepOrder())
}

func TestMap_Read_Basic(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	data := map[string]any{
		"foo": "bar",
		"nested": map[string]any{
			"key": 42,
		},
	}
	mc := collectors.NewMap(data)
	ch := mc.Read(ctx)

	values := make([]config.Value, 0, 2)
	for v := range ch {
		values = append(values, v)
	}

	test.Len(t, 2, values)

	// Verify values can be extracted.
	for _, val := range values {
		var dest any

		err := val.Get(&dest)
		must.NoError(t, err)
	}
}

func TestMap_Read_Cancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	data := map[string]any{
		"a": 1,
		"b": 2,
	}
	mc := collectors.NewMap(data)
	valueCh := mc.Read(ctx)

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

func TestMap_Read_ComplexStructure(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	data := map[string]any{
		"server": map[string]any{
			"port":    8080,
			"timeout": "30s",
		},
		"log": map[string]any{
			"level": "debug",
		},
	}
	mc := collectors.NewMap(data).WithName("test").WithSourceType(config.FileSource)
	test.Eq(t, "test", mc.Name())
	test.Eq(t, config.FileSource, mc.Source())

	ch := mc.Read(ctx)

	count := 0
	for v := range ch {
		count++

		var dest any

		err := v.Get(&dest)
		must.NoError(t, err)
	}

	test.Eq(t, 3, count)
}
