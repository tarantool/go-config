package config

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/tarantool/go-config/v2/tree"
)

// Tarantool permits ASCII spaces around names; tabs and newlines are part of
// the variable name. Values substituted into a string are not expanded again.
var templatePattern = regexp.MustCompile(`(?s)\{\{ *(.*?) *\}\}`)

func expandTemplate(text string, vars map[string]string, path KeyPath) (string, error) {
	matches := templatePattern.FindAllStringSubmatchIndex(text, -1)
	if len(matches) == 0 {
		return text, nil
	}

	var result strings.Builder

	result.Grow(len(text))

	end := 0

	for _, match := range matches {
		const nameStart, nameEnd = 2, 3

		name := text[match[nameStart]:match[nameEnd]]
		value, ok := vars[name]

		if !ok {
			return "", fmt.Errorf("%s: %w %q", path, ErrUnknownTemplateVariable, name)
		}

		result.WriteString(text[end:match[0]])
		result.WriteString(value)

		end = match[1]
	}

	result.WriteString(text[end:])

	return result.String(), nil
}

// expandTemplates only mutates the freshly resolved tree. Leaf containers and
// YAML annotations are copied before changes so the raw config remains intact.
func expandTemplates(node *tree.Node, path KeyPath, vars map[string]string) error {
	if node.IsLeaf() {
		return expandTemplateLeaf(node, path, vars)
	}

	keys := node.ChildrenKeys()
	children := node.Children()
	renamed := false

	for i, key := range keys {
		childPath := path.Append(key)
		if !node.IsArray() {
			name, err := expandTemplate(key, vars, childPath)
			if err != nil {
				return err
			}

			if name != key {
				keys[i] = name
				renamed = true

				annotation := yamlAnnotation(children[i])
				if annotation.Key != nil {
					annotation.Key = cloneScalarYAMLNode(annotation.Key)
					annotation.Key.Tag = yamlStringTag
					annotation.Key.Value = name
					children[i].SetAnnotation(annotation)
				}
			}
		}

		err := expandTemplates(children[i], childPath, vars)
		if err != nil {
			return err
		}
	}

	if renamed {
		ordered := node.OrderSet()

		node.ClearChildren()

		for i, key := range keys {
			node.SetChild(key, children[i])
		}

		node.SetOrderSet(ordered)
	}

	return nil
}

func expandTemplateLeaf(node *tree.Node, path KeyPath, vars map[string]string) error {
	value, changed, err := expandTemplateValue(reflect.ValueOf(node.Value), path, vars)
	if err != nil {
		return err
	}

	if !changed {
		return nil
	}

	node.Value = value.Interface()
	if value.Kind() != reflect.String {
		return nil
	}

	node.SetTypeFixed(true)

	annotation := yamlAnnotation(node)
	if annotation.Val != nil {
		annotation.Val = cloneScalarYAMLNode(annotation.Val)
		annotation.Val.Tag = yamlStringTag
		annotation.Val.Value = value.String()
		node.SetAnnotation(annotation)
	}

	return nil
}

func expandTemplateValue(value reflect.Value, path KeyPath, vars map[string]string) (reflect.Value, bool, error) {
	if !value.IsValid() {
		return value, false, nil
	}

	switch value.Kind() { //nolint:exhaustive // Only strings and containers can contain templates.
	case reflect.String:
		text, err := expandTemplate(value.String(), vars, path)
		if err != nil {
			return value, false, err
		}

		if text == value.String() {
			return value, false, nil
		}

		result := reflect.New(value.Type()).Elem()
		result.SetString(text)

		return result, true, nil
	case reflect.Interface:
		if value.IsNil() {
			return value, false, nil
		}

		result, changed, err := expandTemplateValue(value.Elem(), path, vars)
		if err != nil || !changed {
			return value, false, err
		}

		wrapped := reflect.New(value.Type()).Elem()
		wrapped.Set(result)

		return wrapped, true, nil
	case reflect.Map:
		if value.IsNil() {
			return value, false, nil
		}

		result := reflect.MakeMapWithSize(value.Type(), value.Len())
		changed := false
		iter := value.MapRange()

		for iter.Next() {
			key := iter.Key()
			childPath := path.Append(fmt.Sprint(key.Interface()))

			newKey, keyChanged, err := expandTemplateValue(key, childPath, vars)
			if err != nil {
				return value, false, err
			}

			item, itemChanged, err := expandTemplateValue(iter.Value(), childPath, vars)
			if err != nil {
				return value, false, err
			}

			result.SetMapIndex(newKey, item)

			changed = changed || keyChanged || itemChanged
		}

		return result, changed, nil
	case reflect.Slice, reflect.Array:
		var result reflect.Value

		if value.Kind() == reflect.Slice {
			if value.IsNil() {
				return value, false, nil
			}

			result = reflect.MakeSlice(value.Type(), value.Len(), value.Len())
		} else {
			result = reflect.New(value.Type()).Elem()
		}

		changed := false

		for i := range value.Len() {
			item, itemChanged, err := expandTemplateValue(value.Index(i), path.Append(strconv.Itoa(i)), vars)
			if err != nil {
				return value, false, err
			}

			result.Index(i).Set(item)

			changed = changed || itemChanged
		}

		return result, changed, nil
	default:
		return value, false, nil
	}
}
