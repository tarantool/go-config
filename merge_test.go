package config_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tarantool/go-config"
	"github.com/tarantool/go-config/collectors"
	"github.com/tarantool/go-config/meta"
	"github.com/tarantool/go-config/tree"
)

func TestMergeCollector_Success(t *testing.T) {
	t.Parallel()

	root := tree.New()
	col := collectors.NewMock().
		WithEntry(config.NewKeyPath("server/port"), 8080).
		WithEntry(config.NewKeyPath("server/host"), "localhost").
		WithName("test")

	err := config.MergeCollector(root, col)
	require.NoError(t, err)

	portNode := root.Get(config.NewKeyPath("server/port"))
	require.NotNil(t, portNode)
	assert.Equal(t, 8080, portNode.Value)

	hostNode := root.Get(config.NewKeyPath("server/host"))
	require.NotNil(t, hostNode)
	assert.Equal(t, "localhost", hostNode.Value)

	assert.Equal(t, "test", portNode.Source)
	assert.Equal(t, "test", hostNode.Source)
}

func TestMergeCollector_MapMerging(t *testing.T) {
	t.Parallel()

	root := tree.New()

	col1 := collectors.NewMock().
		WithEntry(config.NewKeyPath("server"), map[string]any{
			"port": 8080,
			"host": "localhost",
		}).
		WithName("first")

	err := config.MergeCollector(root, col1)
	require.NoError(t, err)

	col2 := collectors.NewMock().
		WithEntry(config.NewKeyPath("server"), map[string]any{
			"port": 9090,
			"ssl":  true,
		}).
		WithName("second")

	err = config.MergeCollector(root, col2)
	require.NoError(t, err)

	serverNode := root.Get(config.NewKeyPath("server"))
	require.NotNil(t, serverNode)
	assert.False(t, serverNode.IsLeaf())
	assert.Len(t, serverNode.Children(), 3)

	portNode := serverNode.Child("port")
	require.NotNil(t, portNode)
	assert.Equal(t, 9090, portNode.Value)
	assert.Equal(t, "second", portNode.Source)

	hostNode := serverNode.Child("host")
	require.NotNil(t, hostNode)
	assert.Equal(t, "localhost", hostNode.Value)
	assert.Equal(t, "first", hostNode.Source)

	sslNode := serverNode.Child("ssl")
	require.NotNil(t, sslNode)
	assert.Equal(t, true, sslNode.Value)
	assert.Equal(t, "second", sslNode.Source)
}

func TestMergeCollector_LeafToMapConversion(t *testing.T) {
	t.Parallel()

	root := tree.New()

	col1 := collectors.NewMock().
		WithEntry(config.NewKeyPath("server"), 8080).
		WithName("first")

	err := config.MergeCollector(root, col1)
	require.NoError(t, err)

	serverNode := root.Get(config.NewKeyPath("server"))
	require.NotNil(t, serverNode)
	assert.True(t, serverNode.IsLeaf())
	assert.Equal(t, 8080, serverNode.Value)

	col2 := collectors.NewMock().
		WithEntry(config.NewKeyPath("server"), map[string]any{
			"port": 9090,
			"host": "localhost",
		}).
		WithName("second")

	err = config.MergeCollector(root, col2)
	require.NoError(t, err)

	serverNode = root.Get(config.NewKeyPath("server"))
	require.NotNil(t, serverNode)
	assert.False(t, serverNode.IsLeaf())
	assert.Nil(t, serverNode.Value)
	assert.Len(t, serverNode.Children(), 2)

	portNode := serverNode.Child("port")
	require.NotNil(t, portNode)
	assert.Equal(t, 9090, portNode.Value)
	assert.Equal(t, "second", portNode.Source)

	hostNode := serverNode.Child("host")
	require.NotNil(t, hostNode)
	assert.Equal(t, "localhost", hostNode.Value)
	assert.Equal(t, "second", hostNode.Source)
}

func TestMergeCollector_SliceReplacement(t *testing.T) {
	t.Parallel()

	root := tree.New()

	col1 := collectors.NewMock().
		WithEntry(config.NewKeyPath("items"), []string{"a", "b"}).
		WithName("first")

	err := config.MergeCollector(root, col1)
	require.NoError(t, err)

	itemsNode := root.Get(config.NewKeyPath("items"))
	require.NotNil(t, itemsNode)
	assert.True(t, itemsNode.IsLeaf())
	assert.Equal(t, []string{"a", "b"}, itemsNode.Value)
	assert.Equal(t, "first", itemsNode.Source)

	col2 := collectors.NewMock().
		WithEntry(config.NewKeyPath("items"), []string{"c", "d", "e"}).
		WithName("second")

	err = config.MergeCollector(root, col2)
	require.NoError(t, err)

	itemsNode = root.Get(config.NewKeyPath("items"))
	require.NotNil(t, itemsNode)
	assert.True(t, itemsNode.IsLeaf())
	assert.Equal(t, []string{"c", "d", "e"}, itemsNode.Value)
	assert.Equal(t, "second", itemsNode.Source)

	col3 := collectors.NewMock().
		WithEntry(config.NewKeyPath("items"), 42).
		WithName("third")

	err = config.MergeCollector(root, col3)
	require.NoError(t, err)

	itemsNode = root.Get(config.NewKeyPath("items"))
	require.NotNil(t, itemsNode)
	assert.True(t, itemsNode.IsLeaf())
	assert.Equal(t, 42, itemsNode.Value)
	assert.Equal(t, "third", itemsNode.Source)
}

