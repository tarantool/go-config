package collectors

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tarantool/go-config"
	"github.com/tarantool/go-config/tree"
)

// Directory implements config.Collector and config.MultiCollector for reading
// multiple configuration files from a filesystem directory. Each file matching
// the configured extension is parsed according to the given Format and merged
// independently as a separate sub-collector. File names (without extension)
// are used for source identification; the file content determines the tree
// structure.
//
// When recursive mode is enabled, subdirectories are scanned recursively.
// Symbolic links to files are followed, but symbolic links to directories
// are skipped to prevent infinite loops and cyclic traversals.
type Directory struct {
	name       string
	sourceType config.SourceType
	revision   config.RevisionType
	keepOrder  bool
	path       string
	extension  string
	format     Format
	recursive  bool
}

// NewDirectory creates a new Directory collector that reads all files with
// the given extension from the specified directory path. Each file's content
// is parsed using the provided Format.
// The extension should include the leading dot (e.g., ".yaml").
func NewDirectory(
	path string,
	extension string,
	format Format,
) *Directory {
	return &Directory{
		name:       "directory",
		sourceType: config.FileSource,
		revision:   "",
		keepOrder:  false,
		path:       path,
		extension:  extension,
		format:     format,
		recursive:  false,
	}
}

// WithName sets a custom name prefix for the collector (default "directory").
// The final SourceInfo.Name for each value will be "<name>:<path>/<filename>",
// where <filename> is the name of the file from which the value was read.
// For example, WithName("config") with path "/etc/app" and file "db.yaml"
// produces SourceInfo.Name "config:/etc/app/db.yaml".
func (d *Directory) WithName(name string) *Directory {
	d.name = name
	return d
}

// WithSourceType sets the source type reported by the collector
// (default config.FileSource).
func (d *Directory) WithSourceType(source config.SourceType) *Directory {
	d.sourceType = source
	return d
}

// WithRevision sets the revision for the collector.
func (d *Directory) WithRevision(rev config.RevisionType) *Directory {
	d.revision = rev
	return d
}

// WithKeepOrder sets whether the collector should preserve the order
// of keys as they appear in each file (default false).
func (d *Directory) WithKeepOrder(keep bool) *Directory {
	d.keepOrder = keep
	return d
}

// WithRecursive sets whether to scan subdirectories recursively (default false).
func (d *Directory) WithRecursive(recursive bool) *Directory {
	d.recursive = recursive
	return d
}

// Name returns the collector's name.
func (d *Directory) Name() string {
	return d.name
}

// Source returns the collector's source type.
func (d *Directory) Source() config.SourceType {
	return d.sourceType
}

// Revision returns the collector's current revision.
func (d *Directory) Revision() config.RevisionType {
	return d.revision
}

// KeepOrder returns whether the collector preserves key order.
func (d *Directory) KeepOrder() bool {
	return d.keepOrder
}

// Recursive returns whether the collector scans subdirectories recursively.
func (d *Directory) Recursive() bool {
	return d.recursive
}

// Collectors implements config.MultiCollector. It reads the configured
// directory, filters files by extension, parses each file using the
// collector's Format, and returns one sub-collector per file. Each
// sub-collector is merged independently by the Builder with its own
// MergerContext, source name, and revision.
// Files that cannot be read or parsed are skipped.
// Symbolic links to files are followed, but symbolic links to directories
// are skipped.
func (d *Directory) Collectors(_ context.Context) ([]config.Collector, error) {
	entries, err := os.ReadDir(d.path)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDirectoryRead, err)
	}

	if len(entries) == 0 {
		return nil, nil
	}

	docs := make([]config.Collector, 0, len(entries))
	d.collectFiles(d.path, &docs)

	return docs, nil
}

// Read performs a directory scan and emits all values from all files on a
// single channel. This is a convenience method; the Builder uses Collectors
// for independent per-file merging.
func (d *Directory) Read(ctx context.Context) <-chan config.Value {
	valueChan := make(chan config.Value)

	go func() {
		defer close(valueChan)

		subs, err := d.Collectors(ctx)
		if err != nil {
			return
		}

		for _, sub := range subs {
			subCh := sub.Read(ctx)

			for val := range subCh {
				select {
				case <-ctx.Done():
					return
				case valueChan <- val:
				}
			}
		}
	}()

	return valueChan
}

func (d *Directory) collectFiles(dirPath string, docs *[]config.Collector) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return
	}

	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.Mode()&os.ModeSymlink != 0 {
			linkPath := filepath.Join(dirPath, entry.Name())

			targetInfo, err := os.Stat(linkPath)
			if err != nil {
				continue
			}

			if targetInfo.IsDir() {
				continue
			}
		}

		if entry.IsDir() {
			if d.recursive {
				subdirPath := filepath.Join(dirPath, entry.Name())
				d.collectFiles(subdirPath, docs)
			}

			continue
		}

		if !strings.HasSuffix(entry.Name(), d.extension) {
			continue
		}

		filePath := filepath.Join(dirPath, entry.Name())

		data, readErr := os.ReadFile(filepath.Clean(filePath))
		if readErr != nil {
			continue
		}

		if len(data) == 0 {
			continue
		}

		subtree, parseErr := d.parseData(data)
		if parseErr != nil {
			continue
		}

		relPath, _ := filepath.Rel(d.path, filePath)
		docName := d.sourceName(relPath)
		setSource(subtree, docName)

		*docs = append(*docs, &directoryDocument{
			docName:   docName,
			srcType:   d.sourceType,
			revision:  d.revision,
			keepOrder: d.keepOrder,
			root:      subtree,
		})
	}
}

// parseData parses raw bytes using the collector's format.
func (d *Directory) parseData(data []byte) (*tree.Node, error) {
	reader := strings.NewReader(string(data))
	format := d.format.From(reader)

	node, err := format.Parse()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFormatParse, err)
	}

	return node, nil
}

// sourceName builds the source identifier for a specific file.
// The format is "<name>:<path>/<filepath>" where filepath can be a filename
// or a relative path for files in subdirectories.
func (d *Directory) sourceName(filepath string) string {
	return d.name + ":" + d.path + "/" + filepath
}

// directoryDocument is an unexported Collector wrapping a single parsed
// configuration file from a directory. Each file is merged independently.
type directoryDocument struct {
	docName   string
	srcType   config.SourceType
	revision  config.RevisionType
	keepOrder bool
	root      *tree.Node
}

// Read walks the parsed tree and emits leaf values.
func (dd *directoryDocument) Read(ctx context.Context) <-chan config.Value {
	valueChan := make(chan config.Value)

	go func() {
		defer close(valueChan)

		walkTree(ctx, dd.root, config.NewKeyPath(""), valueChan)
	}()

	return valueChan
}

// Name returns the per-file source name (e.g. "config:/etc/app/db.yaml").
func (dd *directoryDocument) Name() string { return dd.docName }

// Source returns the source type inherited from the parent Directory collector.
func (dd *directoryDocument) Source() config.SourceType { return dd.srcType }

// Revision returns the file's revision.
func (dd *directoryDocument) Revision() config.RevisionType { return dd.revision }

// KeepOrder returns whether key order should be preserved.
func (dd *directoryDocument) KeepOrder() bool { return dd.keepOrder }
