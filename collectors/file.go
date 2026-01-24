package collectors

import (
	"context"
	"os"
	"path/filepath"

	"github.com/tarantool/go-config"
)

// FileCollector represents configuration data from io.Reader.
type FileCollector struct {
	file string
	yc   YamlCollector
}

// NewFileCollector creates new FileCollector from the given file name.
// The source type defaults to config.FileSource.
func NewFileCollector(file string) *FileCollector {
	data, err := os.ReadFile(filepath.Clean(file))
	if err != nil {
		return nil
	}

	return &FileCollector{
		file: file,
		yc: YamlCollector{
			name:       "file",
			sourceType: config.FileSource,
			revision:   "",
			keepOrder:  true,
			data:       data,
		},
	}
}

// WithName sets a custom name for the collector.
func (fc *FileCollector) WithName(name string) *FileCollector {
	fc.yc.name = name
	return fc
}

// WithSourceType sets the source type for the collector.
func (fc *FileCollector) WithSourceType(source config.SourceType) *FileCollector {
	fc.yc.sourceType = source
	return fc
}

// WithRevision sets the revision for the collector.
func (fc *FileCollector) WithRevision(rev config.RevisionType) *FileCollector {
	fc.yc.revision = rev
	return fc
}

// WithKeepOrder sets whether the collector preserves key order.
func (fc *FileCollector) WithKeepOrder(keep bool) *FileCollector {
	fc.yc.keepOrder = keep
	return fc
}

// Name implements the Collector interface.
func (fc *FileCollector) Name() string {
	return fc.yc.name
}

// Source implements the Collector interface.
func (fc *FileCollector) Source() config.SourceType {
	return fc.yc.sourceType
}

// Revision implements the Collector interface.
func (fc *FileCollector) Revision() config.RevisionType {
	return fc.yc.revision
}

// KeepOrder implements the Collector interface.
func (fc *FileCollector) KeepOrder() bool {
	return fc.yc.keepOrder
}

// Read implements the Collector interface.
func (fc *FileCollector) Read(ctx context.Context) <-chan config.Value {
	return fc.yc.Read(ctx)
}
