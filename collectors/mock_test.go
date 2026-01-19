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

func TestNewMock(t *testing.T) {
	t.Parallel()

	mc := collectors.NewMock()
	must.NotNil(t, mc)
	test.Eq(t, "mock", mc.Name())
	test.Eq(t, config.UnknownSource, mc.Source())
	test.Eq(t, "", mc.Revision())
	test.False(t, mc.KeepOrder())
}

func TestMock_WithName(t *testing.T) {
	t.Parallel()

	mc := collectors.NewMock().WithName("custom")
	test.Eq(t, "custom", mc.Name())
}

func TestMock_WithSourceType(t *testing.T) {
	t.Parallel()

	mc := collectors.NewMock().WithSourceType(config.FileSource)
	test.Eq(t, config.FileSource, mc.Source())
}

func TestMock_WithRevision(t *testing.T) {
	t.Parallel()

	mc := collectors.NewMock().WithRevision("v1.0.0")
	test.Eq(t, "v1.0.0", mc.Revision())
}

func TestMock_WithKeepOrder(t *testing.T) {
	t.Parallel()

	mc := collectors.NewMock().WithKeepOrder(true)
	test.True(t, mc.KeepOrder())
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

	test.Len(t, 2, values)

	// Verify values can be extracted.
	var host string

	err := values[0].Get(&host)
	must.NoError(t, err)
	test.Eq(t, "localhost", host)

	var port int

	err = values[1].Get(&port)
	must.NoError(t, err)
	test.Eq(t, 8080, port)
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
		must.NoError(t, err)

		meta := val.Meta()

		got[meta.Key.String()] = dest
	}

	test.Eq(t, entries, got)
}

func TestMock_Read_Cancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	mc := collectors.NewMock().
		WithEntry(config.NewKeyPath("/a"), 1).
		WithEntry(config.NewKeyPath("/b"), 2)
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
