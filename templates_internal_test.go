package config

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tarantool/go-config/v2/tree"
)

func TestTemplateIndex_Lazy(t *testing.T) {
	t.Parallel()

	for _, layered := range []bool{false, true} {
		t.Run(strconv.FormatBool(layered), func(t *testing.T) {
			t.Parallel()

			source := tree.New()
			source.Set(KeyPath{"common"}, "global-{{name}}")
			source.Set(KeyPath{"instances", "first", "value"}, "first-{{name}}")
			source.Set(KeyPath{"instances", "second", "value"}, "second-{{name}}")

			inheritanceCfg := inheritanceConfig{levels: Levels(Global, "instances")}
			WithTemplateVariables(func(path KeyPath) map[string]string {
				return map[string]string{"name": path.Leaf()}
			})(&inheritanceCfg)

			cfg := newConfig(source, []inheritanceConfig{inheritanceCfg}, nil)
			if layered {
				cfg = newLayeredConfig(
					cloneNode(source), []*tree.Node{source}, []inheritanceConfig{inheritanceCfg}, nil,
				)
			}

			first, err := cfg.Effective(KeyPath{"instances", "first"})
			require.NoError(t, err)

			var value string

			_, err = first.Get(KeyPath{"common"}, &value)
			require.NoError(t, err)
			require.Equal(t, "global-first", value)
			require.Contains(t, cfg.templates.plans, source.Child("common"))
			require.NotContains(t, cfg.templates.plans, source.Get(KeyPath{"instances", "second", "value"}))
			require.NotContains(t, cfg.templates.plans, cfg.root)

			count := len(cfg.templates.plans)
			for range 100 {
				_, err := cfg.Effective(KeyPath{"instances", "first"})
				require.NoError(t, err)
			}

			require.Len(t, cfg.templates.plans, count)

			var workers sync.WaitGroup
			for i := range 32 {
				workers.Go(func() {
					name := "first"
					if i%2 != 0 {
						name = "second"
					}

					view, err := cfg.Effective(KeyPath{"instances", name})
					if err != nil {
						t.Error(err)
						return
					}

					var value string

					_, err = view.Get(KeyPath{"value"}, &value)
					if err != nil {
						t.Error(err)
						return
					}

					if value != name+"-"+name {
						t.Errorf("got %q for %s", value, name)
					}
				})
			}

			workers.Wait()
			require.Len(t, cfg.templates.plans, count+1)
			require.Contains(t, cfg.templates.plans, source.Get(KeyPath{"instances", "second", "value"}))
		})
	}
}

func TestTemplateIndex_PrunedSources(t *testing.T) {
	t.Parallel()

	source := tree.New()
	source.Set(NewKeyPath("app/keep"), "global-{{name}}")
	source.Set(NewKeyPath("app/excluded"), "excluded-{{name}}")
	source.Set(NewKeyPath("app/deleted"), "deleted-{{name}}")
	source.Set(NewKeyPath("instances/first/app/local"), "local-{{name}}")

	upper := tree.New()
	upper.Set(NewKeyPath("app/upper"), "upper-{{name}}")

	name := "first"
	inheritanceCfg := inheritanceConfig{levels: Levels(Global, "instances")}
	WithNoInherit("app/excluded")(&inheritanceCfg)
	WithDefaults(DefaultsType{"app/default": "default-{{name}}"})(&inheritanceCfg)
	WithTemplateVariables(func(KeyPath) map[string]string {
		return map[string]string{"name": name}
	})(&inheritanceCfg)

	cfg := newLayeredConfig(cloneNode(source), []*tree.Node{source, upper}, []inheritanceConfig{inheritanceCfg}, nil)

	cfg.modified = tree.New()
	cfg.modified.Set(NewKeyPath("app/keep"), "modified-{{name}}")

	cfg.tombstones = []KeyPath{NewKeyPath("app/deleted")}

	allowed := make(map[*tree.Node]bool)

	var collect func(*tree.Node)

	collect = func(node *tree.Node) {
		allowed[node] = true
		for _, key := range node.ChildrenKeys() {
			collect(node.Child(key))
		}
	}
	collect(source)
	collect(upper)
	collect(cfg.modified)

	count := 0

	for iteration := range 10 {
		name = fmt.Sprintf("call-%d", iteration)

		view, err := cfg.Effective(NewKeyPath("instances/first"))
		require.NoError(t, err)

		var app map[string]string

		_, err = view.Get(KeyPath{"app"}, &app)
		require.NoError(t, err)
		require.Equal(t, map[string]string{
			"keep":    "modified-" + name,
			"local":   "local-" + name,
			"upper":   "upper-" + name,
			"default": "default-" + name,
		}, app)

		for node := range cfg.templates.plans {
			require.True(t, allowed[node], "the index must retain only source nodes")
		}

		if iteration == 0 {
			count = len(cfg.templates.plans)
		}

		require.Len(t, cfg.templates.plans, count)
	}

	require.Contains(t, cfg.templates.plans, source.Child("app"))
	require.Contains(t, cfg.templates.plans, upper.Child("app"))
	require.Contains(t, cfg.templates.plans, cfg.modified.Child("app"))
}