func TestMergeCollectorWithMerger_ErrorInMergeValue(t *testing.T) {
	t.Parallel()

	root := tree.New()
	col := collectors.NewMock().
		WithEntry(config.NewKeyPath("key"), "value").
		WithName("test")

	merger := &errorMerger{err: errors.New("merge failed")} //nolint:err113
	err := config.MergeCollectorWithMerger(root, col, merger)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "collector test: merge value at key: merge failed")

	var collectorErr *config.CollectorError
	require.ErrorAs(t, err, &collectorErr)
	assert.Equal(t, "test", collectorErr.CollectorName)
	assert.Equal(t, "merge value at key: merge failed", collectorErr.Unwrap().Error())
}

func TestMergeCollectorWithMerger_ApplyOrderingError(t *testing.T) {
	t.Parallel()

	root := tree.New()
	col := collectors.NewMock().
		WithEntry(config.NewKeyPath("key"), "value").
		WithName("test").
		WithKeepOrder(true)

	merger := &orderingErrorMerger{}
	err := config.MergeCollectorWithMerger(root, col, merger)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "collector test: apply ordering: ordering failed")

	var collectorErr *config.CollectorError
	require.ErrorAs(t, err, &collectorErr)
	assert.Equal(t, "test", collectorErr.CollectorName)
	assert.Equal(t, "apply ordering: ordering failed", collectorErr.Unwrap().Error())
}

func TestMergeCollectorWithMerger_MultipleErrors(t *testing.T) {
	t.Parallel()

	root := tree.New()
	col := &multiErrorCollector{
		name: "multi",
		errors: []error{
			errors.New("first error"),  //nolint:err113
			errors.New("second error"), //nolint:err113
		},
	}

	err := config.MergeCollector(root, col)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "collector multi: failed to get raw value for key key0: first error")
	assert.Contains(t, err.Error(), "failed to get raw value for key key1: second error")

	var collectorErr *config.CollectorError
	require.ErrorAs(t, err, &collectorErr)
	assert.Equal(t, "multi", collectorErr.CollectorName)

	unwrapped := collectorErr.Unwrap()
	require.Error(t, unwrapped)
	assert.Contains(t, unwrapped.Error(), "first error")
	assert.Contains(t, unwrapped.Error(), "second error")
}

type errorMerger struct {
	err error
}

func (e *errorMerger) CreateContext(col config.Collector) config.MergerContext {
	return &errorContext{collector: col}
}

func (e *errorMerger) MergeValue(_ config.MergerContext, _ *tree.Node, _ config.KeyPath, _ any) error {
	return e.err
}

type errorContext struct {
	collector config.Collector
}

func (c *errorContext) Collector() config.Collector               { return c.collector }
func (c *errorContext) RecordOrdering(_ config.KeyPath, _ string) {}
func (c *errorContext) ApplyOrdering(_ *tree.Node) error          { return nil }

type orderingErrorMerger struct{}

func (m *orderingErrorMerger) CreateContext(col config.Collector) config.MergerContext {
	return &orderingErrorContext{collector: col}
}

func (m *orderingErrorMerger) MergeValue(
	ctx config.MergerContext,
	root *tree.Node,
	path config.KeyPath,
	value any,
) error {
	return config.Default.MergeValue(ctx, root, path, value)
}

type orderingErrorContext struct {
	collector config.Collector
}

func (c *orderingErrorContext) Collector() config.Collector               { return c.collector }
func (c *orderingErrorContext) RecordOrdering(_ config.KeyPath, _ string) {}
func (c *orderingErrorContext) ApplyOrdering(_ *tree.Node) error {
	return errors.New("ordering failed") //nolint:err113
}

type multiErrorCollector struct {
	name   string
	errors []error
}

func (c *multiErrorCollector) Read(_ context.Context) <-chan config.Value {
	valueCh := make(chan config.Value, len(c.errors))

	go func() {
		defer close(valueCh)

		for i, err := range c.errors {
			valueCh <- &multiErrorValue{
				err: err,
				key: config.NewKeyPath("key" + string(rune('0'+i))),
			}
		}
	}()

	return valueCh
}

func (c *multiErrorCollector) Name() string                  { return c.name }
func (c *multiErrorCollector) Source() config.SourceType     { return config.UnknownSource }
func (c *multiErrorCollector) Revision() config.RevisionType { return "" }
func (c *multiErrorCollector) KeepOrder() bool               { return false }

type multiErrorValue struct {
	err error
	key config.KeyPath
}

func (v *multiErrorValue) Get(_ any) error {
	return v.err
}

func (v *multiErrorValue) Meta() meta.Info {
	return meta.Info{
		Key: v.key,
		Source: meta.SourceInfo{
			Name: "multi-error-collector",
			Type: config.UnknownSource,
		},
		Revision: "",
	}
}
