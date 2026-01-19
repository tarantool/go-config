package config

import (
	"context"
	"fmt"
	"sync"

	"github.com/tarantool/go-config/internal/tree"
)

// Config provides access to the final configuration data.
type Config struct {
	// root is the internal tree representation of the configuration.
	root *tree.Node
}

// newConfig creates a Config from a tree node.
func newConfig(root *tree.Node) Config {
	return Config{root: root}
}

// Get is the primary, most convenient method for retrieving a value.
// It finds the value at the specified path and extracts it into the variable `dest`.
// Returns metadata and an error if the key is not found or the type cannot be converted.
func (c *Config) Get(path KeyPath, dest any) (MetaInfo, error) {
	val, ok := c.Lookup(path)
	if !ok {
		return MetaInfo{}, fmt.Errorf("%w: %s", ErrKeyNotFound, path)
	}

	err := val.Get(dest)
	if err != nil {
		return val.Meta(), err
	}

	return val.Meta(), nil
}

// Lookup searches for a value by key. Unlike Get, it does not
// return an error if the key is not found, but reports it via a boolean flag.
// Returns a special `Value` object and a flag indicating whether the value was found.
// This is useful when you need to distinguish between a missing value and a nil value.
func (c *Config) Lookup(path KeyPath) (Value, bool) {
	node := c.root.Get(path)
	if node == nil {
		return nil, false
	}

	return tree.NewValue(node, path), true
}

// Stat returns metadata for a key (source name, revision)
// without touching the actual value. Useful for debugging and introspection tools.
func (c *Config) Stat(path KeyPath) (MetaInfo, bool) {
	node := c.root.Get(path)
	if node == nil {
		return MetaInfo{Key: nil, Source: SourceInfo{Name: "", Type: UnknownSource}, Revision: ""}, false
	}
	// Create a temporary value to extract metadata.
	val := tree.NewValue(node, path)

	return val.Meta(), true
}

// Walk returns a channel through which you can iterate over all keys and values in the configuration.
// This is useful for traversing all parameters without needing to know their keys in advance.
// path may be empty (or `nil`) to start from the root of the configuration.
// If depth > 0, only the part of the configuration tree limited by the specified depth is traversed.
// If depth <= 0, the entire object is traversed.
func (c *Config) Walk(ctx context.Context, path KeyPath, depth int) (<-chan Value, error) {
	start := c.root
	if len(path) > 0 {
		start = c.root.Get(path)
		if start == nil {
			return nil, fmt.Errorf("%w: %s", ErrPathNotFound, path)
		}
	}

	valueCh := make(chan Value)

	go func() {
		defer close(valueCh)

		walkNodes(ctx, start, path, depth, valueCh)
	}()

	return valueCh, nil
}

// walkNodes recursively sends values for leaf nodes.
func walkNodes(ctx context.Context, node *tree.Node, prefix KeyPath, depth int, valueCh chan<- Value) {
	if depth == 0 {
		return
	}
	// If node is leaf, send its value.
	if node.IsLeaf() {
		select {
		case <-ctx.Done():
			return
		case valueCh <- tree.NewValue(node, prefix):
		}

		return
	}
	// Otherwise, recurse into children.
	for _, key := range node.ChildrenKeys() {
		child := node.Child(key)
		if child == nil {
			continue
		}

		walkNodes(ctx, child, prefix.Append(key), depth-1, valueCh)
	}
}

// Slice returns a slice of the original config that corresponds to the specified path.
// Used to obtain a sub-configuration as a separate Config object.
// If the path does not correspond to an object, returns an error.
// If path is empty (or `nil`), returns a copy of the current Config object.
func (c *Config) Slice(path KeyPath) (Config, error) {
	if len(path) == 0 {
		return newConfig(c.root), nil
	}

	root := c.root.Get(path)
	if root == nil {
		return Config{}, fmt.Errorf("%w: %s", ErrPathNotFound, path)
	}

	return newConfig(root), nil
}

// String returns a string with the current representation of the configuration according to the YAML format.
func (c *Config) String() string {
	//nolint:godox
	// TODO: implement YAML serialization.
	return ""
}

// MarshalYAML serializes the Config object into YAML format.
// Thanks to key order preservation, the resulting YAML will have a predictable and stable structure.
func (c *Config) MarshalYAML() ([]byte, error) {
	//nolint:godox
	// TODO: implement YAML marshaling.
	return nil, nil
}

// MutableConfig is an extension of Config that allows safe runtime modifications.
type MutableConfig struct {
	Config // Embeds the read-only interface.

	// mu provides synchronization for thread-safe configuration changes.
	mu sync.RWMutex
}

// Set sets or overwrites a value at the specified path.
// The key's metadata must be updated: Source becomes 'ModifiedSource', and Revision is incremented.
func (mc *MutableConfig) Set(_ KeyPath, _ any) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	//nolint:godox
	// TODO: not implemented.
	return nil
}

// Merge merges two configurations so that all values from the new configuration
// are added or override similar values in the current one.
// An error may occur if the new values do not conform to the current schema.
func (mc *MutableConfig) Merge(_ *Config) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	//nolint:godox
	// TODO: not implemented.
	return nil
}

// Update merges two configurations, but applies only those values that already exist
// in the current config. Everything else is ignored.
// An error may occur if the new values do not match the type of the current value according to the schema.
func (mc *MutableConfig) Update(_ *Config) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	//nolint:godox
	// TODO: not implemented.
	return nil
}

// Delete removes a key from the configuration.
func (mc *MutableConfig) Delete(_ KeyPath) bool {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	//nolint:godox
	// TODO: not implemented.
	return false
}
