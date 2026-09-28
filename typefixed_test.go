package config_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tarantool/go-config/v2"
	"github.com/tarantool/go-config/v2/collectors"
	"github.com/tarantool/go-config/v2/tree"
)

const typeFixedSchema = `{
	"$schema": "https://json-schema.org/draft/2020-12/schema",
	"type": "object",
	"properties": {
		"enabled": { "type": "boolean" },
		"port": { "type": "integer" }
	}
}`

func yamlCollector(t *testing.T, doc string) config.Collector {
	t.Helper()

	col, err := collectors.NewSource(t.Context(), yamlString(doc), collectors.NewYamlFormat())
	require.NoError(t, err)

	return col
}

func buildConfig(t *testing.T, cols ...config.Collector) config.Config {
	t.Helper()

	builder := config.NewBuilder()
	for _, col := range cols {
		builder = builder.AddCollector(col)
	}

	cfg, errs := builder.Build(t.Context())
	require.Empty(t, errs)

	return cfg
}

func TestConfig_Get_QuotedYAMLStringIsNotParsed(t *testing.T) {
	t.Parallel()

	cfg := buildConfig(t, yamlCollector(t, `plain: false
double: "false"
single: 'true'
port: "3301"
timeout: "5s"
`))

	var plain bool

	_, err := cfg.Get(config.NewKeyPath("plain"), &plain)
	require.NoError(t, err)
	assert.False(t, plain)

	for _, key := range []string{"double", "single"} {
		var got bool

		_, err = cfg.Get(config.NewKeyPath(key), &got)
		require.ErrorIs(t, err, tree.ErrFixedTypeString, key)
		require.ErrorIs(t, err, tree.ErrConvertToBool, key)
	}

	var port int

	_, err = cfg.Get(config.NewKeyPath("port"), &port)
	require.ErrorIs(t, err, tree.ErrFixedTypeString)

	var double string

	_, err = cfg.Get(config.NewKeyPath("double"), &double)
	require.NoError(t, err)
	assert.Equal(t, "false", double)

	var timeout time.Duration

	_, err = cfg.Get(config.NewKeyPath("timeout"), &timeout)
	require.NoError(t, err)
	assert.Equal(t, 5*time.Second, timeout)
}

func TestConfig_Get_OverrideReplacesTypeFixed(t *testing.T) {
	t.Parallel()

	quoted := `enabled: "true"`
	untyped := map[string]any{"enabled": "true"}

	t.Run("untyped over quoted", func(t *testing.T) {
		t.Parallel()

		cfg := buildConfig(t, yamlCollector(t, quoted), collectors.NewMap(untyped))

		var got bool

		_, err := cfg.Get(config.NewKeyPath("enabled"), &got)
		require.NoError(t, err, "the overriding value brings its own, unfixed type")
		assert.True(t, got)
	})

	t.Run("quoted over untyped", func(t *testing.T) {
		t.Parallel()

		cfg := buildConfig(t, collectors.NewMap(untyped), yamlCollector(t, quoted))

		var got bool

		_, err := cfg.Get(config.NewKeyPath("enabled"), &got)
		require.ErrorIs(t, err, tree.ErrFixedTypeString)
	})
}

// Not parallel: t.Setenv.
func TestConfig_Get_EnvOverQuotedYAML(t *testing.T) {
	t.Setenv("GOCONFIG_TYPEFIXED_ENABLED", "true")

	cfg := buildConfig(t,
		yamlCollector(t, `enabled: "false"`),
		collectors.NewEnv().WithPrefix("GOCONFIG_TYPEFIXED_"),
	)

	var got bool

	_, err := cfg.Get(config.NewKeyPath("enabled"), &got)
	require.NoError(t, err)
	assert.True(t, got)
}

func TestBuilder_JSONSchema_QuotedYAMLString(t *testing.T) {
	t.Parallel()

	build := func(t *testing.T, cols ...config.Collector) []error {
		t.Helper()

		builder := config.NewBuilder()
		builder, err := builder.WithJSONSchema(strings.NewReader(typeFixedSchema))
		require.NoError(t, err)

		for _, col := range cols {
			builder = builder.AddCollector(col)
		}

		_, errs := builder.Build(t.Context())

		return errs
	}

	t.Run("plain", func(t *testing.T) {
		t.Parallel()

		assert.Empty(t, build(t, yamlCollector(t, "enabled: true\nport: 3301\n")))
	})

	t.Run("quoted", func(t *testing.T) {
		t.Parallel()

		errs := build(t, yamlCollector(t, "enabled: \"true\"\nport: '3301'\n"))
		require.Len(t, errs, 2)

		msgs := errs[0].Error() + "\n" + errs[1].Error()
		assert.Contains(t, msgs, "enabled [type]")
		assert.Contains(t, msgs, "port [type]")
	})

	t.Run("untyped over quoted", func(t *testing.T) {
		t.Parallel()

		assert.Empty(t, build(t,
			yamlCollector(t, "enabled: \"true\"\nport: '3301'\n"),
			collectors.NewMap(map[string]any{"enabled": "true", "port": "3301"}),
		))
	})
}

