package collectors_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tarantool/go-config"
	"github.com/tarantool/go-config/collectors"
	"github.com/tarantool/go-config/internal/testutil"
)

func TestNewStorage(t *testing.T) {
	t.Parallel()

	mock := testutil.NewMockStorage()
	typed := testutil.NewRawTyped(mock, "/config/")
	collector := collectors.NewStorage(typed, "/config/", collectors.NewYamlFormat())

	require.NotNil(t, collector)
	assert.Equal(t, "storage", collector.Name())
	assert.Equal(t, config.StorageSource, collector.Source())
	assert.Equal(t, config.RevisionType(""), collector.Revision())
	assert.False(t, collector.KeepOrder())
}

func TestStorage_WithName(t *testing.T) {
	t.Parallel()

	mock := testutil.NewMockStorage()
	typed := testutil.NewRawTyped(mock, "/config/")
	collector := collectors.NewStorage(typed, "/config/", collectors.NewYamlFormat()).
		WithName("etcd")
	assert.Equal(t, "etcd", collector.Name())
}

func TestStorage_WithSourceType(t *testing.T) {
	t.Parallel()

	mock := testutil.NewMockStorage()
	typed := testutil.NewRawTyped(mock, "/config/")
	collector := collectors.NewStorage(typed, "/config/", collectors.NewYamlFormat()).
		WithSourceType(config.FileSource)
	assert.Equal(t, config.FileSource, collector.Source())
}

func TestStorage_WithRevision(t *testing.T) {
	t.Parallel()

	mock := testutil.NewMockStorage()
	typed := testutil.NewRawTyped(mock, "/config/")
	collector := collectors.NewStorage(typed, "/config/", collectors.NewYamlFormat()).
		WithRevision("v2")
	assert.Equal(t, config.RevisionType("v2"), collector.Revision())
}

func TestStorage_WithKeepOrder(t *testing.T) {
	t.Parallel()

	mock := testutil.NewMockStorage()
	typed := testutil.NewRawTyped(mock, "/config/")
	collector := collectors.NewStorage(typed, "/config/", collectors.NewYamlFormat()).
		WithKeepOrder(true)
	assert.True(t, collector.KeepOrder())
}

func TestStorage_Read_SingleKey(t *testing.T) {
	t.Parallel()

	mock := testutil.NewMockStorage()
	testutil.PutIntegrity(mock, "/config/", "app", []byte("port: 8080\nhost: localhost"))

	typed := testutil.NewRawTyped(mock, "/config/")
	collector := collectors.NewStorage(typed, "/config/", collectors.NewYamlFormat())

	ctx := context.Background()
	channel := collector.Read(ctx)

	values := make([]config.Value, 0, 2)
	for val := range channel {
		values = append(values, val)
	}

	assert.Len(t, values, 2)

	valuesMap := make(map[string]any)

	for _, val := range values {
		var dest any

		err := val.Get(&dest)
		require.NoError(t, err)

		valuesMap[val.Meta().Key.String()] = dest
		assert.Equal(t, "storage:/config/app", val.Meta().Source.Name)
	}

	assert.Equal(t, int64(8080), valuesMap["port"])
	assert.Equal(t, "localhost", valuesMap["host"])
	assert.NotEmpty(t, collector.Revision())
}

func TestStorage_Read_SourceName_WithName(t *testing.T) {
	t.Parallel()

	mock := testutil.NewMockStorage()
	testutil.PutIntegrity(mock, "/config/", "app", []byte("port: 8080"))

	typed := testutil.NewRawTyped(mock, "/config/")
	collector := collectors.NewStorage(typed, "/config/", collectors.NewYamlFormat()).
		WithName("etcd")

	ctx := context.Background()
	channel := collector.Read(ctx)

	val := <-channel
	assert.Equal(t, "etcd:/config/app", val.Meta().Source.Name)
}

