package collectors_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tarantool/go-config"
	"github.com/tarantool/go-config/collectors"
)

func TestNewFile(t *testing.T) {
	t.Parallel()

	file := collectors.NewFile("config.yaml")
	require.NotNil(t, file)
}

func TestNewFile_Name(t *testing.T) {
	t.Parallel()

	file := collectors.NewFile("config.yaml")
	assert.NotNil(t, file)

	assert.Equal(t, "file", file.Name())
}

func TestNewFile_SourceType(t *testing.T) {
	t.Parallel()

	file := collectors.NewFile("config.yaml")
	assert.NotNil(t, file)

	require.Equal(t, config.FileSource, file.SourceType())
}

func TestNewFile_Revision(t *testing.T) {
	t.Parallel()

	file := collectors.NewFile("config.yaml")
	require.NotNil(t, file)

	assert.Empty(t, file.Revision())
}

func TestNewFile_FetchStream(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	file := collectors.NewFile("testdata/config.yaml")
	require.NotNil(t, file)

	assert.Equal(t, "file", file.Name())
	assert.Equal(t, config.FileSource, file.SourceType())
	assert.Empty(t, file.Revision())

	reader, err := file.FetchStream(ctx)
	require.NotNil(t, reader)
	require.NoError(t, err)
}

func TestNewFile_FetchStream_Error(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	file := collectors.NewFile("testdata/invalid.yaml")
	require.NotNil(t, file)

	assert.Equal(t, "file", file.Name())
	assert.Equal(t, config.FileSource, file.SourceType())
	assert.Empty(t, file.Revision())

	reader, err := file.FetchStream(ctx)
	if err == nil {
		defer reader.Close() //nolint:errcheck
	}

	require.Nil(t, reader)
	require.Error(t, err)
}

func TestNewSource_Creation(t *testing.T) {
	t.Parallel()

	source, err := collectors.NewSource(collectors.NewFile("testdata/config.yaml"), collectors.NewYamlFormat())
	require.NotNil(t, source)
	require.NoError(t, err)
}

func TestNewSource_Name(t *testing.T) {
	t.Parallel()

	source, err := collectors.NewSource(collectors.NewFile("testdata/config.yaml"), collectors.NewYamlFormat())
	require.NotNil(t, source)
	require.NoError(t, err)

	assert.Equal(t, "file", source.Name())
}

func TestNewSource_Source(t *testing.T) {
	t.Parallel()

	source, err := collectors.NewSource(collectors.NewFile("testdata/config.yaml"), collectors.NewYamlFormat())
	require.NotNil(t, source)
	require.NoError(t, err)

	assert.Equal(t, config.FileSource, source.Source())
}

func TestNewSource_Revision(t *testing.T) {
	t.Parallel()

	source, err := collectors.NewSource(collectors.NewFile("testdata/config.yaml"), collectors.NewYamlFormat())
	require.NotNil(t, source)
	require.NoError(t, err)

	assert.Empty(t, source.Revision())
}

func TestNewSource_KeepOrder(t *testing.T) {
	t.Parallel()

	source, err := collectors.NewSource(collectors.NewFile("testdata/config.yaml"), collectors.NewYamlFormat())
	require.NotNil(t, source)
	require.NoError(t, err)

	assert.True(t, source.KeepOrder())
}

func TestNewSource_Read(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	file := collectors.NewFile("testdata/config.yaml")
	require.NotNil(t, file)

	format := collectors.NewYamlFormat()
	require.NotNil(t, format)

	source, err := collectors.NewSource(file, format)
	require.NotNil(t, source)
	require.NoError(t, err)

	ch := source.Read(ctx)

	values := make([]config.Value, 0, 20)
	for val := range ch {
		values = append(values, val)
	}

	assert.Len(t, values, 20)

	var dest any

	require.Equal(t, "credentials/users/client/roles/2", values[3].Meta().Key.String())

	err = values[3].Get(&dest)
	require.NoError(t, err)
	assert.Equal(t, "paratrooper", dest)

	assert.Equal(t, "initial-settings/clusters/0/storage-connection/etcd-connection/endpoints/0",
		values[19].Meta().Key.String())

	err = values[19].Get(&dest)
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:2379", dest)
}
