package config

import "context"

// Collector reads data from a source and streams it as a sequence of values.
type Collector interface {
	// Read returns a channel that streams values from the source.
	// The position of each element (its key) is contained in the Meta information of the Value.
	// The channel must be closed by the collector after all data has been sent.
	Read(ctx context.Context) <-chan Value

	// Name returns a human-readable name of the data source for logging and debugging.
	Name() string

	// Source returns the type of the data source (file, environment variable, etc.).
	Source() SourceType

	// Revision returns the revision identifier of the configuration.
	// For sources that do not support versioning, it should return an empty string.
	Revision() RevisionType

	// KeepOrder returns true if the order of keys must be preserved.
	// When true, the collector is considered as the "source of truth" for key order
	// at the corresponding nesting level during merging.
	KeepOrder() bool
}
