package config

import (
	"maps"
	"reflect"
	"slices"
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

	end := 0

	for _, match := range matches {
		name := text[match.nameStart:match.nameEnd]
		value, ok := vars[name]

		if !ok || value == text[match.start:match.end] {
			continue
		}

		// Allocate only for the first change; skipped placeholders remain
		// part of the unchanged span copied from the source string.
		if end == 0 {
			result.Grow(len(text))
		}

		result.WriteString(text[end:match.start])
		result.WriteString(value)

		end = match.end
	}

	if end == 0 {
		return text
	}

	result.WriteString(text[end:])

	return result.String()
}

// templateIndex belongs to one published configuration version. Only source
// nodes are retained; merged effective nodes get plans local to that call.
// Partial source summaries retain no plans for excluded children.
// Only string leaves and mapping keys are inspected. Containers stored as leaf
// values are left unchanged. No template state is attached to public tree nodes.
type templateIndex struct {
	mu    sync.RWMutex
	plans map[*tree.Node]*sourceTemplatePlan
}

// A source summary is immutable once published. Pending keys were excluded
// from the effective tree and have not been inspected yet. A nil plan with no
// pending keys records a template-free subtree in one entry.
type sourceTemplatePlan struct {
	plan    *templatePlan
	pending map[string]struct{}
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

func newTemplateIndex() *templateIndex {
	return &templateIndex{mu: sync.RWMutex{}, plans: nil}
}

// planFor builds a plan only after inheritance has selected the final values.
// Detached nodes reuse a source summary and inspect only their changed keys.
func (index *templateIndex) planFor(root *tree.Node, temporary map[*tree.Node]*templateChanges) *templatePlan {
	if root == nil {
		return nil
	}

	changes, isTemporary := temporary[root]
	if !isTemporary {
		return index.sourcePlan(root, nil)
	}

	if root.IsLeaf() {
		return compileTemplateLeaf(root)
	}

	var plan *templatePlan

	if changes == nil {
		for _, key := range root.ChildrenKeys() {
			plan = appendTemplateChild(plan, root, key, index.planFor(root.Child(key), temporary))
		}

		return plan
	}

	base := index.sourcePlan(changes.source, changes.keys)
	if base != nil {
		for _, entry := range base.children {
			if _, changed := changes.keys[entry.key]; changed {
				continue
			}

			if plan == nil {
				plan = &templatePlan{matches: nil, children: nil}
			}

			plan.children = append(plan.children, entry)
		}
	}

	for key := range changes.keys {
		if child := root.Child(key); child != nil {
			plan = appendTemplateChild(plan, root, key, index.planFor(child, temporary))
		}
	}

	return plan
}

// sourcePlan caches one summary per requested source subtree, rather than a
// nil entry for every ordinary descendant. Excluded children stay pending, so
// pruning and overrides never trigger compilation of discarded values.
func (index *templateIndex) sourcePlan(node *tree.Node, excluded map[string]struct{}) *templatePlan {
	for {
		index.mu.RLock()

		cached := index.plans[node]
		index.mu.RUnlock()

		var keys []string

		if cached == nil {
			keys = node.ChildrenKeys()
		} else {
			for key := range cached.pending {
				if _, skip := excluded[key]; !skip {
					keys = append(keys, key)
				}
			}

			if len(keys) == 0 {
				return cached.plan
			}
		}

		compiled := compileSourceTemplates(node, cached, keys, excluded)

		index.mu.Lock()

		if index.plans[node] != cached {
			// Another reader published a summary while we compiled. Retry
			// using it; only its still-pending children may need work.
			index.mu.Unlock()
			continue
		}

		if index.plans == nil {
			index.plans = make(map[*tree.Node]*sourceTemplatePlan)
		}

		index.plans[node] = compiled
		index.mu.Unlock()

		return compiled.plan
	}
}

func compileSourceTemplates(
	node *tree.Node, cached *sourceTemplatePlan, keys []string, excluded map[string]struct{},
) *sourceTemplatePlan {
	compiled := &sourceTemplatePlan{plan: nil, pending: nil}

	if node.IsLeaf() {
		compiled.plan = compileTemplateLeaf(node)
		return compiled
	}

	if cached != nil {
		compiled.pending = maps.Clone(cached.pending)
		if cached.plan != nil {
			compiled.plan = &templatePlan{matches: nil, children: slices.Clone(cached.plan.children)}
		}
	}

	for _, key := range keys {
		if _, skip := excluded[key]; skip {
			if compiled.pending == nil {
				compiled.pending = make(map[string]struct{})
			}

			compiled.pending[key] = struct{}{}

			continue
		}

		childPlan := compileTemplateTree(node.Child(key))

		compiled.plan = appendTemplateChild(compiled.plan, node, key, childPlan)
		delete(compiled.pending, key)
	}

	if len(compiled.pending) == 0 {
		compiled.pending = nil
	}

	return compiled
}

// Descendant plans are stored inside their subtree's summary. Ordinary nodes
// do not need entries in the index or a lock acquisition of their own.
func compileTemplateTree(node *tree.Node) *templatePlan {
	if node == nil {
		return nil
	}

	if node.IsLeaf() {
		return compileTemplateLeaf(node)
	}

	var plan *templatePlan
	for _, key := range node.ChildrenKeys() {
		plan = appendTemplateChild(plan, node, key, compileTemplateTree(node.Child(key)))
	}

	return plan
}

// appendTemplateChild mutates only a plan owned by the current compilation.
func appendTemplateChild(plan *templatePlan, parent *tree.Node, key string, childPlan *templatePlan) *templatePlan {
	var matches []templateMatch
	if !parent.IsArray() {
		matches = compileTemplate(key)
	}

	if childPlan == nil && len(matches) == 0 {
		return plan
	}

	if plan == nil {
		plan = &templatePlan{matches: nil, children: nil}
	}

	plan.children = append(plan.children, templateChild{key: key, matches: matches, plan: childPlan})

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
				child = child.ShallowClone()
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
	result := node.ShallowClone()

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

	result := node.ShallowClone()
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
