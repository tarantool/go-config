package collectors

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tarantool/go-config"
)

// FileCollector represents configuration data from io.Reader.
type FileCollector struct {
	file string
	yc   YamlCollector
}

// Name implements the Collector interface.
func (f *FileCollector) Name() string {
	return f.yc.name
}

// Source implements the Collector interface.
func (f *FileCollector) Source() config.SourceType {
	return f.yc.sourceType
}

// Revision implements the Collector interface.
func (f *FileCollector) Revision() config.RevisionType {
	return f.yc.revision
}

// KeepOrder implements the Collector interface.
func (f *FileCollector) KeepOrder() bool {
	return f.yc.keepOrder
}

// Read implements the Collector interface.
func (f *FileCollector) Read(ctx context.Context) <-chan config.Value {
	return f.yc.Read(ctx)
}

// FileCollectorBuilder represent Builder object.
type FileCollectorBuilder struct {
	file string
	yc   YamlCollector
}

// NewFileCollectorBuilder returns new FileCollectorBuilder object.
func NewFileCollectorBuilder(file string) FileCollectorBuilder {
	return FileCollectorBuilder{
		file: file,
		yc: YamlCollector{
			name:       "file",
			sourceType: config.FileSource,
			revision:   "",
			keepOrder:  true,
			data:       nil,
		},
	}
}

// SetName sets a custom name for the collector.
func (fcb FileCollectorBuilder) SetName(name string) FileCollectorBuilder {
	fcb.yc.name = name
	return fcb
}

// SetSourceType sets the source type for the collector.
func (fcb FileCollectorBuilder) SetSourceType(source config.SourceType) FileCollectorBuilder {
	fcb.yc.sourceType = source
	return fcb
}

// SetRevision sets the revision for the collector.
func (fcb FileCollectorBuilder) SetRevision(rev config.RevisionType) FileCollectorBuilder {
	fcb.yc.revision = rev
	return fcb
}

// SetKeepOrder sets whether the collector preserves key order.
func (fcb FileCollectorBuilder) SetKeepOrder(keep bool) FileCollectorBuilder {
	fcb.yc.keepOrder = keep
	return fcb
}

// Build creates new FileCollector from the given file name.
func (fcb FileCollectorBuilder) Build() (*FileCollector, error) {
	data, err := os.ReadFile(filepath.Clean(fcb.file))
	if err != nil {
		return nil, fmt.Errorf("%w: %s", errFileError, fcb.file)
	}

	return &FileCollector{
		file: fcb.file,
		yc: YamlCollector{
			name:       fcb.yc.name,
			sourceType: fcb.yc.sourceType,
			revision:   fcb.yc.revision,
			keepOrder:  fcb.yc.keepOrder,
			data:       data,
		},
	}, nil
}
