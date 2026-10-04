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
					source.Clone(), []*tree.Node{source}, []inheritanceConfig{inheritanceCfg}, nil,
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

func TestTemplateIndex_ConcurrentColdSubtrees(t *testing.T) {
	t.Parallel()

	for _, layered := range []bool{false, true} {
		t.Run(strconv.FormatBool(layered), func(t *testing.T) {
			t.Parallel()

			source := tree.New()
			for _, name := range []string{"first", "second", "third"} {
				source.Set(KeyPath{"instances", name, "value"}, name+"-{{name}}")

				for leaf := range 16 {
					source.Set(KeyPath{"instances", name, "shared", strconv.Itoa(leaf)},
						fmt.Sprintf("%s-%d-{{name}}", name, leaf))
				}
			}

			inheritanceCfg := inheritanceConfig{levels: Levels(Global, "instances")}
			WithTemplateVariables(func(path KeyPath) map[string]string {
				return map[string]string{"name": path.Leaf()}
			})(&inheritanceCfg)

			cfg := newConfig(source, []inheritanceConfig{inheritanceCfg}, nil)
			if layered {
				cfg = newLayeredConfig(source.Clone(), []*tree.Node{source}, []inheritanceConfig{inheritanceCfg}, nil)
			}

			var workers sync.WaitGroup

			start := make(chan struct{})

			for i := range 48 {
				workers.Go(func() {
					<-start

					name := []string{"first", "second", "third"}[i%3]

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

					var shared map[string]string

					_, err = view.Get(KeyPath{"shared"}, &shared)
					if err != nil {
						t.Error(err)
						return
					}

					for leaf := range 16 {
						key := strconv.Itoa(leaf)
						if shared[key] != fmt.Sprintf("%s-%d-%s", name, leaf, name) {
							t.Errorf("shared[%s] = %q", key, shared[key])
						}
					}
				})
			}

			close(start)
			workers.Wait()

			cfg.templates.mu.RLock()
			defer cfg.templates.mu.RUnlock()

			for _, name := range []string{"first", "second", "third"} {
				require.Contains(t, cfg.templates.plans, source.Get(KeyPath{"instances", name, "value"}))
			}

			require.Len(t, cfg.templates.plans, 3*2, "one summary for each value and shared subtree")
		})
	}
}

func TestTemplateIndex_ReentrantProvider(t *testing.T) {
	t.Parallel()

	source := tree.New()
	source.Set(KeyPath{"instances", "outer", "value"}, "{{name}}")
	source.Set(KeyPath{"instances", "inner", "value"}, "{{name}}")

	var (
		cfg       Config
		nestedErr error
	)

	inheritanceCfg := inheritanceConfig{levels: Levels(Global, "instances")}
	WithTemplateVariables(func(path KeyPath) map[string]string {
		if path.Leaf() == "outer" {
			_, nestedErr = cfg.Effective(KeyPath{"instances", "inner"})
		}

		return map[string]string{"name": path.Leaf()}
	})(&inheritanceCfg)

	cfg = newConfig(source, []inheritanceConfig{inheritanceCfg}, nil)

	view, err := cfg.Effective(KeyPath{"instances", "outer"})
	require.NoError(t, err)
	require.NoError(t, nestedErr)

	var value string

	_, err = view.Get(KeyPath{"value"}, &value)
	require.NoError(t, err)
	require.Equal(t, "outer", value)
}

