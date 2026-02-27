package collectors_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tarantool/go-config"
	"github.com/tarantool/go-config/collectors"
	"github.com/tarantool/go-config/internal/testutil"
	"github.com/tarantool/go-config/storage"
)

var errTestTxFailure = errors.New("tx failed")

func TestNewStorageSource(t *testing.T) {
	t.Parallel()

	mock := testutil.NewMockStorage()
	source := collectors.NewStorageSource(mock, []byte("/config/app"))
	require.NotNil(t, source)
	assert.Equal(t, "storage", source.Name())
	assert.Equal(t, config.StorageSource, source.SourceType())
	assert.Equal(t, config.RevisionType(""), source.Revision())
}

func TestStorageSource_Name(t *testing.T) {
	t.Parallel()

	mock := testutil.NewMockStorage()
	source := collectors.NewStorageSource(mock, []byte("/key"))
	assert.Equal(t, "storage", source.Name())
}

func TestStorageSource_SourceType(t *testing.T) {
	t.Parallel()

	mock := testutil.NewMockStorage()
	source := collectors.NewStorageSource(mock, []byte("/key"))
	assert.Equal(t, config.StorageSource, source.SourceType())
}

func TestStorageSource_Revision(t *testing.T) {
	t.Parallel()

	mock := testutil.NewMockStorage()
	source := collectors.NewStorageSource(mock, []byte("/key"))
	assert.Equal(t, config.RevisionType(""), source.Revision())
}

func TestStorageSource_FetchStream(t *testing.T) {
	t.Parallel()

	yamlBytes := []byte("server:\n  port: 8080\n  host: localhost")
	resp := storage.Response{
		Results: []storage.Result{
			{
				Values: []storage.KeyValue{
					{Key: []byte("/config/app"), Value: yamlBytes, ModRevision: 5},
				},
			},
		},
	}

	mock := testutil.NewMockStorage().WithTxResponse(resp)
	source := collectors.NewStorageSource(mock, []byte("/config/app"))

	ctx := context.Background()
	reader, err := source.FetchStream(ctx)
	require.NoError(t, err)

	defer func() { _ = reader.Close() }()

	data, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.True(t, bytes.Equal(yamlBytes, data))
	assert.Equal(t, config.RevisionType("5"), source.Revision())
}

func TestStorageSource_FetchStream_KeyNotFound(t *testing.T) {
	t.Parallel()

	resp := storage.Response{
		Results: []storage.Result{
			{Values: []storage.KeyValue{}},
		},
	}

	mock := testutil.NewMockStorage().WithTxResponse(resp)
	source := collectors.NewStorageSource(mock, []byte("/config/missing"))

	ctx := context.Background()
	_, err := source.FetchStream(ctx)
	require.Error(t, err)
	assert.ErrorIs(t, err, collectors.ErrStorageKeyNotFound)
}

func TestStorageSource_FetchStream_TxError(t *testing.T) {
	t.Parallel()

	mock := testutil.NewMockStorage().WithTxError(errTestTxFailure)
	source := collectors.NewStorageSource(mock, []byte("/config/app"))

	ctx := context.Background()
	_, err := source.FetchStream(ctx)
	require.Error(t, err)
	assert.ErrorIs(t, err, collectors.ErrStorageFetch)
}

func TestStorageSource_FetchStream_EmptyValue(t *testing.T) {
	t.Parallel()

	resp := storage.Response{
		Results: []storage.Result{
			{
				Values: []storage.KeyValue{
					{Key: []byte("/config/empty"), Value: []byte{}, ModRevision: 1},
				},
			},
		},
	}

	mock := testutil.NewMockStorage().WithTxResponse(resp)
	source := collectors.NewStorageSource(mock, []byte("/config/empty"))

	ctx := context.Background()
	reader, err := source.FetchStream(ctx)
	require.NoError(t, err)

	defer func() { _ = reader.Close() }()

	data, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Empty(t, data)
}

func TestStorageSource_FetchStream_RevisionUpdated(t *testing.T) {
	t.Parallel()

	yamlBytes := []byte("key: value")
	mock := testutil.NewMockStorage()
	source := collectors.NewStorageSource(mock, []byte("/config/app"))

	mock.TxResponse = storage.Response{
		Results: []storage.Result{
			{
				Values: []storage.KeyValue{
					{Key: []byte("/config/app"), Value: yamlBytes, ModRevision: 7},
				},
			},
		},
	}

	ctx := context.Background()
	reader, err := source.FetchStream(ctx)
	require.NoError(t, err)

	_ = reader.Close()

	assert.Equal(t, config.RevisionType("7"), source.Revision())

	mock.TxResponse = storage.Response{
		Results: []storage.Result{
			{
				Values: []storage.KeyValue{
					{Key: []byte("/config/app"), Value: yamlBytes, ModRevision: 12},
				},
			},
		},
	}

	reader, err = source.FetchStream(ctx)
	require.NoError(t, err)

	_ = reader.Close()

	assert.Equal(t, config.RevisionType("12"), source.Revision())
}

