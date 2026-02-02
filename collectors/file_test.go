package collectors_test

import (
	"context"
	"testing"

	"github.com/shoenig/test"
	"github.com/shoenig/test/must"
	"github.com/tarantool/go-config"
	"github.com/tarantool/go-config/collectors"
)

const configFile = "testdata/config.yaml"

func TestNewFileBuilder(t *testing.T) {
	t.Parallel()

	fileCollector, err := collectors.NewFileCollectorBuilder(configFile).Build()
	must.NotNil(t, fileCollector)
	must.Nil(t, err)

	test.Eq(t, "file", fileCollector.Name())
	test.Eq(t, config.FileSource, fileCollector.Source())
	test.Eq(t, "", fileCollector.Revision())
	test.True(t, fileCollector.KeepOrder())
}

func TestFile_Unexist(t *testing.T) {
	t.Parallel()

	fc, err := collectors.NewFileCollectorBuilder("unexist.file").Build()
	must.Nil(t, fc)
	must.NotNil(t, err)
}

func TestFile_SetName(t *testing.T) {
	t.Parallel()

	fc, err := collectors.NewFileCollectorBuilder(configFile).SetName("custom").Build()
	must.Nil(t, err)
	test.Eq(t, "custom", fc.Name())
}

func TestFile_SetSourceType(t *testing.T) {
	t.Parallel()

	fc, err := collectors.NewFileCollectorBuilder(configFile).SetSourceType(config.UnknownSource).Build()
	must.Nil(t, err)
	test.Eq(t, config.UnknownSource, fc.Source())
}

func TestFile_SetRevision(t *testing.T) {
	t.Parallel()

	fc, err := collectors.NewFileCollectorBuilder(configFile).SetRevision("v1.0.0").Build()
	must.Nil(t, err)
	test.Eq(t, "v1.0.0", fc.Revision())
}

func TestFile_SetKeepOrder(t *testing.T) {
	t.Parallel()

	fc, err := collectors.NewFileCollectorBuilder(configFile).SetKeepOrder(false).Build()
	must.Nil(t, err)
	test.False(t, fc.KeepOrder())
}

func TestFile_Read_Basic(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	fc, err := collectors.NewFileCollectorBuilder(configFile).Build()
	must.NotNil(t, fc)
	must.Nil(t, err)

	ch := fc.Read(ctx)

	values := make([]config.Value, 0, 32)
	for val := range ch {
		values = append(values, val)
	}

	// Verify values can be extracted.
	var length int

	for _, val := range values {
		var dest any

		err := val.Get(&dest)
		must.NoError(t, err)

		length++
	}

	must.Len(t, length, values)

	var dest any

	must.Eq(t, values[3].Meta().Key.String(), "credentials/users/client/roles/2")

	err = values[3].Get(&dest)
	must.NoError(t, err)
	must.Eq(t, dest, "paratrooper")

	must.Eq(t, values[19].Meta().Key.String(),
		"initial-settings/clusters/0/storage-connection/etcd-connection/endpoints/0")

	err = values[19].Get(&dest)
	must.NoError(t, err)
	must.Eq(t, dest, "http://localhost:2379")
}
