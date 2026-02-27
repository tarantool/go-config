package testutil_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tarantool/go-config/internal/testutil"
	"github.com/tarantool/go-config/storage"
)

func TestMockStorage_Range(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	kvs := []storage.KeyValue{
		{Key: []byte("/config/a"), Value: []byte("value1"), ModRevision: 1},
		{Key: []byte("/config/b"), Value: []byte("value2"), ModRevision: 2},
	}

	mock := testutil.NewMockStorage().WithRangeResponse(kvs)
	result, err := mock.Range(ctx)
	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "/config/a", string(result[0].Key))
	assert.Equal(t, "/config/b", string(result[1].Key))
}

func TestMockStorage_Range_Error(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	mock := testutil.NewMockStorage().WithRangeError(storage.ErrRangeFailed)
	_, err := mock.Range(ctx)
	require.Error(t, err)
}

func TestMockStorage_Tx(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	resp := storage.Response{
		Results: []storage.Result{
			{
				Values: []storage.KeyValue{
					{Key: []byte("/key"), Value: []byte("value"), ModRevision: 5},
				},
			},
		},
	}

	mock := testutil.NewMockStorage().WithTxResponse(resp)
	tx := mock.Tx(ctx)
	result, err := tx.Commit()
	require.NoError(t, err)
	assert.Len(t, result.Results, 1)
	assert.Len(t, result.Results[0].Values, 1)
	assert.Equal(t, int64(5), result.Results[0].Values[0].ModRevision)
}

func TestMockStorage_Tx_Error(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	mock := testutil.NewMockStorage().WithTxError(storage.ErrTxFailed)
	tx := mock.Tx(ctx)
	_, err := tx.Commit()
	require.Error(t, err)
}

func TestMockTx_Then(t *testing.T) {
	t.Parallel()

	resp := storage.Response{
		Results: []storage.Result{
			{Values: []storage.KeyValue{{Key: []byte("key"), Value: []byte("value")}}},
		},
	}
	mock := testutil.NewMockStorage().WithTxResponse(resp)

	ctx := context.Background()
	tx := mock.Tx(ctx)

	tx = tx.Then(storage.Get([]byte("/test")))

	result, err := tx.Commit()
	require.NoError(t, err)
	assert.Len(t, result.Results, 1)
}
