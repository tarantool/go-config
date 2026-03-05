package testutil

import (
	"github.com/tarantool/go-storage"
	"github.com/tarantool/go-storage/integrity"
	"github.com/tarantool/go-storage/kv"
	"github.com/tarantool/go-storage/namer"
)

// NewRawTyped creates an *integrity.Typed[[]byte] with no hashers or
// signers, using a raw passthrough marshaller for byte handling.
func NewRawTyped(strg storage.Storage, prefix string) *integrity.Typed[[]byte] {
	return integrity.NewTypedBuilder[[]byte](strg).
		WithPrefix(prefix).
		WithMarshaller(rawBytesMarshaller{}).
		Build()
}

// rawBytesMarshaller passes bytes through without any encoding/decoding.
type rawBytesMarshaller struct{}

func (rawBytesMarshaller) Marshal(data []byte) ([]byte, error)   { return data, nil }
func (rawBytesMarshaller) Unmarshal(data []byte) ([]byte, error) { return data, nil }

// NewRawValidator creates an integrity.Validator[[]byte] with no hashers or
// signers. It validates integrity keys but performs no hash/signature checks.
func NewRawValidator(prefix string) integrity.Validator[[]byte] {
	n := namer.NewDefaultNamer(prefix, nil, nil)
	m := rawBytesMarshaller{}

	return integrity.NewValidator[[]byte](n, m, nil, nil)
}

// NewRawGenerator creates an integrity.Generator[[]byte] with no hashers or
// signers. It generates namer-formatted keys for test data.
func NewRawGenerator(prefix string) integrity.Generator[[]byte] {
	n := namer.NewDefaultNamer(prefix, nil, nil)
	m := rawBytesMarshaller{}

	return integrity.NewGenerator[[]byte](n, m, nil, nil)
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
