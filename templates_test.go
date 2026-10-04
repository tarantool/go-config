package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	config "github.com/tarantool/go-config/v2"
	"github.com/tarantool/go-config/v2/collectors"
)

const (
	templateValueKey     = "value"
	templateNestedKey    = "nested"
	templateEmptyVar     = "empty"
	templateFirstEntity  = "first"
	templateNameVar      = "name"
	templateEnabledCase  = "enabled"
	templateOtherVar     = "other"
	templateNewValue     = "new"
	templateInstancesKey = "instances"
)

func TestEffective_TemplateVariables(t *testing.T) {
	t.Parallel()

	const nameTemplate = "{{ name }}"

	data := map[string]any{
		templateValueKey: nameTemplate,
		templateNestedKey: map[string]any{
			nameTemplate: "{{ special }}",
		},
		"map_leaf":           map[string]string{nameTemplate: nameTemplate},
		"slice_leaf":         []any{nameTemplate, map[string]any{nameTemplate: nameTemplate}},
		"array_leaf":         [2]string{nameTemplate, "literal"},
		"once":               "{{ once }}",
		templateEmptyVar:     "before{{ empty }}after",
		"spaces":             "{{   name   }}{{name}}",
		"unfinished":         "{{ name",
		"number":             42,
		"nil":                nil,
		"bytes":              []byte("{{name}}"),
		templateInstancesKey: map[string]any{templateFirstEntity: map[string]any{}, "second": map[string]any{}},
	}
	resolve := func(path config.KeyPath) map[string]string {
		return map[string]string{
			templateNameVar: path.Leaf(), "special": "$1\\%value", "once": "{{unknown}}", templateEmptyVar: "",
		}
	}

	builder := config.NewBuilder()

	builder = builder.AddCollector(collectors.NewMap(data))
	builder = builder.WithInheritance(config.Levels(config.Global, templateInstancesKey),
		config.WithTemplateVariables(resolve))

	cfg, errs := builder.Build(t.Context())
	require.Empty(t, errs)

	for _, name := range []string{templateFirstEntity, "second"} {
		effective, err := cfg.Effective(config.NewKeyPath("instances/" + name))
		require.NoError(t, err)

		var values map[string]any

		_, err = effective.Get(nil, &values)
		require.NoError(t, err)
		assert.Equal(t, name, values[templateValueKey])
		assert.Equal(t, map[string]any{name: "$1\\%value"}, values[templateNestedKey])

		for _, key := range []string{"map_leaf", "slice_leaf", "array_leaf"} {
			assert.Equal(t, data[key], values[key], "leaf containers remain unchanged")
		}

		assert.Equal(t, "{{unknown}}", values["once"], "replacement values are expanded only once")
		assert.Equal(t, "beforeafter", values[templateEmptyVar])
		assert.Equal(t, name+name, values["spaces"])
		assert.Equal(t, "{{ name", values["unfinished"])
		assert.Equal(t, 42, values["number"])
		assert.Nil(t, values["nil"])
		assert.Equal(t, []byte("{{name}}"), values["bytes"])
	}

	var raw map[string]any

	_, err := cfg.Get(config.NewKeyPath(templateNestedKey), &raw)
	require.NoError(t, err)
	assert.Equal(t, data[templateNestedKey], raw, "raw nested mappings retain their templates")
	assert.Equal(t, nameTemplate, data[templateValueKey])

	sliced, err := cfg.Slice(nil)
	require.NoError(t, err)

	effective, err := sliced.Effective(config.NewKeyPath("instances/" + templateFirstEntity))
	require.NoError(t, err, "a config without source layers also expands templates")

	var resolved string

	_, err = effective.Get(config.NewKeyPath(templateValueKey), &resolved)
	require.NoError(t, err)
	assert.Equal(t, templateFirstEntity, resolved)
}

func TestEffective_TemplateVariables_OptionalAndCombined(t *testing.T) {
	t.Parallel()

	for _, enabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "disabled", true: templateEnabledCase}[enabled], func(t *testing.T) {
			t.Parallel()

			builder := config.NewBuilder()

			builder = builder.AddCollector(collectors.NewMap(map[string]any{
				templateValueKey: "{{name}}-{{other}}", templateInstancesKey: map[string]any{"i": map[string]any{}},
			}))

			opts := []config.InheritanceOption{config.WithTemplateVariables(nil)}

			if enabled {
				opts = append(opts,
					config.WithTemplateVariables(func(config.KeyPath) map[string]string {
						return map[string]string{templateNameVar: "old", templateOtherVar: "kept"}
					}),
					config.WithTemplateVariables(func(config.KeyPath) map[string]string {
						return map[string]string{templateNameVar: templateNewValue}
					}),
				)
			}

			builder = builder.WithInheritance(config.Levels(config.Global, templateInstancesKey), opts...)

			cfg, errs := builder.Build(t.Context())
			require.Empty(t, errs)

			effective, err := cfg.Effective(config.NewKeyPath("instances/i"))
			require.NoError(t, err)

			var value string

			_, err = effective.Get(config.NewKeyPath(templateValueKey), &value)
			require.NoError(t, err)

			if enabled {
				assert.Equal(t, "new-kept", value)
			} else {
				assert.Equal(t, "{{name}}-{{other}}", value)
			}
		})
	}
}