//nolint:paralleltest // AllocsPerRun requires serial execution.
func TestCompileTemplateLeaf_Allocations(t *testing.T) {
	node := tree.New()

	node.Value = "before-{{ name }}-after"

	var plan *templatePlan

	allocations := testing.AllocsPerRun(100, func() {
		plan = compileTemplateLeaf(node)
	})

	require.NotNil(t, plan)
	require.LessOrEqual(t, allocations, float64(2), "one plan and one contiguous match slice")
}

//nolint:paralleltest // AllocsPerRun requires serial execution.
func TestExpandPlannedLeaf_Allocations(t *testing.T) {
	node := tree.New()

	node.Value = "before-{{ name }}-after"

	plan := compileTemplateLeaf(node)
	vars := map[string]string{"name": "result"}

	var expanded *tree.Node

	allocations := testing.AllocsPerRun(100, func() {
		expanded = expandPlannedLeaf(node, plan, vars)
	})

	require.Equal(t, "before-result-after", expanded.Value)
	require.Equal(t, "before-{{ name }}-after", node.Value)
	require.True(t, expanded.TypeFixed())
	require.LessOrEqual(t, allocations, float64(3), "string bytes, boxed string, and detached node")
}

func TestExpandPlannedLeaf_NamedString(t *testing.T) {
	t.Parallel()

	type label string

	node := tree.New()

	node.Value = label("{{ name }}")

	plan := compileTemplateLeaf(node)
	expanded := expandPlannedLeaf(node, plan, map[string]string{"name": "resolved"})
	require.Equal(t, label("resolved"), expanded.Value)
	require.Equal(t, label("{{ name }}"), node.Value)
	require.True(t, expanded.TypeFixed())
}

func FuzzCompiledTemplate(f *testing.F) {
	for _, seed := range []string{
		"", "literal", "{{}}", "{{   }}", "{{ name }}", "{{ a  b }}", "{{name}}{{name}}",
		"{{\tname\t}}", "{{\nname\n}}", "{{\xc2\xa0name\xc2\xa0}}", "{{ name", "}}{{name}}",
		"{{{{name}}}}", "{{missing}}/{{ empty }}", "\xff{{name}}\xfe{{\xff}}", "{{ a }}-{{b}}",
	} {
		f.Add(seed, "$1\\value{{name}}")
	}

	pattern := regexp.MustCompile(`(?s)\{\{ *(.*?) *\}\}`)

	f.Fuzz(func(t *testing.T, text, replacement string) {
		vars := map[string]string{
			"name": replacement, "": "empty-name", "empty": "", "a": "A", "b": "B",
			"a  b": "internal-spaces", "\tname\t": "tabs", "\nname\n": "newlines", "\xff": "invalid-utf8",
		}

		for _, input := range []string{text, "prefix{{ " + text + " }}suffix{{name}}"} {
			reference := pattern.FindAllStringSubmatchIndex(input, -1)
			matches := compileTemplate(input)
			require.Len(t, matches, len(reference))

			for i, match := range matches {
				actualMatch := []int{match.start, match.end, match.nameStart, match.nameEnd}
				require.Equal(t, reference[i], actualMatch)
			}

			var expected strings.Builder

			end := 0
			for _, match := range reference {
				expected.WriteString(input[end:match[0]])

				value, found := vars[input[match[2]:match[3]]]
				if found {
					expected.WriteString(value)
				} else {
					expected.WriteString(input[match[0]:match[1]])
				}

				end = match[1]
			}

			expected.WriteString(input[end:])

			node := tree.New()

			node.Value = input

			plan := compileTemplateLeaf(node)

			actual := input
			if plan != nil {
				actual = expandCompiledTemplate(input, plan.matches, vars)
			}

			require.Equal(t, expected.String(), actual, "input %q", input)
		}
	})
}
