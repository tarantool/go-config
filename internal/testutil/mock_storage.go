package testutil

import (
	"bytes"
	"context"
	"sort"
	"sync"

	"github.com/tarantool/go-storage"
	"github.com/tarantool/go-storage/kv"
	"github.com/tarantool/go-storage/operation"
	"github.com/tarantool/go-storage/predicate"
	"github.com/tarantool/go-storage/tx"
	"github.com/tarantool/go-storage/watch"
)

// MockStorage is an in-memory implementation of storage.Storage for testing.
// It supports Get (with prefix matching), Put, and Delete operations,
// and optionally injects errors.
type MockStorage struct {
	mu   sync.RWMutex
	data map[string]kv.KeyValue
	rev  int64

	txErr    error
	watchChs []chan watch.Event
}

// NewMockStorage creates a new MockStorage instance.
func NewMockStorage() *MockStorage {
	return &MockStorage{ //nolint:exhaustruct
		data: make(map[string]kv.KeyValue),
	}
}

// WithTxError configures the mock to return an error on Tx.Commit.
func (m *MockStorage) WithTxError(err error) *MockStorage {
	m.txErr = err
	return m
}

// Put stores a key-value pair in the mock. It assigns an auto-incremented
// ModRevision and returns the mock for chaining.
func (m *MockStorage) Put(key, value []byte) *MockStorage {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.rev++

	m.data[string(key)] = kv.KeyValue{
		Key:         key,
		Value:       value,
		ModRevision: m.rev,
	}

	return m
}

// PutKV stores a kv.KeyValue directly (preserving its ModRevision).
func (m *MockStorage) PutKV(entry kv.KeyValue) *MockStorage {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.data[string(entry.Key)] = entry

	return m
}

// Watch implements storage.Storage.
func (m *MockStorage) Watch(_ context.Context, _ []byte, _ ...watch.Option) <-chan watch.Event {
	eventCh := make(chan watch.Event, 16) //nolint:mnd

	m.mu.Lock()
	m.watchChs = append(m.watchChs, eventCh)
	m.mu.Unlock()

	return eventCh
}

// SendWatchEvent sends an event to all registered watch channels.
func (m *MockStorage) SendWatchEvent(event watch.Event) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, ch := range m.watchChs {
		ch <- event
	}
}

// CloseWatchChannels closes all registered watch channels.
func (m *MockStorage) CloseWatchChannels() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, ch := range m.watchChs {
		close(ch)
	}

	m.watchChs = nil
}

// Tx implements storage.Storage.
func (m *MockStorage) Tx(_ context.Context) tx.Tx {
	return &mockTx{ //nolint:exhaustruct
		storage: m,
		err:     m.txErr,
	}
}

// Range implements storage.Storage.
func (m *MockStorage) Range(_ context.Context, _ ...storage.RangeOption) ([]kv.KeyValue, error) {
	return nil, nil
}

// mockTx implements tx.Tx for MockStorage.
type mockTx struct {
	storage    *MockStorage
	err        error
	predicates []predicate.Predicate
	ops        []operation.Operation
}

// If implements tx.Tx.
func (t *mockTx) If(predicates ...predicate.Predicate) tx.Tx {
	t.predicates = append(t.predicates, predicates...)
	return t
}

// Then implements tx.Tx.
func (t *mockTx) Then(ops ...operation.Operation) tx.Tx {
	t.ops = append(t.ops, ops...)
	return t
}

// Else implements tx.Tx.
func (t *mockTx) Else(_ ...operation.Operation) tx.Tx {
	return t
}

// Commit implements tx.Tx.
func (t *mockTx) Commit() (tx.Response, error) {
	if t.err != nil {
		return tx.Response{}, t.err
	}

	t.storage.mu.Lock()
	defer t.storage.mu.Unlock()

	results := make([]tx.RequestResponse, 0, len(t.ops))

	for _, oper := range t.ops {
		switch oper.Type() {
		case operation.TypeGet:
			kvs := t.executeGet(oper.Key())

			results = append(results, tx.RequestResponse{Values: kvs})
		case operation.TypePut:
			t.storage.rev++

			t.storage.data[string(oper.Key())] = kv.KeyValue{
				Key:         oper.Key(),
				Value:       oper.Value(),
				ModRevision: t.storage.rev,
			}

			results = append(results, tx.RequestResponse{Values: nil})
		case operation.TypeDelete:
			delete(t.storage.data, string(oper.Key()))

			results = append(results, tx.RequestResponse{Values: nil})
		}
	}

	return tx.Response{
		Succeeded: true,
		Results:   results,
	}, nil
}

// executeGet retrieves key-value pairs matching the given key.
// If the key ends with "/", it performs a prefix match.
func (t *mockTx) executeGet(key []byte) []kv.KeyValue {
	if bytes.HasSuffix(key, []byte("/")) {
		return t.getByPrefix(key)
	}

	if entry, ok := t.storage.data[string(key)]; ok {
		return []kv.KeyValue{entry}
	}

	return nil
}

// getByPrefix returns all entries whose key starts with the given prefix,
// sorted by key for deterministic test output.
func (t *mockTx) getByPrefix(prefix []byte) []kv.KeyValue {
	var result []kv.KeyValue

	prefixStr := string(prefix)

	for k, v := range t.storage.data {
		if len(k) >= len(prefixStr) && k[:len(prefixStr)] == prefixStr {
			result = append(result, v)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return bytes.Compare(result[i].Key, result[j].Key) < 0
	})

	return result
}
