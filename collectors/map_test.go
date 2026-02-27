package collectors_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
	require.NotNil(t, mc)
	assert.Equal(t, "map", mc.Name())
	assert.Equal(t, config.UnknownSource, mc.Source())
	assert.Equal(t, config.RevisionType(""), mc.Revision())
	assert.False(t, mc.KeepOrder())
}

func TestMap_WithName(t *testing.T) {
	t.Parallel()

	data := map[string]any{}
	mc := collectors.NewMap(data).WithName("custom")
	assert.Equal(t, "custom", mc.Name())
}

func TestMap_WithSourceType(t *testing.T) {
	t.Parallel()

	data := map[string]any{}
	mc := collectors.NewMap(data).WithSourceType(config.FileSource)
	assert.Equal(t, config.FileSource, mc.Source())
}

func TestMap_WithRevision(t *testing.T) {
	t.Parallel()

	data := map[string]any{}
	mc := collectors.NewMap(data).WithRevision("v1.0.0")
	assert.Equal(t, config.RevisionType("v1.0.0"), mc.Revision())
}

func TestMap_WithKeepOrder(t *testing.T) {
	t.Parallel()

	data := map[string]any{}
	mc := collectors.NewMap(data).WithKeepOrder(true)
	assert.True(t, mc.KeepOrder())
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

	assert.Len(t, values, 2)

	for _, val := range values {
		var dest any

		err := val.Get(&dest)
		require.NoError(t, err)
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

	val, ok := <-valueCh
	require.True(t, ok)

	var dest int

	err := val.Get(&dest)
	require.NoError(t, err)

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
	assert.Equal(t, "test", mc.Name())
	assert.Equal(t, config.FileSource, mc.Source())

	ch := mc.Read(ctx)

	count := 0
	for v := range ch {
		count++

		var dest any

		err := v.Get(&dest)
		require.NoError(t, err)
	}

	assert.Equal(t, 3, count)
}