func TestStorage_Read_SourceName_EmptyName(t *testing.T) {
	t.Parallel()

	mock := testutil.NewMockStorage()
	testutil.PutIntegrity(mock, "/config/", "app", []byte("port: 8080"))

	typed := testutil.NewRawTyped(mock, "/config/")
	collector := collectors.NewStorage(typed, "/config/", collectors.NewYamlFormat()).
		WithName("")

	ctx := context.Background()
	channel := collector.Read(ctx)

	val := <-channel
	assert.Equal(t, ":/config/app", val.Meta().Source.Name)
}

func TestStorage_Read_SourceName_MultipleKeys(t *testing.T) {
	t.Parallel()

	mock := testutil.NewMockStorage()
	testutil.PutIntegrity(mock, "/config/", "db", []byte("dbhost: localhost"))
	testutil.PutIntegrity(mock, "/config/", "cache", []byte("cachehost: redis"))

	typed := testutil.NewRawTyped(mock, "/config/")
	collector := collectors.NewStorage(typed, "/config/", collectors.NewYamlFormat()).
		WithName("etcd")

	ctx := context.Background()
	channel := collector.Read(ctx)

	sourceNames := make(map[string]string)

	for val := range channel {
		sourceNames[val.Meta().Key.String()] = val.Meta().Source.Name
	}

	assert.Len(t, sourceNames, 2)
	assert.Equal(t, "etcd:/config/db", sourceNames["dbhost"])
	assert.Equal(t, "etcd:/config/cache", sourceNames["cachehost"])
}

func TestStorage_Read_MultipleKeys(t *testing.T) {
	t.Parallel()

	mock := testutil.NewMockStorage()
	testutil.PutIntegrity(mock, "/config/", "cfg-instances", []byte(
		"instances:\n  i001:\n    roles:\n      - router\n    memory: 1G\n"+
			"  i002:\n    roles:\n      - storage\n    memory: 2G"))
	testutil.PutIntegrity(mock, "/config/", "cfg-global",
		[]byte("replication:\n  failover: manual"))

	typed := testutil.NewRawTyped(mock, "/config/")
	collector := collectors.NewStorage(typed, "/config/", collectors.NewYamlFormat())

	ctx := context.Background()
	channel := collector.Read(ctx)

	valuesMap := make(map[string]any)

	for val := range channel {
		var dest any

		err := val.Get(&dest)
		require.NoError(t, err)

		valuesMap[val.Meta().Key.String()] = dest
	}

	assert.Equal(t, "router", valuesMap["instances/i001/roles/0"])
	assert.Equal(t, "1G", valuesMap["instances/i001/memory"])
	assert.Equal(t, "storage", valuesMap["instances/i002/roles/0"])
	assert.Equal(t, "2G", valuesMap["instances/i002/memory"])
	assert.Equal(t, "manual", valuesMap["replication/failover"])
}

func TestStorage_Read_NestedYaml(t *testing.T) {
	t.Parallel()

	yamlValue := "a:\n  b:\n    c: deep\n    d:\n      - 1\n      - 2"
	mock := testutil.NewMockStorage()
	testutil.PutIntegrity(mock, "/config/", "nested-doc", []byte(yamlValue))

	typed := testutil.NewRawTyped(mock, "/config/")
	collector := collectors.NewStorage(typed, "/config/", collectors.NewYamlFormat())

	ctx := context.Background()
	channel := collector.Read(ctx)

	valuesMap := make(map[string]any)

	for val := range channel {
		var dest any

		err := val.Get(&dest)
		require.NoError(t, err)

		valuesMap[val.Meta().Key.String()] = dest
	}

	assert.Len(t, valuesMap, 3)
	assert.Equal(t, "deep", valuesMap["a/b/c"])
	assert.Equal(t, int64(1), valuesMap["a/b/d/0"])
	assert.Equal(t, int64(2), valuesMap["a/b/d/1"])
}

