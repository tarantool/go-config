package testutil_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tarantool/go-config/internal/testutil"
	"github.com/tarantool/go-storage/kv"
	"github.com/tarantool/go-storage/operation"
)

func TestMockStorage_Put_And_Get(t *testing.T) {
	t.Parallel()

	mock := testutil.NewMockStorage().
		Put([]byte("/config/a"), []byte("value1")).
		Put([]byte("/config/b"), []byte("value2"))

	ctx := context.Background()
	resp, err := mock.Tx(ctx).
		Then(operation.Get([]byte("/config/a"))).
		Commit()

	require.NoError(t, err)
	require.Len(t, resp.Results, 1)
	require.Len(t, resp.Results[0].Values, 1)
	assert.Equal(t, []byte("value1"), resp.Results[0].Values[0].Value)
}

func TestMockStorage_PrefixGet(t *testing.T) {
	t.Parallel()

	mock := testutil.NewMockStorage().
		Put([]byte("/config/a"), []byte("v1")).
		Put([]byte("/config/b"), []byte("v2")).
		Put([]byte("/other/c"), []byte("v3"))

	ctx := context.Background()
	resp, err := mock.Tx(ctx).
		Then(operation.Get([]byte("/config/"))).
		Commit()

	require.NoError(t, err)
	require.Len(t, resp.Results, 1)
	assert.Len(t, resp.Results[0].Values, 2)
}

var errTestTx = assert.AnError

func TestMockStorage_TxError(t *testing.T) {
	t.Parallel()

	mock := testutil.NewMockStorage().WithTxError(errTestTx)

	ctx := context.Background()
	_, err := mock.Tx(ctx).
		Then(operation.Get([]byte("/key"))).
		Commit()

	require.ErrorIs(t, err, errTestTx)
}

func TestMockStorage_PutKV(t *testing.T) {
	t.Parallel()

	mock := testutil.NewMockStorage().PutKV(kv.KeyValue{
		Key:         []byte("/key"),
		Value:       []byte("value"),
		ModRevision: 42,
	})

	ctx := context.Background()
	resp, err := mock.Tx(ctx).
		Then(operation.Get([]byte("/key"))).
		Commit()

	require.NoError(t, err)
	require.Len(t, resp.Results, 1)
	require.Len(t, resp.Results[0].Values, 1)
	assert.Equal(t, int64(42), resp.Results[0].Values[0].ModRevision)
}

func TestMockStorage_GetNotFound(t *testing.T) {
	t.Parallel()

	mock := testutil.NewMockStorage()

	ctx := context.Background()
	resp, err := mock.Tx(ctx).
		Then(operation.Get([]byte("/missing"))).
		Commit()

	require.NoError(t, err)
	require.Len(t, resp.Results, 1)
	assert.Empty(t, resp.Results[0].Values)
}

func TestMockStorage_TxPut(t *testing.T) {
	t.Parallel()

	mock := testutil.NewMockStorage()

	ctx := context.Background()
	_, err := mock.Tx(ctx).
		Then(operation.Put([]byte("/key"), []byte("value"))).
		Commit()
	require.NoError(t, err)

	resp, err := mock.Tx(ctx).
		Then(operation.Get([]byte("/key"))).
		Commit()

	require.NoError(t, err)
	require.Len(t, resp.Results, 1)
	require.Len(t, resp.Results[0].Values, 1)
	assert.Equal(t, []byte("value"), resp.Results[0].Values[0].Value)
}
