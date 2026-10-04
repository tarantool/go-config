package tarantool_test

import (
	"bytes"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	config "github.com/tarantool/go-config/v2"
	"github.com/tarantool/go-config/v2/collectors"
	"github.com/tarantool/go-config/v2/internal/testutil"
	"github.com/tarantool/go-config/v2/tarantool"
)

const templateDirectorySource = "directory"
const templateFileSource = "file"
const templateStorageSource = "storage"

const templateConfig = `console:
  socket: 'var/run/{{ instance_name }}/tarantool.control' # socket template
log:
  file: '{{group_name}}/{{ replicaset_name }}/{{instance_name}}.log'
app:
  cfg:
    '{{instance_name}}': '{{group_name}}'
    nested:
      - '{{replicaset_name}}'
      - name: '{{ instance_name }}'
groups:
  storages:
    replicasets:
      rs-001:
        instances:
          storage-001: {}
          storage-002: {}
`

func TestBuild_TarantoolTemplates(t *testing.T) {
	t.Parallel()

	for _, source := range []string{templateFileSource, templateDirectorySource, templateStorageSource} {
		t.Run(source, func(t *testing.T) {
			t.Parallel()

			builder := tarantool.New().WithEnvPrefix("GO_CONFIG_TEMPLATE_TEST_").WithoutSchema()

			switch source {
			case templateFileSource, templateDirectorySource:
				dir := t.TempDir()
				filename := filepath.Join(dir, "config.yaml")
				writeFile(t, filename, templateConfig)

				if source == templateFileSource {
					builder.WithConfigFile(filename)
				} else {
					builder.WithConfigDir(dir)
				}
			case templateStorageSource:
				mock := testutil.NewMockStorage()
				testutil.PutIntegrity(mock, "/config/", "app", []byte(templateConfig))
				builder.WithStorage(testutil.NewRawTyped(mock, "/config/"))
			}

			cfg, err := builder.Build(t.Context())
			require.NoError(t, err)

			rawYAML, err := cfg.MarshalYAML()
			require.NoError(t, err)

			for _, instance := range []string{"storage-001", "storage-002"} {
				t.Run(instance, func(t *testing.T) {
					t.Parallel()

					path := config.NewKeyPath("groups/storages/replicasets/rs-001/instances/" + instance)
					effective, err := cfg.Effective(path)
					require.NoError(t, err)

					for key, want := range map[string]string{
						"console/socket":        "var/run/" + instance + "/tarantool.control",
						"log/file":              "storages/rs-001/" + instance + ".log",
						"app/cfg/" + instance:   "storages",
						"app/cfg/nested/0":      "rs-001",
						"app/cfg/nested/1/name": instance,
					} {
						var value string

						_, err = effective.Get(config.NewKeyPath(key), &value)
						require.NoError(t, err)
						assert.Equal(t, want, value, key)
					}

					rawMeta, ok := cfg.Stat(config.NewKeyPath("console/socket"))
					require.True(t, ok)

					effectiveMeta, ok := effective.Stat(config.NewKeyPath("console/socket"))
					require.True(t, ok)
					assert.Equal(t, rawMeta, effectiveMeta)

					encoded, err := effective.MarshalYAML()
					require.NoError(t, err)
					assert.NotContains(t, string(encoded), "{{")
					assert.Contains(t, string(encoded), "# socket template")

					root, err := collectors.NewYamlFormat(collectors.WithTarantoolParserFormatting()).
						From(bytes.NewReader(encoded)).Parse()
					require.NoError(t, err)
					assert.Equal(t, "storages", root.Get(config.NewKeyPath("app/cfg/"+instance)).Value)

					all, err := cfg.EffectiveAll()
					require.NoError(t, err)
					require.Len(t, all, 2)

					selected := all[path.String()]

					var socket string

					_, err = selected.Get(config.NewKeyPath("console/socket"), &socket)
					require.NoError(t, err)
					assert.Equal(t, "var/run/"+instance+"/tarantool.control", socket)

					after, err := cfg.MarshalYAML()
					require.NoError(t, err)
					assert.True(t, bytes.Equal(rawYAML, after), "effective views preserve raw YAML styles and comments")
				})
			}
		})
	}
}