func TestStorage_Read_EmptyRange(t *testing.T) {
	t.Parallel()

	mock := testutil.NewMockStorage()
	typed := testutil.NewRawTyped(mock, "/config/")
	collector := collectors.NewStorage(typed, "/config/", collectors.NewYamlFormat())

	ctx := context.Background()
	channel := collector.Read(ctx)

	var count int
	for range channel {
		count++
	}

	assert.Equal(t, 0, count)
}

func TestStorage_Read_RangeError(t *testing.T) {
	t.Parallel()

	mock := testutil.NewMockStorage().WithTxError(errTestTxFailure)
	typed := testutil.NewRawTyped(mock, "/config/")
	collector := collectors.NewStorage(typed, "/config/", collectors.NewYamlFormat())

	ctx := context.Background()
	channel := collector.Read(ctx)

	var count int
	for range channel {
		count++
	}

	assert.Equal(t, 0, count)
}

func TestStorage_Collectors_InvalidYamlValue_Strict(t *testing.T) {
	t.Parallel()

	mock := testutil.NewMockStorage()
	testutil.PutIntegrity(mock, "/config/", "invalid", []byte("bad: yaml: [unclosed"))
	testutil.PutIntegrity(mock, "/config/", "valid", []byte("mykey: value"))

	typed := testutil.NewRawTyped(mock, "/config/")
	collector := collectors.NewStorage(typed, "/config/", collectors.NewYamlFormat())

	assert.False(t, collector.SkipInvalid())

	subs, err := collector.Collectors(context.Background())

	var fpErr *collectors.FormatParseError

	require.ErrorAs(t, err, &fpErr)
	assert.Equal(t, "/config/invalid", fpErr.Key)
	assert.Nil(t, subs)
}

func TestStorage_Builder_InvalidYamlValue_Strict(t *testing.T) {
	t.Parallel()

	mock := testutil.NewMockStorage()
	testutil.PutIntegrity(mock, "/config/", "invalid", []byte("bad: yaml: [unclosed"))
	testutil.PutIntegrity(mock, "/config/", "valid", []byte("mykey: value"))

	typed := testutil.NewRawTyped(mock, "/config/")
	collector := collectors.NewStorage(typed, "/config/", collectors.NewYamlFormat())

	builder := config.NewBuilder()

	builder = builder.AddCollector(collector)

	_, errs := builder.Build(t.Context())
	require.NotEmpty(t, errs)

	var fpErr *collectors.FormatParseError

	var matched bool

	for _, e := range errs {
		if errors.As(e, &fpErr) {
			matched = true

			break
		}
	}

	require.True(t, matched, "expected *FormatParseError among builder errors: %v", errs)
	assert.Equal(t, "/config/invalid", fpErr.Key)
}

func TestStorage_Read_InvalidYamlValue_SkipInvalid(t *testing.T) {
	t.Parallel()

	mock := testutil.NewMockStorage()
	testutil.PutIntegrity(mock, "/config/", "invalid", []byte("bad: yaml: [unclosed"))
	testutil.PutIntegrity(mock, "/config/", "valid", []byte("mykey: value"))

	typed := testutil.NewRawTyped(mock, "/config/")
	collector := collectors.NewStorage(typed, "/config/", collectors.NewYamlFormat()).
		WithSkipInvalid(true)

	assert.True(t, collector.SkipInvalid())

	ctx := context.Background()
	channel := collector.Read(ctx)

	values := make([]config.Value, 0, 1)
	for val := range channel {
		values = append(values, val)
	}

	assert.Len(t, values, 1)

	var val string

	err := values[0].Get(&val)
	require.NoError(t, err)
	assert.Equal(t, "value", val)
}

