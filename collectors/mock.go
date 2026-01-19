package collectors

import (
	"context"

	"github.com/tarantool/go-config"
	"github.com/tarantool/go-config/internal/tree"
)

// Mock is a testing collector that returns a predefined set of values.
type Mock struct {
	name       string
	sourceType config.SourceType
	revision   config.RevisionType
	keepOrder  bool
	entries    []mockEntry
}

type mockEntry struct {
	keyPath config.KeyPath
	value   any
}

// NewMock creates a Mock with default settings.
func NewMock() *Mock {
	return &Mock{
		name:       "mock",
		sourceType: config.UnknownSource,
		revision:   "",
		keepOrder:  false,
		entries:    nil,
	}
}

// WithName sets a custom name for the collector.
func (mc *Mock) WithName(name string) *Mock {
	mc.name = name
	return mc
}

// WithSourceType sets the source type for the collector.
func (mc *Mock) WithSourceType(source config.SourceType) *Mock {
	mc.sourceType = source
	return mc
}

// WithRevision sets the revision for the collector.
func (mc *Mock) WithRevision(rev config.RevisionType) *Mock {
	mc.revision = rev
	return mc
}

// WithKeepOrder sets whether the collector preserves key order.
func (mc *Mock) WithKeepOrder(keep bool) *Mock {
	mc.keepOrder = keep
	return mc
}

// WithEntry adds a key-value pair to the collector.
func (mc *Mock) WithEntry(keyPath config.KeyPath, value any) *Mock {
	mc.entries = append(mc.entries, mockEntry{keyPath, value})
	return mc
}

// WithEntries adds multiple key-value pairs to the collector.
func (mc *Mock) WithEntries(entries map[string]any) *Mock {
	for key, value := range entries {
		mc.entries = append(mc.entries, mockEntry{config.NewKeyPath(key), value})
	}

	return mc
}

// Read implements the Collector interface.
func (mc *Mock) Read(ctx context.Context) <-chan config.Value {
	valueCh := make(chan config.Value)

	go func() {
		defer close(valueCh)
		// Build a tree from entries.
		root := tree.New()
		for _, entry := range mc.entries {
			// Set source and revision on the node.
			node := root.Get(entry.keyPath)
			if node == nil {
				// Create path.
				root.Set(entry.keyPath, entry.value)

				node = root.Get(entry.keyPath)
			}

			if node != nil {
				// Override source and revision if they are set at collector level.
				// For simplicity, we set source and revision on the leaf node.
				// This assumes each entry is a leaf (no nested structures).
				node.Source = mc.name
				node.Revision = string(mc.revision)
			}
		}
		// Walk the tree and send leaf values.
		walkTree(ctx, root, config.NewKeyPath(""), valueCh)
	}()

	return valueCh
}

// Name implements the Collector interface.
func (mc *Mock) Name() string {
	return mc.name
}

// Source implements the Collector interface.
func (mc *Mock) Source() config.SourceType {
	return mc.sourceType
}

// Revision implements the Collector interface.
func (mc *Mock) Revision() config.RevisionType {
	return mc.revision
}

// KeepOrder implements the Collector interface.
func (mc *Mock) KeepOrder() bool {
	return mc.keepOrder
}
