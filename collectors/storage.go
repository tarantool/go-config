package collectors

import (
	"bytes"
	"context"
	"strconv"
	"strings"

	"github.com/tarantool/go-config"
	"github.com/tarantool/go-config/storage"
	"github.com/tarantool/go-config/tree"
)

// Storage implements config.Collector for reading multiple configuration
// documents from a key-value storage under a common prefix. Each key's value
// is parsed according to the given Format and merged into a single config tree.
// Keys are interpreted as paths, with a configurable delimiter (default "/").
type Storage struct {
	name       string
	sourceType config.SourceType
	revision   config.RevisionType
	keepOrder  bool
	storage    storage.Storage
	format     Format
	prefix     string
	delimiter  string
}

// NewStorage creates a new Storage collector that will read all keys with the
// given prefix from the storage backend. Each key's value is parsed using the
// provided Format. The prefix is removed from the key to form the config path.
func NewStorage(strg storage.Storage, prefix string, format Format) *Storage {
	return &Storage{
		name:       "storage",
		sourceType: config.StorageSource,
		revision:   "",
		keepOrder:  false,
		storage:    strg,
		format:     format,
		prefix:     prefix,
		delimiter:  "/",
	}
}

// WithName sets a custom name for the collector (default "storage").
func (s *Storage) WithName(name string) *Storage {
	s.name = name
	return s
}

// WithSourceType sets the source type reported by the collector
// (default config.StorageSource).
func (s *Storage) WithSourceType(source config.SourceType) *Storage {
	s.sourceType = source
	return s
}

// WithRevision sets an initial revision for the collector.
// If not set, the revision will be derived from the highest ModRevision
// among the fetched keys after a successful Read.
func (s *Storage) WithRevision(rev config.RevisionType) *Storage {
	s.revision = rev
	return s
}

// WithKeepOrder sets whether the collector should preserve the order
// of keys as they appear in the storage range (default false).
func (s *Storage) WithKeepOrder(keep bool) *Storage {
	s.keepOrder = keep
	return s
}

// WithDelimiter sets the delimiter used to split storage keys into
// config path segments. The default is "/". If the delimiter differs
// from "/", it is replaced internally with "/" before constructing
// the KeyPath.
func (s *Storage) WithDelimiter(delim string) *Storage {
	s.delimiter = delim
	return s
}

// Name returns the collector's name.
func (s *Storage) Name() string {
	return s.name
}

// Source returns the collector's source type.
func (s *Storage) Source() config.SourceType {
	return s.sourceType
}

// Revision returns the collector's current revision.
func (s *Storage) Revision() config.RevisionType {
	return s.revision
}

// KeepOrder returns whether the collector preserves key order.
func (s *Storage) KeepOrder() bool {
	return s.keepOrder
}

// Read performs a range query with the configured prefix, parses each
// key's value using the collector's Format, merges the resulting subtrees,
// and emits config.Value entries on the returned channel. The collector's
// revision is updated to the maximum ModRevision among the fetched keys.
// If an error occurs during the range query, the channel is closed without
// emitting any values. Keys with empty values or parsing errors are skipped.
func (s *Storage) Read(ctx context.Context) <-chan config.Value {
	valueChan := make(chan config.Value)

	go func() {
		defer close(valueChan)

		kvs, err := s.storage.Range(ctx, storage.WithPrefix([]byte(s.prefix)))
		if err != nil {
			return
		}

		if len(kvs) == 0 {
			return
		}

		root := tree.New()

		var maxRev int64

		for _, keyValue := range kvs {
			relKey := strings.TrimPrefix(string(keyValue.Key), s.prefix)
			if s.delimiter != "/" && strings.Contains(relKey, s.delimiter) {
				relKey = strings.ReplaceAll(relKey, s.delimiter, "/")
			}

			path := config.NewKeyPath(relKey)

			if len(keyValue.Value) == 0 {
				if keyValue.ModRevision > maxRev {
					maxRev = keyValue.ModRevision
				}

				continue
			}

			format := s.format.From(bytes.NewReader(keyValue.Value))

			subtree, parseErr := format.Parse()
			if parseErr != nil {
				if keyValue.ModRevision > maxRev {
					maxRev = keyValue.ModRevision
				}

				continue
			}

			mergeSubtree(root, path, subtree)

			if keyValue.ModRevision > maxRev {
				maxRev = keyValue.ModRevision
			}
		}

		s.revision = config.RevisionType(strconv.FormatInt(maxRev, 10))

		walkTree(ctx, root, config.NewKeyPath(""), valueChan)
	}()

	return valueChan
}

// mergeSubtree recursively merges subtree into root at the given path.
// Leaf values are set with root.Set, and their Source and Revision are
// copied from the subtree node if present.
func mergeSubtree(root *tree.Node, path config.KeyPath, subtree *tree.Node) {
	if subtree.IsLeaf() {
		root.Set(path, subtree.Value)

		if node := root.Get(path); node != nil {
			node.Source = subtree.Source
			node.Revision = subtree.Revision
		}

		return
	}

	for _, key := range subtree.ChildrenKeys() {
		child := subtree.Child(key)
		if child == nil {
			continue
		}

		childPath := path.Append(key)
		if child.IsLeaf() {
			root.Set(childPath, child.Value)

			if node := root.Get(childPath); node != nil {
				node.Source = child.Source
				node.Revision = child.Revision
			}
		} else {
			mergeSubtree(root, childPath, child)
		}
	}
}
