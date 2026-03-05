package collectors_test

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tarantool/go-config"
	"github.com/tarantool/go-config/collectors"
	"github.com/tarantool/go-config/internal/testutil"
)

func TestNewDirectory(t *testing.T) {
	t.Parallel()

	collector := collectors.NewDirectory("testdata/configdir", ".yaml", collectors.NewYamlFormat())

	require.NotNil(t, collector)
	assert.Equal(t, "directory", collector.Name())
	assert.Equal(t, config.FileSource, collector.Source())
	assert.Equal(t, config.RevisionType(""), collector.Revision())
	assert.False(t, collector.KeepOrder())
}

func TestDirectory_WithName(t *testing.T) {
	t.Parallel()

	collector := collectors.NewDirectory("testdata/configdir", ".yaml", collectors.NewYamlFormat()).
		WithName("config")
	assert.Equal(t, "config", collector.Name())
}

func TestDirectory_WithSourceType(t *testing.T) {
	t.Parallel()

	collector := collectors.NewDirectory("testdata/configdir", ".yaml", collectors.NewYamlFormat()).
		WithSourceType(config.StorageSource)
	assert.Equal(t, config.StorageSource, collector.Source())
}

func TestDirectory_WithRevision(t *testing.T) {
	t.Parallel()

	collector := collectors.NewDirectory("testdata/configdir", ".yaml", collectors.NewYamlFormat()).
		WithRevision("v1")
	assert.Equal(t, config.RevisionType("v1"), collector.Revision())
}

func TestDirectory_WithKeepOrder(t *testing.T) {
	t.Parallel()

	collector := collectors.NewDirectory("testdata/configdir", ".yaml", collectors.NewYamlFormat()).
		WithKeepOrder(true)
	assert.True(t, collector.KeepOrder())
}

func TestDirectory_WithRecursive(t *testing.T) {
	t.Parallel()

	collector := collectors.NewDirectory("testdata/configdir", ".yaml", collectors.NewYamlFormat()).
		WithRecursive(true)
	assert.True(t, collector.Recursive())
}

func TestDirectory_Read_SingleFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTestFile(t, dir, "app.yaml", "port: 8080\nhost: localhost")

	collector := collectors.NewDirectory(dir, ".yaml", collectors.NewYamlFormat())

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
	}

	assert.Equal(t, int64(8080), valuesMap["port"])
	assert.Equal(t, "localhost", valuesMap["host"])
}

func TestDirectory_Read_SourceName(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTestFile(t, dir, "app.yaml", "port: 8080")

	collector := collectors.NewDirectory(dir, ".yaml", collectors.NewYamlFormat()).
		WithName("config")

	ctx := context.Background()
	channel := collector.Read(ctx)

	val := <-channel
	assert.Equal(t, "config:"+dir+"/app.yaml", val.Meta().Source.Name)
}

func TestDirectory_Read_MultipleFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTestFile(t, dir, "app.yaml", "port: 8080")
	writeTestFile(t, dir, "db.yaml", "dbhost: postgres")

	collector := collectors.NewDirectory(dir, ".yaml", collectors.NewYamlFormat())

	ctx := context.Background()
	channel := collector.Read(ctx)

	valuesMap := make(map[string]any)

	for val := range channel {
		var dest any

		err := val.Get(&dest)
		require.NoError(t, err)

		valuesMap[val.Meta().Key.String()] = dest
	}

	assert.Len(t, valuesMap, 2)
	assert.Equal(t, int64(8080), valuesMap["port"])
	assert.Equal(t, "postgres", valuesMap["dbhost"])
}

func TestDirectory_Read_FiltersExtension(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTestFile(t, dir, "app.yaml", "port: 8080")
	writeTestFile(t, dir, "ignored.txt", "should: be-ignored")

	collector := collectors.NewDirectory(dir, ".yaml", collectors.NewYamlFormat())

	ctx := context.Background()
	channel := collector.Read(ctx)

	values := make([]config.Value, 0, 1)

	for val := range channel {
		values = append(values, val)
	}

	assert.Len(t, values, 1)
}