func TestEffective_TemplateVariables_ProvidersOnlyForPlaceholders(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name       string
		value      any
		localValue string
		options    []config.InheritanceOption
		wantCalls  bool
	}{
		{name: "literal", value: "literal"},
		{name: "unfinished placeholder", value: "{{ name"},
		{name: "opaque leaf", value: []any{"{{name}}"}},
		{
			name: "excluded template", value: "{{name}}",
			options: []config.InheritanceOption{config.WithNoInherit(templateValueKey)},
		},
		{name: "overridden template", value: "{{name}}", localValue: "literal"},
		{name: "value template", value: "{{name}}", wantCalls: true},
		{name: "key template", value: map[string]any{"{{name}}": "literal"}, wantCalls: true},
		{name: "unknown variable", value: "{{unknown}}", wantCalls: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			for _, mode := range []string{"layered", "slice"} {
				t.Run(mode, func(t *testing.T) {
					t.Parallel()

					local := map[string]any{"present": true}
					if tt.localValue != "" {
						local[templateValueKey] = tt.localValue
					}

					builder := config.NewBuilder()

					builder = builder.AddCollector(collectors.NewMap(map[string]any{
						templateValueKey:     tt.value,
						templateInstancesKey: map[string]any{templateFirstEntity: local},
					}))

					var calls []int

					options := append([]config.InheritanceOption{}, tt.options...)

					options = append(options,
						config.WithTemplateVariables(func(config.KeyPath) map[string]string {
							calls = append(calls, 1)

							return map[string]string{templateNameVar: "initial"}
						}),
						config.WithTemplateVariables(func(config.KeyPath) map[string]string {
							calls = append(calls, 2)

							return map[string]string{templateNameVar: "resolved"}
						}),
					)
					builder = builder.WithInheritance(config.Levels(config.Global, templateInstancesKey), options...)

					cfg, errs := builder.Build(t.Context())
					require.Empty(t, errs)

					if mode == "slice" {
						sliced, err := cfg.Slice(nil)
						require.NoError(t, err)

						cfg = sliced
					}

					for range 2 {
						_, err := cfg.Effective(config.KeyPath{templateInstancesKey, templateFirstEntity})
						require.NoError(t, err)
					}

					if tt.wantCalls {
						require.Equal(t, []int{1, 2, 1, 2}, calls)
					} else {
						require.Empty(t, calls)
					}
				})
			}
		})
	}
}

func TestEffective_TemplateVariables_ResolvedStringIsFixed(t *testing.T) {
	t.Parallel()

	builder := config.NewBuilder()

	builder = builder.AddCollector(collectors.NewMap(map[string]any{
		templateValueKey: "{{name}}", templateInstancesKey: map[string]any{"i": map[string]any{}},
	}))
	builder = builder.WithInheritance(config.Levels(config.Global, templateInstancesKey),
		config.WithTemplateVariables(func(config.KeyPath) map[string]string {
			return map[string]string{templateNameVar: "false"}
		}))

	cfg, errs := builder.Build(t.Context())
	require.Empty(t, errs)

	effective, err := cfg.Effective(config.NewKeyPath("instances/i"))
	require.NoError(t, err)

	var boolean bool

	_, err = effective.Get(config.NewKeyPath(templateValueKey), &boolean)
	require.Error(t, err)

	var value string

	_, err = effective.Get(config.NewKeyPath(templateValueKey), &value)
	require.NoError(t, err)
	assert.Equal(t, "false", value)
}

func TestEffective_TemplateVariables_PreserveUnknown(t *testing.T) {
	t.Parallel()

	const (
		unknown = "{{  remote\t name  }}"
		mixed   = "{{ name }}/" + unknown + "/{{ empty }}/{{\tname\t}}"
	)

	data := map[string]any{
		templateValueKey:        mixed,
		"{{ name }}/" + unknown: "{{name}}-" + unknown,
		templateNestedKey: map[string]any{
			"{{ name }}/" + unknown: mixed,
			"once":                  "{{ once }}",
		},
		templateInstancesKey: map[string]any{templateFirstEntity: map[string]any{}, "second": map[string]any{}},
	}
	builder := config.NewBuilder()

	builder = builder.AddCollector(collectors.NewMap(data))
	builder = builder.WithInheritance(config.Levels(config.Global, templateInstancesKey),
		config.WithTemplateVariables(func(path config.KeyPath) map[string]string {
			return map[string]string{templateNameVar: path.Leaf(), templateEmptyVar: "", "once": "{{ name }}"}
		}))

	cfg, errs := builder.BuildMutable(t.Context())
	require.Empty(t, errs)

	before, err := cfg.MarshalYAML()
	require.NoError(t, err)

	all, err := cfg.EffectiveAll()
	require.NoError(t, err)

	for _, name := range []string{templateFirstEntity, "second"} {
		effective := all["instances/"+name]
		expected := name + "/" + unknown + "//{{\tname\t}}"

		var value string

		_, err = effective.Get(config.KeyPath{templateValueKey}, &value)
		require.NoError(t, err)
		assert.Equal(t, expected, value)

		_, err = effective.Get(config.KeyPath{name + "/" + unknown}, &value)
		require.NoError(t, err)
		assert.Equal(t, name+"-"+unknown, value)

		var nested map[string]any

		_, err = effective.Get(config.KeyPath{templateNestedKey}, &nested)
		require.NoError(t, err)
		assert.Equal(t, map[string]any{
			name + "/" + unknown: expected,
			"once":               "{{ name }}",
		}, nested)
	}

	after, err := cfg.MarshalYAML()
	require.NoError(t, err)
	assert.Equal(t, before, after)
	require.NoError(t, cfg.Set(config.KeyPath{templateValueKey}, "{{ name }}-{{ unresolved }}"))

	snapshot := cfg.Snapshot()
	effective, err := snapshot.Effective(config.NewKeyPath("instances/first"))
	require.NoError(t, err)

	var changed string

	_, err = effective.Get(config.KeyPath{templateValueKey}, &changed)
	require.NoError(t, err)
	assert.Equal(t, "first-{{ unresolved }}", changed)
}
