// Package collectors provides standard implementations of the Collector interface.
package collectors

import (
	"context"

	"github.com/tarantool/go-config"
)

// File reads configuration data from a File.
type File struct {
	name       string
	sourceType config.SourceType
	revision   config.RevisionType
	keepOrder  bool
	// file       *os.File
}

// NewFile creates a File with the given data.
// The source type defaults to config.UnknownSource.
func NewFile( /*file *os.File*/ ) *File {
	return &File{
		name:       "file",
		sourceType: config.FileSource,
		revision:   "",
		keepOrder:  true,
		// file:       file,
	}
}

// WithName sets a custom name for the collector.
func (fc *File) WithName(name string) *File {
	fc.name = name
	return fc
}

// WithSourceType sets the source type for the collector.
func (fc *File) WithSourceType(source config.SourceType) *File {
	fc.sourceType = source
	return fc
}

// WithRevision sets the revision for the collector.
func (fc *File) WithRevision(rev config.RevisionType) *File {
	fc.revision = rev
	return fc
}

// WithKeepOrder sets whether the collector preserves key order.
func (fc *File) WithKeepOrder(keep bool) *File {
	fc.keepOrder = keep
	return fc
}

// Read implements the Collector interface.
func (fc *File) Read(ctx context.Context) <-chan config.Value {
	valueCh := make(chan config.Value)

	return valueCh
}

// Name implements the Collector interface.
func (fc *File) Name() string {
	return fc.name
}

// Source implements the Collector interface.
func (fc *File) Source() config.SourceType {
	return fc.sourceType
}

// Revision implements the Collector interface.
func (fc *File) Revision() config.RevisionType {
	return fc.revision
}

// KeepOrder implements the Collector interface.
func (fc *File) KeepOrder() bool {
	return fc.keepOrder
}