func TestDirectory_Collectors_ParseError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTestFile(t, dir, "invalid.yaml", "bad: yaml: [unclosed")

	collector := collectors.NewDirectory(dir, ".yaml", collectors.NewYamlFormat())

	ctx := context.Background()
	_, err := collector.Collectors(ctx)

	require.Error(t, err)
	assert.ErrorIs(t, err, collectors.ErrFormatParse)
}

func TestDirectory_Collectors_SkipsEmptyFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTestFile(t, dir, "empty.yaml", "")
	writeTestFile(t, dir, "valid.yaml", "key: value")

	collector := collectors.NewDirectory(dir, ".yaml", collectors.NewYamlFormat())

	ctx := context.Background()
	subs, err := collector.Collectors(ctx)

	require.NoError(t, err)
	assert.Len(t, subs, 1)
}

func TestDirectory_Read_EmptyDirectory(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	collector := collectors.NewDirectory(dir, ".yaml", collectors.NewYamlFormat())

	ctx := context.Background()
	channel := collector.Read(ctx)

	var count int

	for range channel {
		count++
	}

	assert.Equal(t, 0, count)
}

func TestDirectory_Read_NonexistentDirectory(t *testing.T) {
	t.Parallel()

	collector := collectors.NewDirectory("/nonexistent/path", ".yaml", collectors.NewYamlFormat())

	ctx := context.Background()
	channel := collector.Read(ctx)

	var count int

	for range channel {
		count++
	}

	assert.Equal(t, 0, count)
}