func TestStorage_Read_EmptyYamlValue(t *testing.T) {
	t.Parallel()

	mock := testutil.NewMockStorage()
	testutil.PutIntegrity(mock, "/config/", "empty", []byte{})
	testutil.PutIntegrity(mock, "/config/", "valid", []byte("key: value"))

	typed := testutil.NewRawTyped(mock, "/config/")
	collector := collectors.NewStorage(typed, "/config/", collectors.NewYamlFormat())

	ctx := context.Background()
	channel := collector.Read(ctx)

	values := make([]config.Value, 0, 1)
	for val := range channel {
		values = append(values, val)
	}

	assert.Len(t, values, 1)
}

func TestStorage_Read_Cancellation(t *testing.T) {
	t.Parallel()

	mock := testutil.NewMockStorage()
	testutil.PutIntegrity(mock, "/config/", "a", []byte("x: 1"))
	testutil.PutIntegrity(mock, "/config/", "b", []byte("x: 2"))

	typed := testutil.NewRawTyped(mock, "/config/")
	collector := collectors.NewStorage(typed, "/config/", collectors.NewYamlFormat())

	ctx, cancel := context.WithCancel(context.Background())
	channel := collector.Read(ctx)

	_, ok := <-channel
	require.True(t, ok)

	cancel()
	testutil.Drain(t, channel)
}

func TestStorage_Watch_ReRead(t *testing.T) {
	t.Parallel()

	mock := testutil.NewMockStorage()
	testutil.PutIntegrity(mock, "/config/", "mykey", []byte("setting: v1"))

	typed := testutil.NewRawTyped(mock, "/config/")
	collector := collectors.NewStorage(typed, "/config/", collectors.NewYamlFormat())

	ctx := context.Background()
	channel := collector.Read(ctx)

	values := make([]config.Value, 0, 1)
	for val := range channel {
		values = append(values, val)
	}

	assert.Len(t, values, 1)

	// Update data and re-read.
	testutil.PutIntegrity(mock, "/config/", "mykey", []byte("setting: v2"))

	channel = collector.Read(ctx)

	values = make([]config.Value, 0, 1)
	for val := range channel {
		values = append(values, val)
	}

	assert.Len(t, values, 1)

	var val string

	err := values[0].Get(&val)
	require.NoError(t, err)
	assert.Equal(t, "v2", val)
}

func TestStorage_Builder_Integration(t *testing.T) {
	t.Parallel()

	mock := testutil.NewMockStorage()
	testutil.PutIntegrity(mock, "/config/", "app-config", []byte("app:\n  port: 8080"))

	typed := testutil.NewRawTyped(mock, "/config/")
	storageCollector := collectors.NewStorage(typed, "/config/", collectors.NewYamlFormat()).
		WithName("etcd")

	builder := config.NewBuilder()

	builder = builder.AddCollector(storageCollector)

	cfg, errs := builder.Build(t.Context())
	assert.Empty(t, errs)

	var port int

	meta, err := cfg.Get(config.NewKeyPath("app/port"), &port)
	require.NoError(t, err)
	assert.Equal(t, 8080, port)
	assert.Equal(t, "etcd:/config/app-config", meta.Source.Name)
}

func TestStorage_Builder_WithOtherCollectors(t *testing.T) {
	t.Parallel()

	mock := testutil.NewMockStorage()
	testutil.PutIntegrity(mock, "/config/", "doc", []byte("setting: storage-value"))

	typed := testutil.NewRawTyped(mock, "/config/")
	storageCollector := collectors.NewStorage(typed, "/config/", collectors.NewYamlFormat())

	mapCollector := collectors.NewMap(map[string]any{
		"setting": "map-value",
	})

	builder := config.NewBuilder()

	builder = builder.AddCollector(mapCollector)
	builder = builder.AddCollector(storageCollector)

	cfg, errs := builder.Build(t.Context())
	assert.Empty(t, errs)

	var val string

	_, err := cfg.Get(config.NewKeyPath("setting"), &val)
	require.NoError(t, err)
	assert.Equal(t, "storage-value", val)
}
