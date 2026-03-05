package collectors

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strconv"
	"sync"

	"github.com/tarantool/go-config"
	"github.com/tarantool/go-storage"
	"github.com/tarantool/go-storage/crypto"
	"github.com/tarantool/go-storage/hasher"
	"github.com/tarantool/go-storage/integrity"
	"github.com/tarantool/go-storage/kv"
	"github.com/tarantool/go-storage/namer"
	"github.com/tarantool/go-storage/operation"
)

// StorageSource implements DataSource for reading a single configuration
// document from a key-value storage with integrity verification. It uses the
// integrity layer's namer and validator to generate properly structured keys
// and verify hashes/signatures on the fetched value.
type StorageSource struct {
	storage   storage.Storage
	name      string
	label     string
	srcType   config.SourceType
	revision  config.RevisionType
	namer     namer.Namer
	validator integrity.Validator[[]byte]
	mu        sync.RWMutex
}

// NewStorageSource creates a new StorageSource that will read from the given
// storage the value identified by the logical name. The prefix determines the
// key namespace in the storage backend (e.g., "/config/").
func NewStorageSource(
	strg storage.Storage,
	prefix string,
	name string,
	hashers []hasher.Hasher,
	verifiers []crypto.Verifier,
) *StorageSource {
	hNames := hasherNames(hashers)
	vNames := verifierNames(verifiers)

	namerInstance := namer.NewDefaultNamer(prefix, hNames, vNames)
	m := rawBytesMarshaller{}
	validator := integrity.NewValidator[[]byte](namerInstance, m, hashers, verifiers)

	return &StorageSource{
		storage:   strg,
		name:      name,
		label:     "storage",
		srcType:   config.StorageSource,
		revision:  "",
		namer:     namerInstance,
		validator: validator,
		mu:        sync.RWMutex{},
	}
}

// Name returns the fixed label "storage".
func (s *StorageSource) Name() string {
	return s.label
}

// SourceType returns config.StorageSource.
func (s *StorageSource) SourceType() config.SourceType {
	return s.srcType
}

// Revision returns the modification revision of the last successfully fetched
// value. The revision is a string representation of the storage's ModRevision.
func (s *StorageSource) Revision() config.RevisionType {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.revision
}

// FetchStream performs a transactional Get for the configured name with
// integrity verification and returns an io.ReadCloser over the raw value bytes.
// If the key is not found, ErrStorageKeyNotFound is returned. Storage errors
// are wrapped with ErrStorageFetch. Integrity verification errors are wrapped
// with ErrStorageValidation. The revision is updated on success.
func (s *StorageSource) FetchStream(ctx context.Context) (io.ReadCloser, error) {
	keys, err := s.namer.GenerateNames(s.name)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrStorageFetch, err)
	}

	ops := make([]operation.Operation, 0, len(keys))
	for _, key := range keys {
		ops = append(ops, operation.Get([]byte(key.Build())))
	}

	resp, err := s.storage.Tx(ctx).Then(ops...).Commit()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrStorageFetch, err)
	}

	var kvs []kv.KeyValue
	for _, r := range resp.Results {
		kvs = append(kvs, r.Values...)
	}

	if len(kvs) == 0 {
		return nil, ErrStorageKeyNotFound
	}

	results, err := s.validator.Validate(kvs)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrStorageValidation, err)
	}

	if len(results) == 0 {
		return nil, ErrStorageKeyNotFound
	}

	result := results[0]
	if result.Error != nil {
		return nil, fmt.Errorf("%w: %w", ErrStorageValidation, result.Error)
	}

	if result.Value.IsZero() {
		return nil, ErrStorageKeyNotFound
	}

	value := result.Value.Unwrap()

	s.mu.Lock()
	s.revision = config.RevisionType(strconv.FormatInt(result.ModRevision, 10))
	s.mu.Unlock()

	return io.NopCloser(bytes.NewReader(value)), nil
}

// Watch implements the Watcher interface. It returns a channel that streams
// change events for the configured name in storage.
func (s *StorageSource) Watch(ctx context.Context) (<-chan WatchEvent, error) {
	prefix := s.namer.Prefix(s.name, false)

	rawCh := s.storage.Watch(ctx, []byte(prefix))
	eventCh := make(chan WatchEvent)

	go func() {
		defer close(eventCh)

		for {
			select {
			case <-ctx.Done():
				return
			case _, ok := <-rawCh:
				if !ok {
					return
				}

				select {
				case <-ctx.Done():
					return
				case eventCh <- WatchEvent{Prefix: s.name}:
				}
			}
		}
	}()

	return eventCh, nil
}

// hasherNames extracts names from a slice of hashers.
func hasherNames(hashers []hasher.Hasher) []string {
	if len(hashers) == 0 {
		return nil
	}

	names := make([]string, 0, len(hashers))
	for _, h := range hashers {
		names = append(names, h.Name())
	}

	return names
}

// verifierNames extracts names from a slice of verifiers.
func verifierNames(verifiers []crypto.Verifier) []string {
	if len(verifiers) == 0 {
		return nil
	}

	names := make([]string, 0, len(verifiers))
	for _, v := range verifiers {
		names = append(names, v.Name())
	}

	return names
}
