package collectors

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/tarantool/go-config"
)

// DataSource represent data source.
type DataSource interface {
	Name() string
	SourceType() config.SourceType
	Fetch(ctx context.Context) ([]byte, error)
}

// YAML implements source with data from any io.Reader.
type YAML struct {
	name       string
	sourceType config.SourceType
	reader     io.Reader
}

// NewYaml returns new YAML object.
func NewYaml(reader io.Reader) YAML {
	return YAML{
		name:       "yaml",
		sourceType: config.UnknownSource,
		reader:     reader,
	}
}

// Name returns name of the source.
func (y *YAML) Name() string {
	return y.name
}

// SourceType returns source type.
func (y *YAML) SourceType() config.SourceType {
	return y.sourceType
}

// Fetch returns slice of bytes of the data.
func (y *YAML) Fetch(_ context.Context) ([]byte, error) {
	data, err := io.ReadAll(y.reader)
	if err != nil {
		return nil, fmt.Errorf("%w", errReaderError)
	}

	return data, nil
}

// FILE implements source with data from file.
type FILE struct {
	name       string
	sourceType config.SourceType
	file       string
}

// NewFile returns new FILE object.
func NewFile(file string) FILE {
	return FILE{
		name:       "file",
		sourceType: config.FileSource,
		file:       file,
	}
}

// Name returns name of the source.
func (f *FILE) Name() string {
	return f.name
}

// SourceType returns source type.
func (f *FILE) SourceType() config.SourceType {
	return f.sourceType
}

// Fetch returns slice of bytes of the data.
func (f *FILE) Fetch(_ context.Context) ([]byte, error) {
	data, err := os.ReadFile(filepath.Clean(f.file))
	if err != nil {
		return nil, fmt.Errorf("%w: %s", errFileError, f.file)
	}

	return data, nil
}
