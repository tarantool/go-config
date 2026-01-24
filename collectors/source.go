package collectors

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/tarantool/go-config"
	"github.com/tarantool/go-config/tree"
)

// DataSource represent data source.
type DataSource interface {
	Name() string
	SourceType() config.SourceType
	Revision() config.RevisionType
	FetchStream(ctx context.Context) (io.ReadCloser, error)
}

// File implements DataSource with data from file.
type File struct {
	name       string
	sourceType config.SourceType
	revision   config.RevisionType
	file       string
}

// NewFile returns new File object.
func NewFile(file string) File {
	return File{
		name:       "file",
		sourceType: config.FileSource,
		revision:   "",
		file:       file,
	}
}

// Name returns name of the source.
func (f File) Name() string {
	return f.name
}

// SourceType returns source type.
func (f File) SourceType() config.SourceType {
	return f.sourceType
}

// Revision returns data revision.
func (f File) Revision() config.RevisionType {
	return f.revision
}

// FetchStream returns reader.
func (f File) FetchStream(_ context.Context) (io.ReadCloser, error) {
	reader, err := os.Open(filepath.Clean(f.file))
	if err != nil {
		return nil, fmt.Errorf("%w %s: %w", ErrFile, f.file, err)
	}

	return reader, nil
}

// Source represent data source with format.
type Source struct {
	source DataSource
	format Format
	node   *tree.Node
}

// NewSource returns new Source object.
func NewSource(source DataSource, format Format) (config.Collector, error) {
	var err error

	ctx := context.Background()

	reader, err := source.FetchStream(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFetchStream, err)
	}
	defer reader.Close() //nolint:errcheck

	format = format.From(reader)

	node, err := format.Parse()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFormatParse, err)
	}

	return &Source{
		source: source,
		format: format,
		node:   node,
	}, nil
}

// Name implements Collector interface.
func (s *Source) Name() string {
	return s.source.Name()
}

// Source implements Collector interface.
func (s *Source) Source() config.SourceType {
	return s.source.SourceType()
}

// Revision implements Collector interface.
func (s *Source) Revision() config.RevisionType {
	return s.source.Revision()
}

// KeepOrder implements Collector interface.
func (s *Source) KeepOrder() bool {
	return s.format.KeepOrder()
}

// Read implements Collector interface.
func (s *Source) Read(ctx context.Context) <-chan config.Value {
	channel := make(chan config.Value)

	go func() {
		defer close(channel)

		// Walk the tree and send leaf values.
		// For simplicity, we traverse recursively.
		walkTree(ctx, s.node, config.NewKeyPath(""), channel)
	}()

	return channel
}