func TestConfig_Effective_KeepsTypeFixed(t *testing.T) {
	t.Parallel()

	builder := config.NewBuilder()

	builder = builder.AddCollector(yamlCollector(t, `enabled: "true"
groups:
  g:
    replicasets:
      r:
        instances:
          i: {}
`))
	builder = builder.WithInheritance(config.Levels(config.Global, "groups", "replicasets", "instances"))

	cfg, errs := builder.Build(t.Context())
	require.Empty(t, errs)

	instance, err := cfg.Effective(config.NewKeyPath("groups/g/replicasets/r/instances/i"))
	require.NoError(t, err)

	var got bool

	_, err = instance.Get(config.NewKeyPath("enabled"), &got)
	require.ErrorIs(t, err, tree.ErrFixedTypeString, "an inherited value keeps the type its source fixed")
}

// fixedTree builds a tree holding value at path with the leaf's type fixed.
func fixedTree(path string, value any) *tree.Node {
	root := tree.New()
	root.Set(config.NewKeyPath(path), value)
	root.Get(config.NewKeyPath(path)).SetTypeFixed(true)

	return root
}

func TestMergeCollector_MapValueClearsTypeFixedOfChildren(t *testing.T) {
	t.Parallel()

	root := fixedTree("svc/enabled", "true")

	// A value that is itself a map replaces the leaves below its path.
	col := collectors.NewMock().WithEntry(config.NewKeyPath("svc"), map[string]any{"enabled": "true"})

	require.NoError(t, config.MergeCollector(t.Context(), root, col))
	assert.False(t, root.Get(config.NewKeyPath("svc/enabled")).TypeFixed())
}

// directMerger writes a value into an existing node itself instead of going
// through the default merge logic.
type directMerger struct{}

func (directMerger) CreateContext(col config.Collector) config.MergerContext {
	return config.Default.CreateContext(col)
}

func (directMerger) MergeValue(_ config.MergerContext, root *tree.Node, path config.KeyPath, value any) error {
	root.Get(path).Value = value

	return nil
}

func TestMergeCollectorWithMerger_CustomMergerClearsTypeFixed(t *testing.T) {
	t.Parallel()

	root := fixedTree("enabled", "true")
	col := collectors.NewMock().WithEntry(config.NewKeyPath("enabled"), "true")

	require.NoError(t, config.MergeCollectorWithMerger(t.Context(), root, col, directMerger{}))
	assert.False(t, root.Get(config.NewKeyPath("enabled")).TypeFixed(),
		"the node takes the flag of the value merged into it, whatever the merger did")
}

func buildMutable(t *testing.T, cols ...config.Collector) *config.MutableConfig {
	t.Helper()

	builder := config.NewBuilder()
	for _, col := range cols {
		builder = builder.AddCollector(col)
	}

	mutable, errs := builder.BuildMutable(t.Context())
	require.Empty(t, errs)

	return &mutable
}

func TestMutableConfig_Set_ClearsTypeFixed(t *testing.T) {
	t.Parallel()

	mutable := buildMutable(t, yamlCollector(t, `enabled: "false"`))

	require.NoError(t, mutable.Set(config.NewKeyPath("enabled"), "true"))

	var got bool

	_, err := mutable.Get(config.NewKeyPath("enabled"), &got)
	require.NoError(t, err, "a value set at runtime carries no fixed type")
	assert.True(t, got)
}

func TestMutableConfig_MergeAndUpdate_KeepTypeFixed(t *testing.T) {
	t.Parallel()

	quoted := buildConfig(t, yamlCollector(t, `enabled: "true"`))

	apply := map[string]func(*config.MutableConfig) error{
		"merge":  func(mutable *config.MutableConfig) error { return mutable.Merge(&quoted) },
		"update": func(mutable *config.MutableConfig) error { return mutable.Update(&quoted) },
	}

	for name, mutate := range apply {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			mutable := buildMutable(t, collectors.NewMap(map[string]any{"enabled": false}))
			require.NoError(t, mutate(mutable))

			var got bool

			_, err := mutable.Get(config.NewKeyPath("enabled"), &got)
			require.ErrorIs(t, err, tree.ErrFixedTypeString, "the merged value keeps the type its source fixed")
		})
	}
}

func TestMutableConfig_MergeAndUpdate_KeepTypeFixedInEffective(t *testing.T) {
	t.Parallel()

	quoted := buildConfig(t, yamlCollector(t, `enabled: "true"`))

	apply := map[string]func(*config.MutableConfig) error{
		"merge":  func(mutable *config.MutableConfig) error { return mutable.Merge(&quoted) },
		"update": func(mutable *config.MutableConfig) error { return mutable.Update(&quoted) },
	}

	for name, mutate := range apply {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			builder := config.NewBuilder()

			builder = builder.AddCollector(collectors.NewMap(map[string]any{
				"enabled": false,
				"groups": map[string]any{"g": map[string]any{"replicasets": map[string]any{
					"r": map[string]any{"instances": map[string]any{"i": map[string]any{}}},
				}}},
			}))
			builder = builder.WithInheritance(config.Levels(config.Global, "groups", "replicasets", "instances"))

			mutable, errs := builder.BuildMutable(t.Context())
			require.Empty(t, errs)
			require.NoError(t, mutate(&mutable))

			// The effective view is folded from the layers and the runtime
			// overlay, not from the merged tree Get reads.
			instance, err := mutable.Effective(config.NewKeyPath("groups/g/replicasets/r/instances/i"))
			require.NoError(t, err)

			var got bool

			_, err = instance.Get(config.NewKeyPath("enabled"), &got)
			require.ErrorIs(t, err, tree.ErrFixedTypeString)
		})
	}
}
