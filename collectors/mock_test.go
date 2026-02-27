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

func TestNewMock(t *testing.T) {
	t.Parallel()

	mc := collectors.NewMock()
	require.NotNil(t, mc)
	assert.Equal(t, "mock", mc.Name())
	assert.Equal(t, config.UnknownSource, mc.Source())
	assert.Equal(t, config.RevisionType(""), mc.Revision())
	assert.False(t, mc.KeepOrder())
}

func TestMock_WithName(t *testing.T) {
	t.Parallel()

	mc := collectors.NewMock().WithName("custom")
	assert.Equal(t, "custom", mc.Name())
}

func TestMock_WithSourceType(t *testing.T) {
	t.Parallel()

	mc := collectors.NewMock().WithSourceType(config.FileSource)
	assert.Equal(t, config.FileSource, mc.Source())
}

func TestMock_WithRevision(t *testing.T) {
	t.Parallel()

	mc := collectors.NewMock().WithRevision("v1.0.0")
	assert.Equal(t, config.RevisionType("v1.0.0"), mc.Revision())
}

func TestMock_WithKeepOrder(t *testing.T) {
	t.Parallel()

	mc := collectors.NewMock().WithKeepOrder(true)
	assert.True(t, mc.KeepOrder())
}

func TestMock_WithEntry(t *testing.T) {
	t.Parallel()

	mc := collectors.NewMock().
		WithEntry(config.NewKeyPath("/server/host"), "localhost").
		WithEntry(config.NewKeyPath("/server/port"), 8080)
	ctx := context.Background()
	ch := mc.Read(ctx)

	values := make([]config.Value, 0, 2)
	for val := range ch {
		values = append(values, val)
	}

	assert.Len(t, values, 2)

	var host string

	err := values[0].Get(&host)
	require.NoError(t, err)
	assert.Equal(t, "localhost", host)

	var port int

	err = values[1].Get(&port)
	require.NoError(t, err)
	assert.Equal(t, 8080, port)
}

func TestMock_WithEntries(t *testing.T) {
	t.Parallel()

	entries := map[string]any{
		"log/level":  "debug",
		"log/output": "stdout",
	}
	mc := collectors.NewMock().WithEntries(entries)
	ctx := context.Background()
	ch := mc.Read(ctx)

	got := make(map[string]any)

	for val := range ch {
		var dest any

		err := val.Get(&dest)
		require.NoError(t, err)

		meta := val.Meta()

		got[meta.Key.String()] = dest
	}

	assert.Equal(t, entries, got)
}

func TestMock_Read_Cancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	mc := collectors.NewMock().
		WithEntry(config.NewKeyPath("/a"), 1).
		WithEntry(config.NewKeyPath("/b"), 2)
	valueCh := mc.Read(ctx)

	val, ok := <-valueCh
	require.True(t, ok)

	var dest int

	err := val.Get(&dest)
	require.NoError(t, err)

	cancel()

	testutil.Drain(t, valueCh)
}
