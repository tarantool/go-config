package collectors

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strconv"
	"sync"

	"github.com/tarantool/go-config"
	"github.com/tarantool/go-config/storage"
)

// StorageSource implements config.DataSource for reading a single configuration
// document from a key-value storage. It uses a transactional Get operation
// to fetch the value at the specified key and tracks its modification revision.
type StorageSource struct {
	storage  storage.Storage
	key      []byte
	revision config.RevisionType
	mu       sync.RWMutex
}

// NewStorageSource creates a new StorageSource that will read from the given
// storage at the specified key. The key is a byte slice; it is not interpreted
// as a path and is passed directly to the storage backend.
func NewStorageSource(strg storage.Storage, key []byte) *StorageSource {
	return &StorageSource{ //nolint:exhaustruct
		storage:  strg,
		key:      key,
		revision: "",
	}
}

// Name returns the fixed name "storage".
func (s *StorageSource) Name() string {
	return "storage"
}

// SourceType returns config.StorageSource.
func (s *StorageSource) SourceType() config.SourceType {
	return config.StorageSource
}

// Revision returns the modification revision of the last successfully fetched
// value. The revision is a string representation of the storage's ModRevision.
func (s *StorageSource) Revision() config.RevisionType {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.revision
}

// FetchStream performs a transactional Get for the configured key and returns
// an io.ReadCloser over the raw value. If the key is not found,
// ErrStorageKeyNotFound is returned. Storage errors are wrapped with
// ErrStorageFetch. The revision is updated on success.
func (s *StorageSource) FetchStream(ctx context.Context) (io.ReadCloser, error) {
	resp, err := s.storage.Tx(ctx).Then(storage.Get(s.key)).Commit()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrStorageFetch, err)
	}

	if len(resp.Results) == 0 || len(resp.Results[0].Values) == 0 {
		return nil, ErrStorageKeyNotFound
	}

	keyValue := resp.Results[0].Values[0]

	s.mu.Lock()
	s.revision = config.RevisionType(strconv.FormatInt(keyValue.ModRevision, 10))
	s.mu.Unlock()

	if len(keyValue.Value) == 0 {
		return io.NopCloser(bytes.NewReader([]byte{})), nil
	}

	return io.NopCloser(bytes.NewReader(keyValue.Value)), nil
}
