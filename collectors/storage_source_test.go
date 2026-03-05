package collectors_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tarantool/go-config"
	"github.com/tarantool/go-config/collectors"
	"github.com/tarantool/go-config/internal/testutil"
)

var errTestTxFailure = errors.New("tx failed")

func TestNewStorageSource(t *testing.T) {
	t.Parallel()

	mock := testutil.NewMockStorage()
	source := collectors.NewStorageSource(mock, "/config/", "app", nil, nil)
	require.NotNil(t, source)
	assert.Equal(t, "storage", source.Name())
	assert.Equal(t, config.StorageSource, source.SourceType())
	assert.Equal(t, config.RevisionType(""), source.Revision())
}

func TestStorageSource_Name(t *testing.T) {
	t.Parallel()

	mock := testutil.NewMockStorage()
	source := collectors.NewStorageSource(mock, "/config/", "key", nil, nil)
	assert.Equal(t, "storage", source.Name())
}

func TestStorageSource_SourceType(t *testing.T) {
	t.Parallel()

	mock := testutil.NewMockStorage()
	source := collectors.NewStorageSource(mock, "/config/", "key", nil, nil)
	assert.Equal(t, config.StorageSource, source.SourceType())
}

func TestStorageSource_Revision(t *testing.T) {
	t.Parallel()

	mock := testutil.NewMockStorage()
	source := collectors.NewStorageSource(mock, "/config/", "key", nil, nil)
	assert.Equal(t, config.RevisionType(""), source.Revision())
}

func TestStorageSource_FetchStream(t *testing.T) {
	t.Parallel()

	yamlBytes := []byte("server:\n  port: 8080\n  host: localhost")
	mock := testutil.NewMockStorage()
	testutil.PutIntegrity(mock, "/config/", "app", yamlBytes)

	source := collectors.NewStorageSource(mock, "/config/", "app", nil, nil)

	ctx := context.Background()
	reader, err := source.FetchStream(ctx)
	require.NoError(t, err)

	defer func() { _ = reader.Close() }()

	data, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.True(t, bytes.Equal(yamlBytes, data))
	assert.NotEmpty(t, source.Revision())
}

func TestStorageSource_FetchStream_KeyNotFound(t *testing.T) {
	t.Parallel()

	mock := testutil.NewMockStorage()
	source := collectors.NewStorageSource(mock, "/config/", "missing", nil, nil)

	ctx := context.Background()
	_, err := source.FetchStream(ctx)
	require.Error(t, err)
	assert.ErrorIs(t, err, collectors.ErrStorageKeyNotFound)
}

func TestStorageSource_FetchStream_TxError(t *testing.T) {
	t.Parallel()

	mock := testutil.NewMockStorage().WithTxError(errTestTxFailure)
	source := collectors.NewStorageSource(mock, "/config/", "app", nil, nil)

	ctx := context.Background()
	_, err := source.FetchStream(ctx)
	require.Error(t, err)
	assert.ErrorIs(t, err, collectors.ErrStorageFetch)
}

func TestStorageSource_FetchStream_EmptyValue(t *testing.T) {
	t.Parallel()

	mock := testutil.NewMockStorage()
	testutil.PutIntegrity(mock, "/config/", "empty", []byte{})

	source := collectors.NewStorageSource(mock, "/config/", "empty", nil, nil)

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
	testutil.PutIntegrity(mock, "/config/", "app", yamlBytes)

	source := collectors.NewStorageSource(mock, "/config/", "app", nil, nil)

	ctx := context.Background()
	reader, err := source.FetchStream(ctx)
	require.NoError(t, err)

	_ = reader.Close()

	rev1 := source.Revision()
	assert.NotEmpty(t, rev1)

	// Update with new data (re-put overwrites with higher revision).
	testutil.PutIntegrity(mock, "/config/", "app", []byte("key: updated"))

	reader, err = source.FetchStream(ctx)
	require.NoError(t, err)

	_ = reader.Close()

	rev2 := source.Revision()
	assert.NotEqual(t, rev1, rev2)
}

func TestStorageSource_WithSource_YamlFormat(t *testing.T) {
	t.Parallel()

	yamlBytes := []byte("server:\n  port: 8080\n  host: localhost")
	mock := testutil.NewMockStorage()
	testutil.PutIntegrity(mock, "/config/", "app", yamlBytes)

	source := collectors.NewStorageSource(mock, "/config/", "app", nil, nil)

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
	mock := testutil.NewMockStorage()
	testutil.PutIntegrity(mock, "/config/", "nested", yamlBytes)

	source := collectors.NewStorageSource(mock, "/config/", "nested", nil, nil)

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
	mock := testutil.NewMockStorage()
	testutil.PutIntegrity(mock, "/config/", "invalid", invalidYaml)

	source := collectors.NewStorageSource(mock, "/config/", "invalid", nil, nil)

	_, err := collectors.NewSource(source, collectors.NewYamlFormat())
	require.Error(t, err)
	assert.ErrorIs(t, err, collectors.ErrFormatParse)
}
