package collectors_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tarantool/go-config"
	"github.com/tarantool/go-config/collectors"
	"github.com/tarantool/go-config/internal/testutil"
	"github.com/tarantool/go-config/storage"
)

func TestNewStorage(t *testing.T) {
	t.Parallel()

	mock := testutil.NewMockStorage()
	collector := collectors.NewStorage(mock, "/config/", collectors.NewYamlFormat())

	require.NotNil(t, collector)
	assert.Equal(t, "storage", collector.Name())
	assert.Equal(t, config.StorageSource, collector.Source())
	assert.Equal(t, config.RevisionType(""), collector.Revision())
	assert.False(t, collector.KeepOrder())
}

func TestStorage_WithName(t *testing.T) {
	t.Parallel()

	mock := testutil.NewMockStorage()
	collector := collectors.NewStorage(mock, "/config/", collectors.NewYamlFormat()).
		WithName("etcd")
	assert.Equal(t, "etcd", collector.Name())
}

func TestStorage_WithSourceType(t *testing.T) {
	t.Parallel()

	mock := testutil.NewMockStorage()
	collector := collectors.NewStorage(mock, "/config/", collectors.NewYamlFormat()).
		WithSourceType(config.FileSource)
	assert.Equal(t, config.FileSource, collector.Source())
}

func TestStorage_WithRevision(t *testing.T) {
	t.Parallel()

	mock := testutil.NewMockStorage()
	collector := collectors.NewStorage(mock, "/config/", collectors.NewYamlFormat()).
		WithRevision("v2")
	assert.Equal(t, config.RevisionType("v2"), collector.Revision())
}

func TestStorage_WithKeepOrder(t *testing.T) {
	t.Parallel()

	mock := testutil.NewMockStorage()
	collector := collectors.NewStorage(mock, "/config/", collectors.NewYamlFormat()).
		WithKeepOrder(true)
	assert.True(t, collector.KeepOrder())
}

func TestStorage_WithDelimiter(t *testing.T) {
	t.Parallel()

	kvs := []storage.KeyValue{
		{Key: []byte("config.a.b"), Value: []byte("value"), ModRevision: 1},
	}
	mock := testutil.NewMockStorage().WithRangeResponse(kvs)
	collector := collectors.NewStorage(mock, "config.", collectors.NewYamlFormat()).
		WithDelimiter(".")

	ctx := context.Background()
	channel := collector.Read(ctx)

	values := make([]config.Value, 0, 1)
	for val := range channel {
		values = append(values, val)
	}

	assert.Len(t, values, 1)
	assert.Equal(t, "a/b", values[0].Meta().Key.String())
}

func TestStorage_Read_SingleKey(t *testing.T) {
	t.Parallel()

	kvs := []storage.KeyValue{
		{Key: []byte("/config/app"), Value: []byte("port: 8080\nhost: localhost"), ModRevision: 3},
	}
	mock := testutil.NewMockStorage().WithRangeResponse(kvs)
	collector := collectors.NewStorage(mock, "/config/", collectors.NewYamlFormat())

	ctx := context.Background()
	channel := collector.Read(ctx)

	values := make([]config.Value, 0, 2)
	for val := range channel {
		values = append(values, val)
	}

	assert.Len(t, values, 2)

	var port int

	err := values[0].Get(&port)
	require.NoError(t, err)
	assert.Equal(t, 8080, port)

	var host string

	err = values[1].Get(&host)
	require.NoError(t, err)
	assert.Equal(t, "localhost", host)

	assert.Equal(t, config.RevisionType("3"), collector.Revision())
}

func TestStorage_Read_MultipleKeys(t *testing.T) {
	t.Parallel()

	kvs := []storage.KeyValue{
		{Key: []byte("/config/instances/i001"), Value: []byte("roles:\n  - router\nmemory: 1G"), ModRevision: 5},
		{Key: []byte("/config/instances/i002"), Value: []byte("roles:\n  - storage\nmemory: 2G"), ModRevision: 8},
	}
	mock := testutil.NewMockStorage().WithRangeResponse(kvs)
	collector := collectors.NewStorage(mock, "/config/", collectors.NewYamlFormat())

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

	assert.Equal(t, config.RevisionType("8"), collector.Revision())
}

