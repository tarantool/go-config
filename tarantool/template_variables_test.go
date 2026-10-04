package tarantool_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	config "github.com/tarantool/go-config/v2"
	"github.com/tarantool/go-config/v2/tarantool"
)

func TestTemplateVariables(t *testing.T) {
	t.Parallel()

	path := config.NewKeyPath("groups/g/replicasets/r/instances/i")
	expected := map[string]string{"group_name": "g", "replicaset_name": "r", "instance_name": "i"}
	first := tarantool.TemplateVariables(path)
	require.Equal(t, expected, first)

	first["instance_name"] = "changed"

	assert.Equal(t, expected, tarantool.TemplateVariables(path), "each call returns an independent map")

	for _, invalid := range []config.KeyPath{
		nil, config.NewKeyPath("instances/i"), config.NewKeyPath("groups/g/replicasets/r"),
		config.NewKeyPath("groups/g/replicasets/r/instances/i/extra"),
		config.NewKeyPath("other/g/replicasets/r/instances/i"),
		config.NewKeyPath("groups/g/other/r/instances/i"),
		config.NewKeyPath("groups/g/replicasets/r/other/i"),
	} {
		assert.Nil(t, tarantool.TemplateVariables(invalid))
	}
}

func TestBuild_TarantoolTemplates_ContextRemainsData(t *testing.T) {
	t.Setenv("GO_CONFIG_TEMPLATE_SECRET", "local-secret")

	content := `config:
  context:
    literal: host
    secret: {from: env, env: GO_CONFIG_TEMPLATE_SECRET}
    file: {from: file, file: /missing/file, rstrip: true}
    invalid: {from: other}
process:
  title: '{{ instance_name }}/{{ context.literal }}/{{  context.secret  }}/{{context.file}}'
groups: {g: {replicasets: {r: {instances: {i: {}}}}}}
`
	file := t.TempDir() + "/config.yaml"
	writeFile(t, file, content)

	cfg, err := tarantool.New().WithConfigFile(file).
		WithEnvPrefix("GO_CONFIG_TEMPLATE_IGNORE_").WithoutSchema().Build(t.Context())
	require.NoError(t, err)

	effective, err := cfg.Effective(config.NewKeyPath("groups/g/replicasets/r/instances/i"))
	require.NoError(t, err)

	var value string

	_, err = effective.Get(config.NewKeyPath("process/title"), &value)
	require.NoError(t, err)
	assert.Equal(t, "i/{{ context.literal }}/{{  context.secret  }}/{{context.file}}", value)

	var rawContext, effectiveContext map[string]any

	_, err = cfg.Get(config.NewKeyPath("config/context"), &rawContext)
	require.NoError(t, err)

	_, err = effective.Get(config.NewKeyPath("config/context"), &effectiveContext)
	require.NoError(t, err)
	assert.Equal(t, rawContext, effectiveContext)
}
