// Package envpath builds a trie of property paths from a Tarantool JSON
// schema and resolves stripped environment-variable keys against it via
// greedy longest-prefix matching.
package envpath

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tarantool/go-config"
)

// Node is a trie node keyed by lowercased schema segment names.
// wildcard is set when the schema permits arbitrary names at this level.
type Node struct {
	children map[string]*Node
	wildcard *Node
}

// Build parses a JSON schema and returns a trie of property paths.
func Build(schema []byte) (*Node, error) {
	var root map[string]any

	err := json.Unmarshal(schema, &root)
	if err != nil {
		return nil, fmt.Errorf("envpath: parse schema: %w", err)
	}

	defs := extractDefs(root)
	node := newNode()

	walkSchemaNode(node, root, defs, map[string]bool{})

	return node, nil
}

func newNode() *Node {
	return &Node{
		children: map[string]*Node{},
		wildcard: nil,
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
	var marked []string

	defer func() {
		for _, name := range marked {
			delete(visited, name)
		}
	}()

	for {
		ref, hasRef := node["$ref"].(string)
		if !hasRef {
			break
		}

		name, ok := localRefName(ref)
		if !ok || visited[name] {
			return
		}

		target, ok := defs[name].(map[string]any)
		if !ok {
			return
		}

		visited[name] = true

		marked = append(marked, name)
		node = target
	}

	props, ok := node["properties"].(map[string]any)
	if ok {
		for name, raw := range props {
			child, isObj := raw.(map[string]any)
			if !isObj {
				continue
			}

			childNode := newNode()

			parent.children[strings.ToLower(name)] = childNode

			walkSchemaNode(childNode, child, defs, visited)
		}
	}

	if hasWildcardChildren(node) {
		parent.wildcard = newNode()

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
// longest-prefix matching against the schema trie. Returns nil on no match.
func (n *Node) Resolve(key string) config.KeyPath {
	if key == "" {
		return nil
	}

	parts := strings.Split(strings.ToLower(key), "_")

	tokens := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			tokens = append(tokens, p)
		}
	}

	if len(tokens) == 0 {
		return nil
	}

	return resolveTokens(n, tokens)
}

func resolveTokens(node *Node, tokens []string) config.KeyPath {
	if len(tokens) == 0 {
		return config.KeyPath{}
	}

	for take := len(tokens); take >= 1; take-- {
		segment := strings.Join(tokens[:take], "_")

		child, ok := node.children[segment]
		if !ok {
			continue
		}

		tail := resolveTokens(child, tokens[take:])
		if tail == nil {
			continue
		}

		return append(config.KeyPath{segment}, tail...)
	}

	if node.wildcard != nil {
		tail := resolveTokens(node.wildcard, tokens[1:])
		if tail == nil {
			return nil
		}

		return append(config.KeyPath{tokens[0]}, tail...)
	}

	return nil
}