func TestDirectory_Read_NestedYaml(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTestFile(t, dir, "nested.yaml", "a:\n  b:\n    c: deep\n    d:\n      - 1\n      - 2")

	collector := collectors.NewDirectory(dir, ".yaml", collectors.NewYamlFormat())

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

func TestDirectory_Read_Cancellation(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTestFile(t, dir, "a.yaml", "x: 1")
	writeTestFile(t, dir, "b.yaml", "x: 2")

	collector := collectors.NewDirectory(dir, ".yaml", collectors.NewYamlFormat())

	ctx, cancel := context.WithCancel(context.Background())
	channel := collector.Read(ctx)

	_, ok := <-channel
	require.True(t, ok)

	cancel()
	testutil.Drain(t, channel)
}

func TestDirectory_Collectors_ReturnsSubCollectors(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTestFile(t, dir, "app.yaml", "port: 8080")
	writeTestFile(t, dir, "db.yaml", "dbhost: postgres")

	collector := collectors.NewDirectory(dir, ".yaml", collectors.NewYamlFormat())

	ctx := context.Background()
	subs, err := collector.Collectors(ctx)

	require.NoError(t, err)
	assert.Len(t, subs, 2)

	for _, sub := range subs {
		assert.Equal(t, config.FileSource, sub.Source())
	}
}

func TestDirectory_Collectors_NonexistentDirectory(t *testing.T) {
	t.Parallel()

	collector := collectors.NewDirectory("/nonexistent/path", ".yaml", collectors.NewYamlFormat())

	ctx := context.Background()
	_, err := collector.Collectors(ctx)

	require.Error(t, err)
	assert.ErrorIs(t, err, collectors.ErrDirectoryRead)
}

func TestDirectory_Builder_Integration(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTestFile(t, dir, "app.yaml", "app:\n  port: 8080")

	dirCollector := collectors.NewDirectory(dir, ".yaml", collectors.NewYamlFormat()).
		WithName("config")

	builder := config.NewBuilder()

	builder = builder.AddCollector(dirCollector)

	cfg, errs := builder.Build()
	assert.Empty(t, errs)

	var port int

	meta, err := cfg.Get(config.NewKeyPath("app/port"), &port)
	require.NoError(t, err)
	assert.Equal(t, 8080, port)
	assert.Equal(t, "config:"+dir+"/app.yaml", meta.Source.Name)
}

func TestDirectory_Builder_WithOtherCollectors(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTestFile(t, dir, "doc.yaml", "setting: dir-value")

	dirCollector := collectors.NewDirectory(dir, ".yaml", collectors.NewYamlFormat())

	mapCollector := collectors.NewMap(map[string]any{
		"setting": "map-value",
	})

	builder := config.NewBuilder()

	builder = builder.AddCollector(mapCollector)
	builder = builder.AddCollector(dirCollector)

	cfg, errs := builder.Build()
	assert.Empty(t, errs)

	var val string

	_, err := cfg.Get(config.NewKeyPath("setting"), &val)
	require.NoError(t, err)
	assert.Equal(t, "dir-value", val)
}

func TestDirectory_Read_SkipsSubdirectories(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTestFile(t, dir, "app.yaml", "port: 8080")

	subdir := dir + "/subdir.yaml"
	require.NoError(t, os.MkdirAll(subdir, 0o750))

	collector := collectors.NewDirectory(dir, ".yaml", collectors.NewYamlFormat())

	ctx := context.Background()
	channel := collector.Read(ctx)

	values := make([]config.Value, 0, 1)

	for val := range channel {
		values = append(values, val)
	}

	assert.Len(t, values, 1)
}

func TestDirectory_Read_Recursive(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTestFile(t, dir, "app.yaml", "port: 8080")

	subdir := dir + "/subdir"
	require.NoError(t, os.MkdirAll(subdir, 0o750))
	writeTestFile(t, subdir, "db.yaml", "dbhost: postgres")

	collector := collectors.NewDirectory(dir, ".yaml", collectors.NewYamlFormat()).
		WithRecursive(true)

	ctx := context.Background()
	channel := collector.Read(ctx)

	valuesMap := make(map[string]any)

	for val := range channel {
		var dest any

		err := val.Get(&dest)
		require.NoError(t, err)

		valuesMap[val.Meta().Key.String()] = dest
	}

	assert.Len(t, valuesMap, 2)
	assert.Equal(t, int64(8080), valuesMap["port"])
	assert.Equal(t, "postgres", valuesMap["dbhost"])
}

func TestDirectory_Collectors_Recursive(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTestFile(t, dir, "app.yaml", "port: 8080")

	subdir := dir + "/subdir"
	require.NoError(t, os.MkdirAll(subdir, 0o750))
	writeTestFile(t, subdir, "db.yaml", "dbhost: postgres")

	collector := collectors.NewDirectory(dir, ".yaml", collectors.NewYamlFormat()).
		WithRecursive(true)

	ctx := context.Background()
	subs, err := collector.Collectors(ctx)

	require.NoError(t, err)
	assert.Len(t, subs, 2)

	names := make([]string, 0, 2)
	for _, sub := range subs {
		names = append(names, sub.Name())
	}

	assert.Contains(t, names[0], "app.yaml")
	assert.Contains(t, names[1], "subdir/db.yaml")
}

func TestDirectory_Read_FollowsFileSymlink(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTestFile(t, dir, "app.yaml", "port: 8080")

	linkPath := dir + "/linked.yaml"
	require.NoError(t, os.Symlink(dir+"/app.yaml", linkPath))

	collector := collectors.NewDirectory(dir, ".yaml", collectors.NewYamlFormat())

	ctx := context.Background()
	channel := collector.Read(ctx)

	valuesMap := make(map[string]any)

	for val := range channel {
		var dest any

		err := val.Get(&dest)
		require.NoError(t, err)

		valuesMap[val.Meta().Key.String()] = dest
	}

	assert.Equal(t, int64(8080), valuesMap["port"])
}

func TestDirectory_Read_SkipsDirectorySymlink(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTestFile(t, dir, "app.yaml", "port: 8080")

	otherDir := t.TempDir()
	writeTestFile(t, otherDir, "db.yaml", "dbhost: postgres")

	require.NoError(t, os.Symlink(otherDir, dir+"/linked"))

	collector := collectors.NewDirectory(dir, ".yaml", collectors.NewYamlFormat())

	ctx := context.Background()
	channel := collector.Read(ctx)

	values := make([]config.Value, 0, 1)

	for val := range channel {
		values = append(values, val)
	}

	assert.Len(t, values, 1)
}

// writeTestFile is a helper to create a file in a test directory.
func writeTestFile(t *testing.T, dir, name, content string) {
	t.Helper()

	err := os.WriteFile(dir+"/"+name, []byte(content), 0o600)
	require.NoError(t, err)
}
