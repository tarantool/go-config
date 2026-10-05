package config

import (
	"reflect"
	"strings"
	"sync"

	"github.com/tarantool/go-config/v2/tree"
)

type templateMatch struct {
	start, end         int
	nameStart, nameEnd int
}

// Tarantool permits ASCII spaces around names; tabs and newlines are part of
// the variable name. Values substituted into a string are not expanded again.
func compileTemplate(text string) []templateMatch {
	var matches []templateMatch

	for offset := 0; offset < len(text); {
		start := strings.Index(text[offset:], "{{")
		if start < 0 {
			break
		}

		start += offset

		nameStart := start + len("{{")

		end := strings.Index(text[nameStart:], "}}")
		if end < 0 {
			break
		}

		end += nameStart

		nameEnd := end
		for nameStart < nameEnd && text[nameStart] == ' ' {
			nameStart++
		}

		for nameEnd > nameStart && text[nameEnd-1] == ' ' {
			nameEnd--
		}

		matches = append(matches, templateMatch{
			start: start, end: end + len("}}"), nameStart: nameStart, nameEnd: nameEnd,
		})
		offset = end + len("}}")
	}

	return matches
}

func expandCompiledTemplate(text string, matches []templateMatch, vars map[string]string) string {
	if len(matches) == 0 {
		return text
	}

	var result strings.Builder

	result.Grow(len(text))

	end := 0

	for _, match := range matches {
		name := text[match.nameStart:match.nameEnd]
		value, ok := vars[name]

		result.WriteString(text[end:match.start])

		if !ok {
			result.WriteString(text[match.start:match.end])
		} else {
			result.WriteString(value)
		}

		end = match.end
	}

	result.WriteString(text[end:])

	return result.String()
}

// templateIndex belongs to one published configuration version. Only source
// nodes are retained; merged effective nodes get plans local to that call.
// Only string leaves and mapping keys are inspected. Containers stored as leaf
// values are left unchanged. No template state is attached to public tree nodes.
type templateIndex struct {
	mu    sync.RWMutex
	plans map[*tree.Node]*templatePlan
}
type templatePlan struct {
	matches  []templateMatch
	children []templateChild
}
type templateChild struct {
	key     string
	matches []templateMatch
	plan    *templatePlan
}

// prepare requires a read lock, which it temporarily releases on a cache miss.
func (index *templateIndex) prepare(node *tree.Node) {
	if _, found := index.plans[node]; found {
		return
	}

	index.mu.RUnlock()
	index.mu.Lock()
	defer func() {
		index.mu.Unlock()
		index.mu.RLock()
	}()

	if index.plans == nil {
		index.plans = make(map[*tree.Node]*templatePlan)
	}

	index.compile(node, true)
}

func newTemplateIndex() *templateIndex {
	return &templateIndex{mu: sync.RWMutex{}, plans: nil}
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

		var matches []templateMatch

		if !node.IsArray() {
			matches = compileTemplate(key)
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

	matches := compileTemplate(value.String())
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

	replacement := reflect.ValueOf(text)
	if replacement.Type() != value.Type() {
		replacement = replacement.Convert(value.Type())
	}

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