func TestTemplateIndex_PrunedSources(t *testing.T) {
	t.Parallel()

	source := tree.New()
	source.Set(NewKeyPath("app/keep"), "global-{{name}}")
	source.Set(NewKeyPath("app/excluded"), "excluded-{{name}}")
	source.Set(NewKeyPath("app/deleted"), "deleted-{{name}}")
	source.Set(NewKeyPath("instances/first/app/local"), "local-{{name}}")
	source.Set(NewKeyPath("blocked/branch/secret"), "secret-{{name}}")
	source.Set(NewKeyPath("blocked/branch/visible"), "visible-{{name}}")
	source.Set(NewKeyPath("replaced/old"), "replaced-old-{{name}}")
	source.Set(NewKeyPath("instances/first/replaced/new"), "replaced-new-{{name}}")
	source.Set(NewKeyPath("nested/override/old"), "nested-old-{{name}}")
	source.Set(NewKeyPath("instances/first/nested/override/new"), "nested-new-{{name}}")

	globalTags := templateTestArray("global-{{name}}")
	source.SetChild("tags", globalTags)

	instance := source.Get(NewKeyPath("instances/first"))
	instance.SetChild("tags", templateTestArray("instance-{{name}}"))

	upper := tree.New()
	upper.Set(NewKeyPath("app/upper"), "upper-{{name}}")

	name := "first"
	inheritanceCfg := inheritanceConfig{levels: Levels(Global, "instances")}
	WithNoInherit("app/excluded")(&inheritanceCfg)
	WithNoInheritFrom(Global, "blocked/branch/secret")(&inheritanceCfg)
	WithInheritMerge("replaced", MergeReplace)(&inheritanceCfg)
	WithInheritMerge("nested/override", MergeReplace)(&inheritanceCfg)
	WithInheritMerge("tags", MergeAppend)(&inheritanceCfg)
	WithDefaults(DefaultsType{"app/default": "default-{{name}}"})(&inheritanceCfg)
	WithTemplateVariables(func(KeyPath) map[string]string {
		return map[string]string{"name": name}
	})(&inheritanceCfg)

	cfg := newLayeredConfig(source.Clone(), []*tree.Node{source, upper}, []inheritanceConfig{inheritanceCfg}, nil)

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

		var value string

		_, err = view.Get(KeyPath{"replaced", "new"}, &value)
		require.NoError(t, err)
		require.Equal(t, "replaced-new-"+name, value)

		_, err = view.Get(KeyPath{"replaced", "old"}, &value)
		require.Error(t, err)

		_, err = view.Get(KeyPath{"nested", "override", "new"}, &value)
		require.NoError(t, err)
		require.Equal(t, "nested-new-"+name, value)

		_, err = view.Get(KeyPath{"nested", "override", "old"}, &value)
		require.Error(t, err)

		var tags []string

		_, err = view.Get(KeyPath{"tags"}, &tags)
		require.NoError(t, err)
		require.Equal(t, []string{"global-" + name, "instance-" + name}, tags)

		for node := range cfg.templates.plans {
			require.True(t, allowed[node], "the index must retain only source nodes")
		}

		for _, path := range []string{
			"blocked/branch/secret", "replaced/old", "nested/override/old",
		} {
			require.NotContains(t, cfg.templates.plans, source.Get(NewKeyPath(path)), path)
		}

		compiledNodes := templateTestCompiledNodes(cfg.templates)
		for _, path := range []string{
			"blocked/branch/visible", "instances/first/replaced/new",
			"instances/first/nested/override/new", "tags/0", "instances/first/tags/0",
		} {
			require.Contains(t, compiledNodes, source.Get(NewKeyPath(path)), path)
		}

		if iteration == 0 {
			count = len(cfg.templates.plans)
		}

		require.Len(t, cfg.templates.plans, count)
	}

	require.NotContains(t, cfg.templates.plans, source.Child("app"))
	require.NotContains(t, cfg.templates.plans, upper.Child("app"))
	require.NotContains(t, cfg.templates.plans, cfg.modified.Child("app"))
	require.NotContains(t, cfg.templates.plans, source.Get(NewKeyPath("app/excluded")))
	require.NotContains(t, cfg.templates.plans, source.Get(NewKeyPath("app/deleted")))

	_, oldKeepCached := cfg.templates.plans[source.Get(NewKeyPath("app/keep"))]
	require.False(t, oldKeepCached)
	require.Contains(t, cfg.templates.plans, source.Get(NewKeyPath("instances/first/app/local")))
	require.Contains(t, cfg.templates.plans, upper.Get(NewKeyPath("app/upper")))
	require.Contains(t, cfg.templates.plans, cfg.modified.Get(NewKeyPath("app/keep")))
}

// Summaries contain descendant plans directly, without indexing each child.
func templateTestCompiledNodes(index *templateIndex) map[*tree.Node]bool {
	nodes := make(map[*tree.Node]bool)

	var collect func(*tree.Node, *templatePlan)

	collect = func(node *tree.Node, plan *templatePlan) {
		if plan == nil {
			return
		}

		nodes[node] = true
		for _, child := range plan.children {
			collect(node.Child(child.key), child.plan)
		}
	}

	for node, summary := range index.plans {
		collect(node, summary.plan)
	}

	return nodes
}

