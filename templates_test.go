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
		templateNestedKey: []any{
			map[string]string{nameTemplate: "{{ special }}"}, [2]string{nameTemplate, "literal"},
		},
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
		assert.Equal(t,
			[]any{map[string]string{name: "$1\\%value"}, [2]string{name, "literal"}}, values[templateNestedKey])
		assert.Equal(t, "{{unknown}}", values["once"], "replacement values are expanded only once")
		assert.Equal(t, "beforeafter", values[templateEmptyVar])
		assert.Equal(t, name+name, values["spaces"])
		assert.Equal(t, "{{ name", values["unfinished"])
		assert.Equal(t, 42, values["number"])
		assert.Nil(t, values["nil"])
		assert.Equal(t, []byte("{{name}}"), values["bytes"])
	}

	var raw []any

	_, err := cfg.Get(config.NewKeyPath(templateNestedKey), &raw)
	require.NoError(t, err)
	assert.Equal(t, data[templateNestedKey], raw, "leaf containers are copied before expanding")
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
