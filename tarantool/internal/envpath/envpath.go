// Package envpath builds a trie of property paths from a Tarantool JSON
// schema and resolves stripped environment-variable keys against it via
// greedy longest-prefix matching. A resolved key also yields the [Type] the
// schema gives its path, and [Type.Parse] reads a variable's value as that
// type the way Tarantool's config module does.
package envpath

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tarantool/go-config/v2"
)

// Node is a trie node keyed by lowercased schema segment names.
// wildcard is set when the schema permits arbitrary names at this level.
// typ is the type the schema gives the value at the node's path.
type Node struct {
	children map[string]*Node
	wildcard *Node
	typ      *Type
}

// Build parses a JSON schema and returns a trie of property paths.
func Build(schema []byte) (*Node, error) {
	var root map[string]any

	err := json.Unmarshal(schema, &root)
	if err != nil {
		return nil, fmt.Errorf("envpath: parse schema: %w", err)
	}

	defs := extractDefs(root)
	visited := map[string]bool{}
	node := newNode(typeOf(root, defs, visited))

	walkSchemaNode(node, root, defs, visited)

	return node, nil
}

func newNode(typ *Type) *Node {
	return &Node{
		children: map[string]*Node{},
		wildcard: nil,
		typ:      typ,
	}
}

func extractDefs(root map[string]any) map[string]any {
	for _, key := range []string{"$defs", "definitions"} {
		raw, ok := root[key].(map[string]any)
		if ok {
			return raw
		}
	}

	return map[string]any{}
}

// walkSchemaNode populates parent with children for every property reachable
// beneath node. visited guards against self-referential $refs.
func walkSchemaNode(parent *Node, node map[string]any, defs map[string]any, visited map[string]bool) {
	node, marked, ok := followRef(node, defs, visited)
	defer unmark(visited, marked)

	if !ok {
		return
	}

	props, ok := node["properties"].(map[string]any)
	if ok {
		for name, raw := range props {
			child, isObj := raw.(map[string]any)
			if !isObj {
				continue
			}

			childNode := newNode(typeOf(child, defs, visited))

			parent.children[strings.ToLower(name)] = childNode

			walkSchemaNode(childNode, child, defs, visited)
		}
	}

	if hasWildcardChildren(node) {
		parent.wildcard = newNode(freeValuesType(node, defs, visited))

		additional, isObj := node["additionalProperties"].(map[string]any)
		if isObj {
			walkSchemaNode(parent.wildcard, additional, defs, visited)
		}

		// Multiple patternProperties merge into the single wildcard node —
		// last writer wins on overlapping property names.
		patterns, isObj := node["patternProperties"].(map[string]any)
		if isObj {
			for _, raw := range patterns {
				schema, isObj := raw.(map[string]any)
				if !isObj {
					continue
				}

				walkSchemaNode(parent.wildcard, schema, defs, visited)
			}
		}
	}
}

// followRef follows the $ref chain of node to the schema it ends at. Every
// definition entered on the way is marked in visited and returned, for the
// caller to [unmark] once it is done with the schema. ok is false when a
// $ref is not local, names a missing definition, or re-enters a definition
// that is already being expanded.
func followRef(
	node map[string]any,
	defs map[string]any,
	visited map[string]bool,
) (map[string]any, []string, bool) {
	var marked []string

	for {
		ref, hasRef := node["$ref"].(string)
		if !hasRef {
			return node, marked, true
		}

		name, ok := localRefName(ref)
		if !ok || visited[name] {
			return nil, marked, false
		}

		target, ok := defs[name].(map[string]any)
		if !ok {
			return nil, marked, false
		}

		visited[name] = true

		marked = append(marked, name)
		node = target
	}
}

// unmark removes the definitions [followRef] marked from visited.
func unmark(visited map[string]bool, marked []string) {
	for _, name := range marked {
		delete(visited, name)
	}
}

func hasWildcardChildren(node map[string]any) bool {
	additional, present := node["additionalProperties"]
	if present {
		if b, isBool := additional.(bool); isBool && b {
			return true
		}

		if _, isObj := additional.(map[string]any); isObj {
			return true
		}
	}

	patterns, isObj := node["patternProperties"].(map[string]any)
	if isObj && len(patterns) > 0 {
		return true
	}

	return false
}

// localRefName extracts the definition name from a $ref into $defs or
// definitions; other $ref shapes return ok=false.
func localRefName(ref string) (string, bool) {
	name, ok := strings.CutPrefix(ref, "#/$defs/")
	if ok {
		return name, true
	}

	return strings.CutPrefix(ref, "#/definitions/")
}

// Resolve maps a stripped env-var key to a config.KeyPath via greedy
// longest-prefix matching against the schema trie, and returns the type the
// schema gives that path. A path through a wildcard level (a group, a
// replicaset or an instance name) has the type of the matching
// additionalProperties or patternProperties schema. Returns nil, nil on no
// match.
func (n *Node) Resolve(key string) (config.KeyPath, *Type) {
	if key == "" {
		return nil, nil
	}

	parts := strings.Split(strings.ToLower(key), "_")

	tokens := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			tokens = append(tokens, p)
		}
	}

	if len(tokens) == 0 {
		return nil, nil
	}

	path, node := resolveTokens(n, tokens)
	if node == nil {
		return nil, nil
	}

	return path, node.typ
}

// resolveTokens matches tokens against the trie below node and returns the
// path they spell together with the node it ends at, or nil, nil when they
// match no path.
func resolveTokens(node *Node, tokens []string) (config.KeyPath, *Node) {
	if len(tokens) == 0 {
		return config.KeyPath{}, node
	}

	for take := len(tokens); take >= 1; take-- {
		segment := strings.Join(tokens[:take], "_")

		child, ok := node.children[segment]
		if !ok {
			continue
		}

		tail, last := resolveTokens(child, tokens[take:])
		if last == nil {
			continue
		}

		return append(config.KeyPath{segment}, tail...), last
	}

	if node.wildcard != nil {
		tail, last := resolveTokens(node.wildcard, tokens[1:])
		if last == nil {
			return nil, nil
		}

		return append(config.KeyPath{tokens[0]}, tail...), last
	}

	return nil, nil
}
