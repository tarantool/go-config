package collectors

import (
	"bytes"
	"context"
	"fmt"
	"strconv"

	"github.com/tarantool/go-config"
	"github.com/tarantool/go-config/tree"
	"github.com/tarantool/go-storage/integrity"
)

// Storage implements config.Collector for reading multiple configuration
// documents from a key-value storage under a common prefix with integrity
// verification. Each key's value is parsed according to the given Format
// and merged into a single config tree. Key names are used only for
// distinguishing documents; the YAML content determines the tree structure.
type Storage struct {
	name        string
	sourceType  config.SourceType
	revision    config.RevisionType
	keepOrder   bool
	skipInvalid bool
	typed       *integrity.Typed[[]byte]
	format      Format
	prefix      string
	delimiter   string
}

// NewStorage creates a new Storage collector that reads all keys under the
// prefix managed by the given integrity.Typed storage. Each key's value is
// parsed using the provided Format.
func NewStorage(
	typed *integrity.Typed[[]byte],
	prefix string,
	format Format,
) *Storage {
	return &Storage{
		name:        "storage",
		sourceType:  config.StorageSource,
		revision:    "",
		keepOrder:   false,
		skipInvalid: false,
		typed:       typed,
		format:      format,
		prefix:      prefix,
		delimiter:   "/",
	}
}

// WithName sets a custom name prefix for the collector (default "storage").
// The final SourceInfo.Name for each value will be "<name>:<prefix><key>",
// where <key> is the storage key from which the value was read.
// For example, WithName("etcd") with prefix "/config/" and key "app"
// produces SourceInfo.Name "etcd:/config/app".
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

// WithSkipInvalid sets whether Collectors should silently skip documents
// whose value fails to parse. Default is false: a parse error on any key
// causes Collectors to return a *FormatParseError identifying the offending
// key. Enable this for tolerant reads where a single bad document should
// not prevent loading the rest.
func (s *Storage) WithSkipInvalid(skip bool) *Storage {
	s.skipInvalid = skip
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

// SkipInvalid returns whether documents with parse errors are skipped
// silently instead of producing an error.
func (s *Storage) SkipInvalid() bool {
	return s.skipInvalid
}

// Collectors implements config.MultiCollector. It performs a range query with
// the configured prefix, validates integrity, parses each key's value using the
// collector's Format, and returns one sub-collector per storage key. Each
// sub-collector is merged independently by the Builder with its own
// MergerContext, source name, and revision.
// The parent Storage's revision is updated to the maximum ModRevision among
// the fetched keys.
// Keys with empty values are skipped. By default, a parse error on any key
// causes Collectors to return a *FormatParseError that identifies the
// offending key; use WithSkipInvalid(true) to silently skip invalid
// documents instead.
func (s *Storage) Collectors(ctx context.Context) ([]config.Collector, error) {
	results, err := s.typed.Range(ctx, "",
		integrity.IgnoreVerificationError())
	if err != nil {
		return nil, fmt.Errorf("storage range failed: %w", err)
	}

	if len(results) == 0 {
		return nil, nil
	}

	var maxRev int64

	docs := make([]config.Collector, 0, len(results))

	for _, result := range results {
		if result.ModRevision > maxRev {
			maxRev = result.ModRevision
		}

		if result.Value.IsZero() {
			continue
		}

		value := result.Value.Unwrap()

		if len(value) == 0 {
			continue
		}

		format := s.format.From(bytes.NewReader(value))

		subtree, parseErr := format.Parse()
		if parseErr != nil {
			if s.skipInvalid {
				continue
			}

			return nil, NewFormatParseError(s.prefix+result.Name, parseErr)
		}

		docName := s.sourceName(result.Name)
		setSource(subtree, docName)

		docs = append(docs, &storageDocument{
			docName:   docName,
			srcType:   s.sourceType,
			revision:  config.RevisionType(strconv.FormatInt(result.ModRevision, 10)),
			keepOrder: s.keepOrder,
			root:      subtree,
		})
	}

	s.revision = config.RevisionType(strconv.FormatInt(maxRev, 10))

	return docs, nil
}

// Read performs a range query with the configured prefix and emits all values
// from all documents on a single channel. This is a convenience method; the
// Builder uses Collectors for independent per-document merging.
func (s *Storage) Read(ctx context.Context) <-chan config.Value {
	valueChan := make(chan config.Value)

	go func() {
		defer close(valueChan)

		subs, err := s.Collectors(ctx)
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

// storageDocument is an unexported Collector wrapping a single parsed
// configuration document from storage. Each document is merged independently.
type storageDocument struct {
	docName   string
	srcType   config.SourceType
	revision  config.RevisionType
	keepOrder bool
	root      *tree.Node
}

// Read walks the parsed tree and emits leaf values.
func (d *storageDocument) Read(ctx context.Context) <-chan config.Value {
	valueChan := make(chan config.Value)

	go func() {
		defer close(valueChan)

		walkTree(ctx, d.root, config.NewKeyPath(""), valueChan)
	}()

	return valueChan
}

// Name returns the per-document source name (e.g. "etcd:/config/app").
func (d *storageDocument) Name() string { return d.docName }

// Source returns the source type inherited from the parent Storage collector.
func (d *storageDocument) Source() config.SourceType { return d.srcType }

// Revision returns the document's ModRevision.
func (d *storageDocument) Revision() config.RevisionType { return d.revision }

// KeepOrder returns whether key order should be preserved.
func (d *storageDocument) KeepOrder() bool { return d.keepOrder }

// Watch implements the Watcher interface. It returns a channel that streams
// change events for the configured prefix in storage.
func (s *Storage) Watch(ctx context.Context) (<-chan WatchEvent, error) {
	rawCh, err := s.typed.Watch(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("failed to watch storage: %w", err)
	}

	eventCh := make(chan WatchEvent)

	go func() {
		defer close(eventCh)

		for {
			select {
			case <-ctx.Done():
				return
			case _, ok := <-rawCh:
				if !ok {
					return
				}

				select {
				case <-ctx.Done():
					return
				case eventCh <- WatchEvent{Prefix: s.prefix}:
				}
			}
		}
	}()

	return eventCh, nil
}

// sourceName builds the source identifier for a specific storage key.
// The format is "<name>:<prefix><key>".
func (s *Storage) sourceName(key string) string {
	return s.name + ":" + s.prefix + key
}