func TestTemplateIndex_CompactSubtree(t *testing.T) {
	t.Parallel()

	for _, templated := range []bool{false, true} {
		t.Run(strconv.FormatBool(templated), func(t *testing.T) {
			t.Parallel()

			source := tree.New()
			for i := range 10000 {
				source.Set(KeyPath{strconv.Itoa(i), "value"}, "literal")
			}

			if templated {
				source.Set(KeyPath{"tag"}, "{{name}}")
			}

			index := newTemplateIndex()
			plan := index.sourcePlan(source, nil)
			require.Len(t, index.plans, 1, "ordinary descendants must not get separate cache entries")

			if templated {
				require.NotNil(t, plan)
				require.Len(t, plan.children, 1)
			} else {
				require.Nil(t, plan)
			}

			summary := index.plans[source]
			for range 10 {
				require.Equal(t, plan, index.sourcePlan(source, nil))
				require.Same(t, summary, index.plans[source])
			}

			require.Len(t, index.plans, 1)
		})
	}
}

func TestTemplateIndex_PendingChildren(t *testing.T) {
	t.Parallel()

	source := tree.New()
	source.Set(NewKeyPath("data/keep"), "keep-{{name}}")
	source.Set(NewKeyPath("data/hidden/value"), "hidden-{{name}}")
	source.Set(NewKeyPath("instances/first/data/hidden"), "literal")
	source.Set(NewKeyPath("instances/second/present"), true)

	inheritanceCfg := inheritanceConfig{levels: Levels(Global, "instances")}
	WithTemplateVariables(func(path KeyPath) map[string]string {
		return map[string]string{"name": path.Leaf()}
	})(&inheritanceCfg)

	cfg := newLayeredConfig(source.Clone(), []*tree.Node{source}, []inheritanceConfig{inheritanceCfg}, nil)

	first, err := cfg.Effective(NewKeyPath("instances/first"))
	require.NoError(t, err)

	var value string

	_, err = first.Get(NewKeyPath("data/hidden"), &value)
	require.NoError(t, err)
	require.Equal(t, "literal", value)

	partial := cfg.templates.plans[source.Child("data")]
	require.NotNil(t, partial)
	require.Contains(t, partial.pending, "hidden")
	require.NotContains(t, templateTestCompiledNodes(cfg.templates), source.Get(NewKeyPath("data/hidden/value")))

	second, err := cfg.Effective(NewKeyPath("instances/second"))
	require.NoError(t, err)

	_, err = second.Get(NewKeyPath("data/hidden/value"), &value)
	require.NoError(t, err)
	require.Equal(t, "hidden-second", value)
	require.Empty(t, cfg.templates.plans[source.Child("data")].pending)
	require.Contains(t, partial.pending, "hidden", "published summaries must remain immutable")

	first, err = cfg.Effective(NewKeyPath("instances/first"))
	require.NoError(t, err)

	_, err = first.Get(NewKeyPath("data/hidden"), &value)
	require.NoError(t, err)
	require.Equal(t, "literal", value, "a completed source plan must still honor overrides")
}

func TestTemplateIndex_ConcurrentPartialSummaries(t *testing.T) {
	t.Parallel()

	const width = 24

	source := tree.New()

	for i := range width {
		key := strconv.Itoa(i)
		source.Set(KeyPath{"data", key}, "source-{{name}}")
		source.Set(KeyPath{"instances", key, "data", key}, "local-{{name}}")
	}

	inheritanceCfg := inheritanceConfig{levels: Levels(Global, "instances")}
	WithTemplateVariables(func(path KeyPath) map[string]string {
		return map[string]string{"name": path.Leaf()}
	})(&inheritanceCfg)

	cfg := newLayeredConfig(source.Clone(), []*tree.Node{source}, []inheritanceConfig{inheritanceCfg}, nil)

	var workers sync.WaitGroup

	start := make(chan struct{})

	for i := range width {
		workers.Go(func() {
			<-start

			name := strconv.Itoa(i)
			for range 3 {
				view, err := cfg.Effective(KeyPath{"instances", name})
				if err != nil {
					t.Error(err)
					return
				}

				var data map[string]string

				_, err = view.Get(KeyPath{"data"}, &data)
				if err != nil {
					t.Error(err)
					return
				}

				for j := range width {
					key := strconv.Itoa(j)

					prefix := "source-"
					if i == j {
						prefix = "local-"
					}

					if data[key] != prefix+name {
						t.Errorf("instance %s data[%s] = %q, want %q", name, key, data[key], prefix+name)
					}
				}
			}
		})
	}

	close(start)
	workers.Wait()

	summary := cfg.templates.plans[source.Child("data")]
	require.NotNil(t, summary)
	require.Empty(t, summary.pending)
	require.Len(t, summary.plan.children, width, "concurrent completion must not duplicate children")
	require.Len(t, cfg.templates.plans, width+1, "one shared summary and one local value per instance")
}