func TestStorage_Read_NestedYaml(t *testing.T) {
	t.Parallel()

	yamlValue := "a:\n  b:\n    c: deep\n    d:\n      - 1\n      - 2"
	kvs := []storage.KeyValue{
		{Key: []byte("/config/deep"), Value: []byte(yamlValue), ModRevision: 1},
	}
	mock := testutil.NewMockStorage().WithRangeResponse(kvs)
	collector := collectors.NewStorage(mock, "/config/", collectors.NewYamlFormat())

	ctx := context.Background()
	channel := collector.Read(ctx)

	values := make([]config.Value, 0, 3)
	for val := range channel {
		values = append(values, val)
	}

	assert.Len(t, values, 3)

	var deep string

	err := values[0].Get(&deep)
	require.NoError(t, err)
	assert.Equal(t, "deep", deep)
}

func TestStorage_Read_EmptyRange(t *testing.T) {
	t.Parallel()

	mock := testutil.NewMockStorage().WithRangeResponse([]storage.KeyValue{})
	collector := collectors.NewStorage(mock, "/config/", collectors.NewYamlFormat())

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

	mock := testutil.NewMockStorage().WithRangeError(storage.ErrRangeFailed)
	collector := collectors.NewStorage(mock, "/config/", collectors.NewYamlFormat())

	ctx := context.Background()
	channel := collector.Read(ctx)

	var count int
	for range channel {
		count++
	}

	assert.Equal(t, 0, count)
}

func TestStorage_Read_InvalidYamlValue(t *testing.T) {
	t.Parallel()

	kvs := []storage.KeyValue{
		{Key: []byte("/config/invalid"), Value: []byte("bad: yaml: [unclosed"), ModRevision: 1},
		{Key: []byte("/config/valid"), Value: []byte("key: value"), ModRevision: 2},
	}
	mock := testutil.NewMockStorage().WithRangeResponse(kvs)
	collector := collectors.NewStorage(mock, "/config/", collectors.NewYamlFormat())

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

	kvs := []storage.KeyValue{
		{Key: []byte("/config/empty"), Value: []byte{}, ModRevision: 1},
		{Key: []byte("/config/valid"), Value: []byte("key: value"), ModRevision: 2},
	}
	mock := testutil.NewMockStorage().WithRangeResponse(kvs)
	collector := collectors.NewStorage(mock, "/config/", collectors.NewYamlFormat())

	ctx := context.Background()
	channel := collector.Read(ctx)

	values := make([]config.Value, 0, 1)
	for val := range channel {
		values = append(values, val)
	}

	assert.Len(t, values, 1)
}

func TestStorage_Read_PrefixStripping(t *testing.T) {
	t.Parallel()

	kvs := []storage.KeyValue{
		{Key: []byte("/config/a/b"), Value: []byte("key: value"), ModRevision: 1},
	}
	mock := testutil.NewMockStorage().WithRangeResponse(kvs)
	collector := collectors.NewStorage(mock, "/config/", collectors.NewYamlFormat())

	ctx := context.Background()
	channel := collector.Read(ctx)

	values := make([]config.Value, 0, 1)
	for val := range channel {
		values = append(values, val)
	}

	assert.Len(t, values, 1)
	assert.Equal(t, "a/b/key", values[0].Meta().Key.String())
}

func TestStorage_Read_CustomDelimiter(t *testing.T) {
	t.Parallel()

	kvs := []storage.KeyValue{
		{Key: []byte("config.a.b"), Value: []byte("key: value"), ModRevision: 1},
	}
	mock := testutil.NewMockStorage().WithRangeResponse(kvs)
	collector := collectors.NewStorage(mock, "config.", collectors.NewYamlFormat()).
		WithDelimiter(".")

	ctx := context.Background()
	channel := collector.Read(ctx)

	values := make([]config.Value, 0, 1)
	for val := range channel {
		values = append(values, val)
	}

	assert.Len(t, values, 1)
	assert.Equal(t, "a/b/key", values[0].Meta().Key.String())
}

func TestStorage_Read_RevisionMaxTracking(t *testing.T) {
	t.Parallel()

	kvs := []storage.KeyValue{
		{Key: []byte("/config/a"), Value: []byte("x: 1"), ModRevision: 3},
		{Key: []byte("/config/b"), Value: []byte("x: 2"), ModRevision: 7},
		{Key: []byte("/config/c"), Value: []byte("x: 3"), ModRevision: 2},
		{Key: []byte("/config/d"), Value: []byte("x: 4"), ModRevision: 9},
		{Key: []byte("/config/e"), Value: []byte("x: 5"), ModRevision: 1},
	}
	mock := testutil.NewMockStorage().WithRangeResponse(kvs)
	collector := collectors.NewStorage(mock, "/config/", collectors.NewYamlFormat())

	ctx := context.Background()

	channel := collector.Read(ctx)
	for range channel { //nolint:revive // empty-block is intentional for draining
	}

	assert.Equal(t, config.RevisionType("9"), collector.Revision())
}

