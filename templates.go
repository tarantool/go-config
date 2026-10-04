package config

import (
	"reflect"
	"regexp"
	"strings"
	"sync"

	"github.com/tarantool/go-config/v2/tree"
)

// Tarantool permits ASCII spaces around names; tabs and newlines are part of
// the variable name. Values substituted into a string are not expanded again.
var templatePattern = regexp.MustCompile(`(?s)\{\{ *(.*?) *\}\}`)

func expandCompiledTemplate(text string, matches [][]int, vars map[string]string) string {
	if len(matches) == 0 {
		return text
	}

	var result strings.Builder

	result.Grow(len(text))

	end := 0

	for _, match := range matches {
		const nameStart, nameEnd = 2, 3

		name := text[match[nameStart]:match[nameEnd]]
		value, ok := vars[name]

		result.WriteString(text[end:match[0]])

		if !ok {
			result.WriteString(text[match[0]:match[1]])
		} else {
			result.WriteString(value)
		}

		end = match[1]
	}

	result.WriteString(text[end:])

	return result.String()
}

// templateIndex belongs to one published configuration version. Only source
// nodes are retained; merged effective nodes get plans local to that call.
// Only string leaves and mapping keys are inspected. Containers stored as leaf
// values are left unchanged. No template state is attached to public tree nodes.
type templateIndex struct {
	once  sync.Once
	plans map[*tree.Node]*templatePlan
}
type templatePlan struct {
	matches  [][]int
	children []templateChild
}
type templateChild struct {
	key     string
	matches [][]int
	plan    *templatePlan
}

func (index *templateIndex) prepare(cfg *Config) {
	index.once.Do(func() {
		index.plans = make(map[*tree.Node]*templatePlan)
		index.compile(cfg.root, true)

		for _, root := range cfg.layers {
			index.compile(root, true)
		}

		index.compile(cfg.modified, true)
	})
}

func newTemplateIndex() *templateIndex {
	return &templateIndex{once: sync.Once{}, plans: nil}
}

func (index *templateIndex) compile(node *tree.Node, retain bool) *templatePlan {
	if node == nil {
		return nil
	}

	if plan, found := index.plans[node]; found {
		return plan
	}

	var plan *templatePlan

	if node.IsLeaf() {
		plan = compileTemplateLeaf(node)
	} else {
		plan = index.compileChildren(node, retain)
	}

	if retain {
		index.plans[node] = plan
	}

	return plan
}

func (index *templateIndex) compileChildren(node *tree.Node, retain bool) *templatePlan {
	var plan *templatePlan

	for _, key := range node.ChildrenKeys() {
		childPlan := index.compile(node.Child(key), retain)

		var matches [][]int

		if !node.IsArray() {
			matches = templatePattern.FindAllStringSubmatchIndex(key, -1)
		}

		if childPlan != nil || len(matches) > 0 {
			if plan == nil {
				plan = &templatePlan{matches: nil, children: nil}
			}

			plan.children = append(plan.children, templateChild{key: key, matches: matches, plan: childPlan})
		}
	}

	return plan
}

func compileTemplateLeaf(node *tree.Node) *templatePlan {
	value := reflect.ValueOf(node.Value)
	if value.Kind() != reflect.String {
		return nil
	}

	matches := templatePattern.FindAllStringSubmatchIndex(value.String(), -1)
	if len(matches) == 0 {
		return nil
	}

	return &templatePlan{matches: matches, children: nil}
}

// expandPlannedTemplates returns the original subtree unless expansion changes
// it. Parents and YAML scalar annotations detach before the first write.
func expandPlannedTemplates(
	node *tree.Node, plan *templatePlan, vars map[string]string,
) *tree.Node {
	if plan == nil {
		return node
	}

	if node.IsLeaf() {
		return expandPlannedLeaf(node, plan, vars)
	}

	result := node
	renamed := make(map[string]string)
	changedChildren := make(map[string]*tree.Node)

	for _, entry := range plan.children {
		name := expandCompiledTemplate(entry.key, entry.matches, vars)

		original := node.Child(entry.key)

		child := expandPlannedTemplates(original, entry.plan, vars)

		if name != entry.key {
			if child == original {
				child = shallowCloneNode(child)
			}

			annotation := yamlAnnotation(child)
			if annotation.Key != nil {
				annotation.Key = cloneScalarYAMLNode(annotation.Key)
				annotation.Key.Tag, annotation.Key.Value = yamlStringTag, name
				child.SetAnnotation(annotation)
			}

			renamed[entry.key] = name
		}

		if child != original {
			changedChildren[entry.key] = child
		}
	}

	if len(renamed) == 0 && len(changedChildren) == 0 {
		return result
	}

	return rebuildExpandedChildren(node, renamed, changedChildren)
}

func rebuildExpandedChildren(node *tree.Node, renamed map[string]string, changed map[string]*tree.Node) *tree.Node {
	result := shallowCloneNode(node)

	if len(renamed) == 0 {
		for key, child := range changed {
			result.SetChild(key, child)
		}

		return result
	}

	result.ClearChildren()

	for _, key := range node.ChildrenKeys() {
		child := node.Child(key)
		if replacement, found := changed[key]; found {
			child = replacement
		}

		name := key
		if replacement, found := renamed[key]; found {
			name = replacement
		}

		result.SetChild(name, child)
	}

	result.SetOrderSet(node.OrderSet())

	return result
}

func expandPlannedLeaf(
	node *tree.Node, plan *templatePlan, vars map[string]string,
) *tree.Node {
	value := reflect.ValueOf(node.Value)
	text := expandCompiledTemplate(value.String(), plan.matches, vars)

	if text == value.String() {
		return node
	}

	replacement := reflect.New(value.Type()).Elem()
	replacement.SetString(text)

	result := shallowCloneNode(node)
	setExpandedLeaf(result, replacement)

	return result
}

func setExpandedLeaf(node *tree.Node, value reflect.Value) {
	node.Value = value.Interface()
	node.SetTypeFixed(true)

	annotation := yamlAnnotation(node)
	if annotation.Val != nil {
		annotation.Val = cloneScalarYAMLNode(annotation.Val)
		annotation.Val.Tag = yamlStringTag
		annotation.Val.Value = value.String()
		node.SetAnnotation(annotation)
	}
}
