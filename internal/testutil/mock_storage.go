package testutil

import (
	"context"

	"github.com/tarantool/go-config/storage"
)

// MockStorage is a mock implementation of storage.Storage for testing.
type MockStorage struct {
	RangeResponse []storage.KeyValue
	RangeError    error
	TxResponse    storage.Response
	TxError       error
}

// NewMockStorage creates a new MockStorage instance.
func NewMockStorage() *MockStorage {
	return &MockStorage{} //nolint:exhaustruct
}

// WithRangeResponse sets the range response for the mock.
func (m *MockStorage) WithRangeResponse(kvs []storage.KeyValue) *MockStorage {
	m.RangeResponse = kvs
	return m
}

// WithRangeError sets the range error for the mock.
func (m *MockStorage) WithRangeError(err error) *MockStorage {
	m.RangeError = err
	return m
}

// WithTxResponse sets the transaction response for the mock.
func (m *MockStorage) WithTxResponse(resp storage.Response) *MockStorage {
	m.TxResponse = resp
	return m
}

// WithTxError sets the transaction error for the mock.
func (m *MockStorage) WithTxError(err error) *MockStorage {
	m.TxError = err
	return m
}

// Range implements storage.Storage.Range.
func (m *MockStorage) Range(_ context.Context, _ ...storage.RangeOption) ([]storage.KeyValue, error) {
	if m.RangeError != nil {
		return nil, m.RangeError
	}

	return m.RangeResponse, nil
}

// Tx implements storage.Storage.Tx.
func (m *MockStorage) Tx(_ context.Context) storage.Tx {
	return &MockTx{ //nolint:exhaustruct
		response: m.TxResponse,
		err:      m.TxError,
	}
}

// MockTx is a mock implementation of storage.Tx for testing.
type MockTx struct {
	response storage.Response
	err      error
	ops      []storage.Operation
}

// If implements storage.Tx.If.
func (t *MockTx) If(_ ...storage.Predicate) storage.Tx {
	return t
}

// Then implements storage.Tx.Then.
func (t *MockTx) Then(ops ...storage.Operation) storage.Tx {
	t.ops = append(t.ops, ops...)
	return t
}

// Else implements storage.Tx.Else.
func (t *MockTx) Else(_ ...storage.Operation) storage.Tx {
	return t
}

// Commit implements storage.Tx.Commit.
func (t *MockTx) Commit() (storage.Response, error) {
	return t.response, t.err
}