func TestBuild_TarantoolTemplates_UnknownVariablePreserved(t *testing.T) {
	t.Parallel()

	for _, input := range []string{
		"{{unknown}}", "{{ }}", "{{}}", "{{\tinstance_name\t}}", "{{\ninstance_name\n}}", "{{{instance_name}}}",
	} {
		t.Run(input, func(t *testing.T) {
			t.Parallel()

			cfgPath := filepath.Join(t.TempDir(), "config.yaml")
			writeFile(t, cfgPath, fmt.Sprintf("console:\n  socket: %q\n", input)+
				"groups:\n  g:\n    replicasets:\n      r:\n        instances:\n          i: {}\n")

			cfg, err := tarantool.New().WithConfigFile(cfgPath).
				WithEnvPrefix("GO_CONFIG_TEMPLATE_TEST_").WithoutSchema().Build(t.Context())
			require.NoError(t, err)

			path := config.NewKeyPath("groups/g/replicasets/r/instances/i")

			effective, err := cfg.Effective(path)
			require.NoError(t, err)

			var value string

			_, err = effective.Get(config.NewKeyPath("console/socket"), &value)
			require.NoError(t, err)
			assert.Equal(t, input, value)

			all, err := cfg.EffectiveAll()
			require.NoError(t, err)

			instance := all[path.String()]

			_, err = instance.Get(config.NewKeyPath("console/socket"), &value)
			require.NoError(t, err)
			assert.Equal(t, input, value)

			var raw string

			_, err = cfg.Get(config.NewKeyPath("console/socket"), &raw)
			require.NoError(t, err)
			assert.Equal(t, input, raw)
		})
	}
}

func TestBuild_TarantoolTemplates_AfterInheritanceAndMutation(t *testing.T) {
	t.Parallel()

	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	writeFile(t, cfgPath, "console:\n  socket: '{{unknown}}'\n"+
		"groups:\n  g:\n    replicasets:\n      r:\n        instances:\n          i:\n"+
		"            console:\n              socket: '{{group_name}}/{{instance_name}}'\n")

	cfg, err := tarantool.New().WithConfigFile(cfgPath).
		WithEnvPrefix("GO_CONFIG_TEMPLATE_TEST_").WithoutSchema().BuildMutable(t.Context())
	require.NoError(t, err)

	path := config.NewKeyPath("groups/g/replicasets/r/instances/i")
	effective, err := cfg.Effective(path)
	require.NoError(t, err, "the overridden unknown variable is never expanded")

	var value string

	_, err = effective.Get(config.NewKeyPath("console/socket"), &value)
	require.NoError(t, err)
	assert.Equal(t, "g/i", value)

	require.NoError(t, cfg.Set(path.Append("console", "socket"), "new-{{instance_name}}"))

	snapshot := cfg.Snapshot()

	effective, err = snapshot.Effective(path)
	require.NoError(t, err)

	_, err = effective.Get(config.NewKeyPath("console/socket"), &value)
	require.NoError(t, err)
	assert.Equal(t, "new-i", value)

	require.True(t, cfg.Delete(path))

	all, err := cfg.EffectiveAll()
	require.NoError(t, err)
	assert.Empty(t, all)
}

func TestBuild_TarantoolTemplates_EnvPriority(t *testing.T) {
	t.Setenv("GO_CONFIG_TEMPLATE_ENV_CONSOLE_SOCKET", "env-{{ instance_name }}")

	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	writeFile(t, cfgPath, templateConfig)

	cfg, err := tarantool.New().WithConfigFile(cfgPath).
		WithEnvPrefix("GO_CONFIG_TEMPLATE_ENV_").WithoutSchema().Build(t.Context())
	require.NoError(t, err)

	effective, err := cfg.Effective(config.NewKeyPath("groups/storages/replicasets/rs-001/instances/storage-001"))
	require.NoError(t, err)

	var value string

	_, err = effective.Get(config.NewKeyPath("console/socket"), &value)
	require.NoError(t, err)
	assert.Equal(t, "env-storage-001", value)
}

func TestBuild_TarantoolTemplates_WithSchema(t *testing.T) {
	t.Parallel()

	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	writeFile(t, cfgPath, templateConfig)

	cfg, err := tarantool.New().WithConfigFile(cfgPath).
		WithSchemaVersion("3.8.0").WithEnvPrefix("GO_CONFIG_TEMPLATE_TEST_").Build(t.Context())
	require.NoError(t, err, "templates remain strings during schema validation")

	effective, err := cfg.Effective(config.NewKeyPath("groups/storages/replicasets/rs-001/instances/storage-001"))
	require.NoError(t, err)

	var socket string

	_, err = effective.Get(config.NewKeyPath("console/socket"), &socket)
	require.NoError(t, err)
	assert.Equal(t, "var/run/storage-001/tarantool.control", socket)
}
