package collectors

import (
	"context"
)

// WatchEvent represents a change notification from storage.
type WatchEvent struct {
	// Prefix indicates the key or prefix that was changed.
	Prefix string
}

// Watcher provides reactive change notifications from a storage backend.
// Collectors that support watching for changes implement this interface
// in addition to the standard Collector interface.
type Watcher interface {
	// Watch returns a channel that streams change events for the collector's
	// key or prefix. The channel is closed when the context is cancelled.
	Watch(ctx context.Context) (<-chan WatchEvent, error)
}
