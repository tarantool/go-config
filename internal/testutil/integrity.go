package testutil

import (
	storage "github.com/tarantool/go-storage/v2"
	"github.com/tarantool/go-storage/v2/integrity"
	"github.com/tarantool/go-storage/v2/kv"
	"github.com/tarantool/go-storage/v2/namer"
)

// NewRawTyped creates an *integrity.Store[[]byte] with no hashers or
// signers, using a raw passthrough marshaller for byte handling. prefix is
// applied via [storage.Prefixed] on strg.
func NewRawTyped(strg storage.Storage, prefix string) *integrity.Store[[]byte] {
	prefixedStorage, err := storage.Prefixed(trimPrefix(prefix), strg)
	if err != nil {
		panic("NewRawTyped: " + err.Error())
	}

	codec, err := integrity.NewCodecBuilder[[]byte]().
		WithMarshaller(rawBytesMarshaller{}).
		Build()
	if err != nil {
		panic("NewRawTyped: " + err.Error())
	}

	return codec.Bind(prefixedStorage)
}

// rawBytesMarshaller passes bytes through without any encoding/decoding.
type rawBytesMarshaller struct{}

func (rawBytesMarshaller) Marshal(data []byte) ([]byte, error)   { return data, nil }
func (rawBytesMarshaller) Unmarshal(data []byte) ([]byte, error) { return data, nil }

// trimPrefix strips the trailing "/" callers historically pass (e.g.
// "/config/"), matching what [storage.Prefixed] and [namer.WithKeyPrefix]
// require.
func trimPrefix(prefix string) string {
	if prefix == "/" {
		return ""
	}

	if len(prefix) > 0 && prefix[len(prefix)-1] == '/' {
		return prefix[:len(prefix)-1]
	}

	return prefix
}

// rawNamer builds an unnamed namer with prefix baked in directly via
// [namer.WithKeyPrefix]. This must produce the same on-disk keys as
// [NewRawTyped], which instead bakes the prefix into the storage layer via
// [storage.Prefixed] — the two are equivalent for an unnamed codec.
func rawNamer(prefix string) namer.Namer {
	n, err := namer.New(namer.ObjectLocationMissing, nil, nil, namer.WithKeyPrefix(trimPrefix(prefix)))
	if err != nil {
		panic("rawNamer: " + err.Error())
	}

	return n
}

// NewRawValidator creates an integrity.Validator[[]byte] with no hashers or
// signers. It validates integrity keys but performs no hash/signature checks.
func NewRawValidator(prefix string) integrity.Validator[[]byte] {
	return integrity.NewValidator[[]byte](rawNamer(prefix), rawBytesMarshaller{}, nil, nil)
}

// NewRawGenerator creates an integrity.Generator[[]byte] with no hashers or
// signers. It generates namer-formatted keys for test data.
func NewRawGenerator(prefix string) integrity.Generator[[]byte] {
	return integrity.NewGenerator[[]byte](rawNamer(prefix), rawBytesMarshaller{}, nil, nil)
}

// PutIntegrity stores a named value in the mock storage using the integrity
// generator. This ensures the correct namer-formatted keys are used,
// matching what the collector's validator will expect. ModRevision is
// auto-incremented by the mock.
func PutIntegrity(mock *MockStorage, prefix, name string, value []byte) {
	gen := NewRawGenerator(prefix)

	kvs, err := gen.Generate(name, value)
	if err != nil {
		panic("PutIntegrity: " + err.Error())
	}

	for _, entry := range kvs {
		mock.Put(entry.Key, entry.Value)
	}
}

// GenerateIntegrityKVs generates integrity-formatted kv.KeyValue entries for
// the given name and value. Useful for building mock responses in tests.
func GenerateIntegrityKVs(prefix, name string, value []byte) []kv.KeyValue {
	gen := NewRawGenerator(prefix)

	kvs, err := gen.Generate(name, value)
	if err != nil {
		panic("GenerateIntegrityKVs: " + err.Error())
	}

	return kvs
}
