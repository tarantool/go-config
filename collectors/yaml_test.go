package collectors_test

import (
	"context"
	_ "embed"
	"os"
	"strings"
	"testing"

	"github.com/shoenig/test"
	"github.com/shoenig/test/must"

	"github.com/tarantool/go-config"
	"github.com/tarantool/go-config/collectors"
)

//go:embed testdata/config.yaml
var configYaml string

func TestNewYamlBuilder(t *testing.T) {
	t.Parallel()

	fileCollector, err := collectors.NewYamlCollectorBuilder(os.Stdin).Build()
	must.NotNil(t, fileCollector)
	must.Nil(t, err)

	test.Eq(t, "yaml", fileCollector.Name())
	test.Eq(t, config.UnknownSource, fileCollector.Source())
	test.Eq(t, "", fileCollector.Revision())
	test.False(t, fileCollector.KeepOrder())
}

func TestYamlBuilder_SetName(t *testing.T) {
	t.Parallel()

	fc, err := collectors.NewYamlCollectorBuilder(os.Stdin).SetName("custom").Build()
	must.NotNil(t, fc)
	must.Nil(t, err)

	test.Eq(t, "custom", fc.Name())
}

func TestYamlBuilder_SetSourceType(t *testing.T) {
	t.Parallel()

	fc, err := collectors.NewYamlCollectorBuilder(os.Stdin).SetSourceType(config.FileSource).Build()
	must.NotNil(t, fc)
	must.Nil(t, err)

	test.Eq(t, config.FileSource, fc.Source())
}

func TestYamlBuilder_SetRevision(t *testing.T) {
	t.Parallel()

	fc, err := collectors.NewYamlCollectorBuilder(os.Stdin).SetRevision("v1.0.0").Build()
	must.NotNil(t, fc)
	must.Nil(t, err)

	test.Eq(t, "v1.0.0", fc.Revision())
}

func TestYamlBuilder_SetKeepOrder(t *testing.T) {
	t.Parallel()

	fc, err := collectors.NewYamlCollectorBuilder(os.Stdin).SetKeepOrder(true).Build()
	must.NotNil(t, fc)
	must.Nil(t, err)

	test.True(t, fc.KeepOrder())
}

func TestYaml_Read_Basic(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	reader := strings.NewReader(configYaml)

	fc, err := collectors.NewYamlCollectorBuilder(reader).Build()
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
