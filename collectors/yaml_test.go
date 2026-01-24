package collectors_test

import (
	"bytes"
	_ "embed"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tarantool/go-config"
	"github.com/tarantool/go-config/collectors"
)

//go:embed testdata/config.yaml
var configYaml string

func TestNewYamlFormat(t *testing.T) {
	t.Parallel()

	format := collectors.NewYamlFormat()
	require.NotNil(t, format)

	assert.Equal(t, "yaml", format.Name())
	assert.True(t, format.KeepOrder())
}

func TestYaml_From(t *testing.T) {
	t.Parallel()

	reader := strings.NewReader(configYaml)
	require.NotNil(t, reader)

	format := collectors.NewYamlFormat().From(reader)
	require.NotNil(t, format)
}

func TestYaml_Parse(t *testing.T) {
	t.Parallel()

	reader := strings.NewReader(configYaml)

	format := collectors.NewYamlFormat().From(reader)
	require.NotNil(t, format)

	root, err := format.Parse()
	require.NotNil(t, root)
	require.NoError(t, err)

	node := root.Get(config.NewKeyPath("storage/provider"))
	require.NotNil(t, node)

	val, ok := node.Value.(string)
	require.True(t, ok)
	assert.Equal(t, "etcd", val)

	node = root.Get(config.NewKeyPath("initial-settings/clusters/0/name"))
	require.NotNil(t, node)

	val, ok = node.Value.(string)
	require.True(t, ok)
	assert.Equal(t, "default-cluster", val)
}

func TestYaml_Parse_Invalid(t *testing.T) {
	t.Parallel()

	format := collectors.NewYamlFormat().From(nil)
	require.NotNil(t, format)

	root, err := format.Parse()
	require.Nil(t, root)
	require.Error(t, err)
	assert.Equal(t, err, collectors.ErrNoData)

	format = collectors.NewYamlFormat().From(nil)
	require.NotNil(t, format)

	root, err = format.Parse()
	require.Nil(t, root)
	require.Error(t, err)
	assert.Equal(t, err, collectors.ErrNoData)

	data := []byte("special: character: value")

	format = collectors.NewYamlFormat().From(bytes.NewReader(data))
	require.NotNil(t, format)

	root, err = format.Parse()
	require.Nil(t, root)
	require.Error(t, err)
}
