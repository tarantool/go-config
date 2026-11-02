package tarantool

import (
	"context"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/tarantool/go-config/v2"
	"github.com/tarantool/go-config/v2/internal/environ"
	"github.com/tarantool/go-config/v2/tarantool/internal/envpath"
	"github.com/tarantool/go-config/v2/tree"
)

// schemaEnv collects the TT_* (or TT_*_DEFAULT) environment variables the
// way Tarantool's env config source does: the name of a variable is resolved
// to a config path through the schema, and its value is read as the type the
// schema gives that path ([envpath.Type.Parse]), so TT_CONFIG_ETCD_ENDPOINTS
// holds an array and TT_CONSOLE_ENABLED a boolean, the same as in a file.
//
// A value comes out the way the YAML collector delivers a file: an array or
// a map becomes a subtree of nodes rather than one opaque leaf, so Get,
// Effective, merge and validation treat it as a file's, and an array
// replaces an array from a lower-priority source wholesale. A string the
// schema made a string, JSON strings included, has a fixed type
// ([tree.Node.TypeFixed]), so "123" in a string field stays a string through
// validation.
//
// An empty value is unset, as in Tarantool, and so is JSON null where JSON is
// read. A value that cannot be read as its type stays the raw string, with no
// fixed type: Tarantool refuses to start on it, but readers of a config such
// as tt need it read best-effort, and validation reports the type error.
//
// Unlike Tarantool, which reads only the variables of the instance config,
// paths under groups, replicasets and instances resolve too (see
// [envpath.Node.Resolve]). Overlapping variables are applied in the order of
// their names, so TT_LABELS_ROLE=x lands inside TT_LABELS='a=b' rather than
// being replaced by it.
type schemaEnv struct {
	name       string
	sourceType config.SourceType
	prefix     string
	ignore     []string
	// selectKey picks the variables of this collector by the name with the
	// prefix stripped and returns what of it names the config path.
	selectKey func(key string) (string, bool)
	resolver  *envpath.Node
}

// Read implements [config.Collector].
func (e *schemaEnv) Read(ctx context.Context) <-chan config.Value {
	valueCh := make(chan config.Value)

	go func() {
		defer close(valueCh)

		root := e.collect()

		// Emitting an empty root would overwrite every lower-priority value.
		if root.IsLeaf() {
			return
		}

		walkLeaves(ctx, []*tree.Node{root}, config.NewKeyPath(""), valueCh)
	}()

	return valueCh
}

// Name implements [config.Collector].
func (e *schemaEnv) Name() string {
	return e.name
}

// Source implements [config.Collector].
func (e *schemaEnv) Source() config.SourceType {
	return e.sourceType
}

// Revision implements [config.Collector].
func (e *schemaEnv) Revision() config.RevisionType {
	return ""
}

// KeepOrder implements [config.Collector].
func (e *schemaEnv) KeepOrder() bool {
	return false
}

// collect builds the tree of the variables of this collector.
func (e *schemaEnv) collect() *tree.Node {
	vars := maps.Collect(environ.ParseAll())
	root := tree.New()

	for _, name := range slices.Sorted(maps.Keys(vars)) {
		key, ok := strings.CutPrefix(name, e.prefix)
		if !ok || envIgnored(name, e.ignore) {
			continue
		}

		key, ok = e.selectKey(key)
		if !ok {
			continue
		}

		path, typ := e.resolver.Resolve(key)
		if path == nil {
			continue
		}

		raw := vars[name]

		value, parsed := typ.Parse(raw)
		if !parsed {
			graft(root, path, valueNode(raw, false))

			continue
		}

		if value != nil {
			graft(root, path, valueNode(value, true))
		}
	}

	return root
}

// valueNode builds the subtree of a value read from a variable: a map becomes
// a node with a child per key, in key order, and an array a node marked as
// one with a child per item, as the YAML collector builds them. An empty map
// or array is a leaf holding it, as it is for YAML. fixed marks the strings in
// it as having a fixed type.
func valueNode(value any, fixed bool) *tree.Node {
	node := tree.New()

	switch typed := value.(type) {
	case map[string]any:
		if len(typed) == 0 {
			node.Value = typed

			return node
		}

		for _, key := range slices.Sorted(maps.Keys(typed)) {
			node.SetChild(key, valueNode(typed[key], fixed))
		}
	case []any:
		node.MarkArray()

		if len(typed) == 0 {
			node.Value = typed

			return node
		}

		for i, item := range typed {
			node.SetChild(strconv.Itoa(i), valueNode(item, fixed))
		}
	case string:
		node.Value = typed
		node.SetTypeFixed(fixed)
	default:
		node.Value = value
	}

	return node
}

// graft places sub at path below root. A map lands in a map already there key
// by key; anything else replaces what is there. A leaf on the way down turns
// into a map, as it does when values merge.
func graft(root *tree.Node, path config.KeyPath, sub *tree.Node) {
	parent := root

	for _, segment := range path[:len(path)-1] {
		child := parent.Child(segment)
		if child == nil || child.IsArray() || child.IsLeaf() {
			child = tree.New()
			parent.SetChild(segment, child)
		}

		parent = child
	}

	key := path[len(path)-1]

	existing := parent.Child(key)
	if !isMapNode(existing) || !isMapNode(sub) {
		parent.SetChild(key, sub)

		return
	}

	for _, childKey := range sub.ChildrenKeys() {
		graft(existing, config.NewKeyPathFromSegments([]string{childKey}), sub.Child(childKey))
	}
}

// isMapNode reports whether node holds a map: it has children and is not an
// array.
func isMapNode(node *tree.Node) bool {
	return node != nil && !node.IsLeaf() && !node.IsArray()
}

// walkLeaves sends every leaf under the last node of path, which holds the
// nodes from the root down to it.
func walkLeaves(ctx context.Context, path []*tree.Node, prefix config.KeyPath, valueCh chan<- config.Value) {
	node := path[len(path)-1]

	if node.IsLeaf() {
		select {
		case <-ctx.Done():
		case valueCh <- tree.NewValueOnPath(path, prefix):
		}

		return
	}

	for _, key := range node.ChildrenKeys() {
		walkLeaves(ctx, append(slices.Clip(path), node.Child(key)), prefix.Append(key), valueCh)
	}
}