func TestStorage_Read_RevisionZero(t *testing.T) {
	t.Parallel()

	kvs := []storage.KeyValue{
		{Key: []byte("/config/a"), Value: []byte("x: 1"), ModRevision: 0},
	}
	mock := testutil.NewMockStorage().WithRangeResponse(kvs)
	collector := collectors.NewStorage(mock, "/config/", collectors.NewYamlFormat())

	ctx := context.Background()

	channel := collector.Read(ctx)
	for range channel { //nolint:revive // empty-block is intentional for draining
	}

	assert.Equal(t, config.RevisionType("0"), collector.Revision())
}

func TestStorage_Read_Cancellation(t *testing.T) {
	t.Parallel()

	kvs := []storage.KeyValue{
		{Key: []byte("/config/a"), Value: []byte("x: 1"), ModRevision: 1},
		{Key: []byte("/config/b"), Value: []byte("x: 2"), ModRevision: 2},
	}
	mock := testutil.NewMockStorage().WithRangeResponse(kvs)
	collector := collectors.NewStorage(mock, "/config/", collectors.NewYamlFormat())

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
	collector := collectors.NewStorage(mock, "/config/", collectors.NewYamlFormat())

	mock.RangeResponse = []storage.KeyValue{
		{Key: []byte("/config/key"), Value: []byte("value: v1"), ModRevision: 1},
	}

	ctx := context.Background()
	channel := collector.Read(ctx)

	values := make([]config.Value, 0, 1)
	for val := range channel {
		values = append(values, val)
	}

	assert.Len(t, values, 1)

	mock.RangeResponse = []storage.KeyValue{
		{Key: []byte("/config/key"), Value: []byte("value: v2"), ModRevision: 2},
	}

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

	kvs := []storage.KeyValue{
		{Key: []byte("/config/app/port"), Value: []byte("value: 8080"), ModRevision: 5},
	}
	mock := testutil.NewMockStorage().WithRangeResponse(kvs)
	storageCollector := collectors.NewStorage(mock, "/config/", collectors.NewYamlFormat())

	builder := config.NewBuilder()

	builder = builder.AddCollector(storageCollector)

	cfg, errs := builder.Build()
	assert.Empty(t, errs)

	var port string

	_, err := cfg.Get(config.NewKeyPath("app/port/value"), &port)
	require.NoError(t, err)
	assert.Equal(t, "8080", port)
}

func TestStorage_Builder_WithOtherCollectors(t *testing.T) {
	t.Parallel()

	kvs := []storage.KeyValue{
		{Key: []byte("/config/key"), Value: []byte("value: storage-value"), ModRevision: 1},
	}
	mock := testutil.NewMockStorage().WithRangeResponse(kvs)
	storageCollector := collectors.NewStorage(mock, "/config/", collectors.NewYamlFormat())

	mapCollector := collectors.NewMap(map[string]any{
		"key/value": "map-value",
	})

	builder := config.NewBuilder()

	builder = builder.AddCollector(mapCollector)
	builder = builder.AddCollector(storageCollector)

	cfg, errs := builder.Build()
	assert.Empty(t, errs)

	var val string

	_, err := cfg.Get(config.NewKeyPath("key/value"), &val)
	require.NoError(t, err)
	assert.Equal(t, "storage-value", val)
}

// ExampleNewStorage demonstrates how to use the Storage collector to read
// multiple configuration documents from a key-value storage under a prefix.
func ExampleNewStorage() {
	ctx := context.Background()

	// Create a mock storage with several key-value pairs under /config/.
	kvs := []storage.KeyValue{
		{
			Key:         []byte("/config/server/host"),
			Value:       []byte("localhost"),
			ModRevision: 1,
		},
		{
			Key:         []byte("/config/server/port"),
			Value:       []byte("8080"),
			ModRevision: 2,
		},
		{
			Key:         []byte("/config/database/url"),
			Value:       []byte("postgres://localhost:5432/app"),
			ModRevision: 3,
		},
	}
	mock := testutil.NewMockStorage().WithRangeResponse(kvs)

	// Create a Storage collector for prefix "/config/" using YAML format.
	// (Each value is a plain YAML scalar; the format can parse them.)
	collector := collectors.NewStorage(mock, "/config/", collectors.NewYamlFormat())

	// Read configuration values.
	for val := range collector.Read(ctx) {
		var dest any

		err := val.Get(&dest)
		if err != nil {
			fmt.Printf("error getting value: %v\n", err)
			continue
		}

		fmt.Printf("%s = %v\n", val.Meta().Key, dest)
	}

	// Output:
	// server/host = localhost
	// server/port = 8080
	// database/url = postgres://localhost:5432/app
}