func TestTemplateIndex_MergedKeyCollisions(t *testing.T) {
	t.Parallel()

	for _, reverse := range []bool{false, true} {
		t.Run(strconv.FormatBool(reverse), func(t *testing.T) {
			t.Parallel()

			source := tree.New()

			keys := []string{"{{name}}", "target"}
			if reverse {
				keys[0], keys[1] = keys[1], keys[0]
			}

			for _, key := range keys {
				source.Set(KeyPath{"data", key}, key+"-{{name}}")
			}

			source.Set(NewKeyPath("instances/first/data/{{name}}"), "overridden-{{name}}")

			inheritanceCfg := inheritanceConfig{levels: Levels(Global, "instances")}
			WithTemplateVariables(func(KeyPath) map[string]string {
				return map[string]string{"name": "target"}
			})(&inheritanceCfg)

			cfg := newLayeredConfig(source.Clone(), []*tree.Node{source}, []inheritanceConfig{inheritanceCfg}, nil)

			for range 5 {
				view, err := cfg.Effective(NewKeyPath("instances/first"))
				require.NoError(t, err)

				var data map[string]string

				_, err = view.Get(KeyPath{"data"}, &data)
				require.NoError(t, err)

				want := "target-target"
				if reverse {
					want = "overridden-target"
				}

				require.Equal(t, map[string]string{"target": want}, data)
			}
		})
	}
}

func templateTestArray(values ...string) *tree.Node {
	array := tree.New()
	array.MarkArray()

	for i, value := range values {
		leaf := tree.New()

		leaf.Value = value
		array.SetChild(strconv.Itoa(i), leaf)
	}

	return array
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

//nolint:paralleltest // AllocsPerRun requires serial execution.
func TestExpandPlannedLeaf_UnchangedAllocations(t *testing.T) {
	for _, tt := range []struct {
		name string
		text string
		vars map[string]string
	}{
		{
			name: "nil variables",
			text: "before-{{unknown}}-after",
		},
		{
			name: "unknown variables",
			text: "{{unknown}}-{{ other }}",
			vars: map[string]string{"name": "value"},
		},
		{
			name: "identical replacement",
			text: "before-{{ name }}-after",
			vars: map[string]string{"name": "{{ name }}"},
		},
		{
			name: "identical and unknown replacements",
			text: "{{unknown}}-{{ name }}-{{ name }}-{{missing}}",
			vars: map[string]string{"name": "{{ name }}"},
		},
		{
			name: "large unknown string",
			text: strings.Repeat("x", 256*1024) + "{{unknown}}",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			node := tree.New()

			node.Value = tt.text

			plan := compileTemplateLeaf(node)
			require.NotNil(t, plan)

			var expanded *tree.Node

			allocations := testing.AllocsPerRun(100, func() {
				expanded = expandPlannedLeaf(node, plan, tt.vars)
			})

			require.Same(t, node, expanded)
			require.Equal(t, tt.text, expanded.Value)
			require.Zero(t, allocations, "unchanged strings must reuse the source buffer and node")
		})
	}
}

func TestExpandCompiledTemplate_Replacements(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name string
		text string
		vars map[string]string
		want string
	}{
		{
			name: "unknown before and after changes",
			text: "prefix-{{unknown}}-{{name}}-{{unknown}}-{{name}}-{{unknown}}-suffix",
			vars: map[string]string{"name": "value"},
			want: "prefix-{{unknown}}-value-{{unknown}}-value-{{unknown}}-suffix",
		},
		{
			name: "identical before and after changes",
			text: "{{same}}-{{name}}-{{same}}",
			vars: map[string]string{"same": "{{same}}", "name": "value"},
			want: "{{same}}-value-{{same}}",
		},
		{
			name: "empty first replacement",
			text: "{{empty}}{{unknown}}{{name}}{{empty}}",
			vars: map[string]string{"empty": "", "name": "value"},
			want: "{{unknown}}value",
		},
		{
			name: "empty result",
			text: "{{empty}}{{empty}}",
			vars: map[string]string{"empty": ""},
			want: "",
		},
		{
			name: "replacement is not expanded again",
			text: "{{name}}/{{unknown}}",
			vars: map[string]string{"name": "{{other}}", "other": "value"},
			want: "{{other}}/{{unknown}}",
		},
		{
			name: "padding is part of the original placeholder",
			text: "{{name}}/{{ name }}",
			vars: map[string]string{"name": "{{ name }}"},
			want: "{{ name }}/{{ name }}",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			matches := compileTemplate(tt.text)
			require.Equal(t, tt.want, expandCompiledTemplate(tt.text, matches, tt.vars))
		})
	}
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
