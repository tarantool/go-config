package collectors

import (
	"context"
	"strings"

	"github.com/tarantool/go-config"
	"github.com/tarantool/go-config/internal/environ"
	"github.com/tarantool/go-config/tree"
)

// Env reads configuration data from environment variables.
type Env struct {
	name       string
	sourceType config.SourceType
	revision   config.RevisionType
	keepOrder  bool
	prefix     string
	delimiter  string                      // delimiter used by default transformation; defaults to underscore ('_').
	transform  func(string) config.KeyPath // custom key transformation.
}

// NewEnv creates an Env with default settings.
// By default, it uses underscore ('_') as delimiter, no prefix, and converts keys to lowercase,
// replacing underscores with slashes to form a hierarchical key path.
func NewEnv() *Env {
	collector := &Env{
		name:       "env",
		sourceType: config.EnvSource,
		revision:   "",
		keepOrder:  false,
		prefix:     "",
		delimiter:  "_",
		transform:  nil,
	}

	collector.transform = collector.defaultTransform

	return collector
}

// WithName sets a custom name for the collector.
func (ec *Env) WithName(name string) *Env {
	ec.name = name
	return ec
}

// WithSourceType sets the source type for the collector.
func (ec *Env) WithSourceType(source config.SourceType) *Env {
	ec.sourceType = source
	return ec
}

// WithRevision sets the revision for the collector.
func (ec *Env) WithRevision(rev config.RevisionType) *Env {
	ec.revision = rev
	return ec
}

// WithKeepOrder sets whether the collector preserves key order.
func (ec *Env) WithKeepOrder(keep bool) *Env {
	ec.keepOrder = keep
	return ec
}

// WithPrefix sets a prefix to strip from environment variable names.
// If set, only variables starting with this prefix are processed, and the prefix is removed.
func (ec *Env) WithPrefix(prefix string) *Env {
	ec.prefix = prefix
	return ec
}

// WithDelimiter sets the delimiter used by the default transformation to split environment variable names.
// The default is underscore ('_').
func (ec *Env) WithDelimiter(delim string) *Env {
	ec.delimiter = delim
	return ec
}

// WithTransform sets a custom transformation function from environment variable name to KeyPath.
// If set, prefix is still applied before transformation; delimiter is ignored.
func (ec *Env) WithTransform(fn func(string) config.KeyPath) *Env {
	if fn == nil {
		panic("transform function cannot be nil")
	}

	ec.transform = fn

	return ec
}

// Read implements the Collector interface.
func (ec *Env) Read(ctx context.Context) <-chan config.Value {
	valueCh := make(chan config.Value)

	go func() {
		defer close(valueCh)
		// Build a tree from environment variables.
		root := tree.New()

		for key, val := range environ.ParseAll() {
			key, ok := ec.stripPrefix(key)
			if !ok {
				continue // Prefix not matched, skip.
			}

			path := ec.transform(key)
			if len(path) == 0 {
				continue // Empty path after transformation, skip.
			}

			// Set leaf value in tree.
			root.Set(path, val)
		}

		// Walk the tree and send leaf values.
		walkTree(ctx, root, config.NewKeyPath(""), valueCh)
	}()

	return valueCh
}

// Name implements the Collector interface.
func (ec *Env) Name() string {
	return ec.name
}

// Source implements the Collector interface.
func (ec *Env) Source() config.SourceType {
	return ec.sourceType
}

// Revision implements the Collector interface.
func (ec *Env) Revision() config.RevisionType {
	return ec.revision
}

// KeepOrder implements the Collector interface.
func (ec *Env) KeepOrder() bool {
	return ec.keepOrder
}

// defaultTransform converts an environment variable key to a hierarchical key path.
// It lowercases the key, splits by the configured delimiter, removes empty parts, and joins with slash.
func (ec *Env) defaultTransform(key string) config.KeyPath {
	// Convert to lowercase.
	key = strings.ToLower(key)
	// Split by delimiter.
	parts := strings.Split(key, ec.delimiter)
	// Remove empty parts (if any).
	var filtered []string

	for _, p := range parts {
		if p != "" {
			filtered = append(filtered, p)
		}
	}

	return config.NewKeyPathWithDelim(strings.Join(filtered, "/"), "/")
}

// stripPrefix removes the configured prefix from the environment variable key.
// It returns the stripped key and true if the prefix matches, otherwise empty string and false.
func (ec *Env) stripPrefix(key string) (string, bool) {
	switch {
	case ec.prefix == "":
		return key, true
	case strings.HasPrefix(key, ec.prefix):
		return strings.TrimPrefix(key, ec.prefix), true
	default:
		return "", false
	}
}