func TestStorageSource_WithSource_YamlFormat(t *testing.T) {
	t.Parallel()

	yamlBytes := []byte("server:\n  port: 8080\n  host: localhost")
	resp := storage.Response{
		Results: []storage.Result{
			{
				Values: []storage.KeyValue{
					{Key: []byte("/config/app"), Value: yamlBytes, ModRevision: 1},
				},
			},
		},
	}

	mock := testutil.NewMockStorage().WithTxResponse(resp)
	source := collectors.NewStorageSource(mock, []byte("/config/app"))

	collector, err := collectors.NewSource(source, collectors.NewYamlFormat())
	require.NoError(t, err)

	ctx := context.Background()
	channel := collector.Read(ctx)

	values := make([]config.Value, 0, 2)
	for val := range channel {
		values = append(values, val)
	}

	assert.Len(t, values, 2)

	var port int

	err = values[0].Get(&port)
	require.NoError(t, err)
	assert.Equal(t, 8080, port)

	var host string

	err = values[1].Get(&host)
	require.NoError(t, err)
	assert.Equal(t, "localhost", host)
}

func TestStorageSource_WithSource_NestedYaml(t *testing.T) {
	t.Parallel()

	yamlBytes := []byte("a:\n  b:\n    c: deep\n    d:\n      - 1\n      - 2")
	resp := storage.Response{
		Results: []storage.Result{
			{
				Values: []storage.KeyValue{
					{Key: []byte("/config/nested"), Value: yamlBytes, ModRevision: 1},
				},
			},
		},
	}

	mock := testutil.NewMockStorage().WithTxResponse(resp)
	source := collectors.NewStorageSource(mock, []byte("/config/nested"))

	collector, err := collectors.NewSource(source, collectors.NewYamlFormat())
	require.NoError(t, err)

	ctx := context.Background()
	channel := collector.Read(ctx)

	values := make([]config.Value, 0, 3)
	for val := range channel {
		values = append(values, val)
	}

	assert.Len(t, values, 3)

	var deep string

	err = values[0].Get(&deep)
	require.NoError(t, err)
	assert.Equal(t, "deep", deep)
}

func TestStorageSource_WithSource_InvalidYaml(t *testing.T) {
	t.Parallel()

	invalidYaml := []byte("bad: yaml: content: [unclosed")
	resp := storage.Response{
		Results: []storage.Result{
			{
				Values: []storage.KeyValue{
					{Key: []byte("/config/invalid"), Value: invalidYaml, ModRevision: 1},
				},
			},
		},
	}

	mock := testutil.NewMockStorage().WithTxResponse(resp)
	source := collectors.NewStorageSource(mock, []byte("/config/invalid"))

	_, err := collectors.NewSource(source, collectors.NewYamlFormat())
	require.Error(t, err)
	assert.ErrorIs(t, err, collectors.ErrFormatParse)
}

// ExampleNewStorageSource demonstrates how to use StorageSource to fetch
// a single configuration document from a key-value storage.
func ExampleNewStorageSource() {
	ctx := context.Background()

	// Create a mock storage with a sample configuration.
	resp := storage.Response{
		Results: []storage.Result{
			{
				Values: []storage.KeyValue{
					{
						Key:         []byte("/config/app"),
						Value:       []byte("server:\n  port: 8080\n  host: localhost"),
						ModRevision: 42,
					},
				},
			},
		},
	}
	mock := testutil.NewMockStorage().WithTxResponse(resp)

	// Create a StorageSource for the key "/config/app".
	src := collectors.NewStorageSource(mock, []byte("/config/app"))

	// Combine with a YAML format parser.
	source, err := collectors.NewSource(src, collectors.NewYamlFormat())
	if err != nil {
		// In a real application you would handle the error appropriately.
		fmt.Printf("error creating source: %v\n", err)
		return
	}

	// Read configuration values.
	for val := range source.Read(ctx) {
		var dest any

		err := val.Get(&dest)
		if err != nil {
			fmt.Printf("error getting value: %v\n", err)
			continue
		}

		fmt.Printf("%s = %v\n", val.Meta().Key, dest)
	}

	// Output:
	// server/port = 8080
	// server/host = localhost
}
