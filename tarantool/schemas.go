package tarantool

import (
	"bytes"
	"compress/gzip"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"sync"

	"github.com/kaptinlin/jsonschema"
)

//go:embed schemas/*/config.schema.json.gz
var embeddedSchemas embed.FS

// Embedded versions are discovered (and sorted) once at init from the embed
// directory listing. The gzipped payload for each version is decompressed
// lazily on first access via [loadEmbedded] — keeping startup cheap and
// avoiding ~2 MB of decompressed JSON in memory for callers that only ever
// touch one version (or none).
//
//nolint:gochecknoglobals // sealed after init; treated as read-only.
var (
	embeddedVersions []string                          // sorted ascending by semver.
	embeddedLoaders  map[string]func() ([]byte, error) // per-version sync.OnceValues.
)

//nolint:gochecknoglobals // package-level user registry is part of the public API.
var (
	userRegistryMu sync.RWMutex
	userRegistry   = make(map[string][]byte)
)

//nolint:gochecknoinits // discover embedded versions at package load
func init() {
	entries, err := embeddedSchemas.ReadDir("schemas")
	if err != nil {
		panic("tarantool: failed to read embedded schemas directory: " + err.Error())
	}

	embeddedLoaders = make(map[string]func() ([]byte, error), len(entries))
	embeddedVersions = make([]string, 0, len(entries))

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		version := entry.Name()
		path := "schemas/" + version + "/config.schema.json.gz"

		embeddedVersions = append(embeddedVersions, version)
		embeddedLoaders[version] = sync.OnceValues(func() ([]byte, error) {
			compressed, err := fs.ReadFile(embeddedSchemas, path)
			if err != nil {
				return nil, fmt.Errorf("%w: %s: %w", ErrSchemaLoad, version, err)
			}

			data, err := gunzip(compressed)
			if err != nil {
				return nil, fmt.Errorf("%w: %s: %w", ErrSchemaLoad, version, err)
			}

			return data, nil
		})
	}

	sortSemverAsc(embeddedVersions)
}

// sortSemverAsc sorts the slice in place ascending by [compareSemver] using
// insertion sort — the embedded version list is small (<100), so we avoid
// pulling in the sort package for a single call site.
//
//nolint:varnamelen // standard insertion-sort indices
func sortSemverAsc(versions []string) {
	for i := 1; i < len(versions); i++ {
		key := versions[i]

		j := i - 1
		for j >= 0 && compareSemver(versions[j], key) > 0 {
			versions[j+1] = versions[j]
			j--
		}

		versions[j+1] = key
	}
}

// loadEmbedded returns the (cached, lazily-decompressed) schema bytes for an
// embedded version. The bool reports whether the version is in the embedded
// registry; the error reports a load/decompression failure (wrapping
// [ErrSchemaLoad]) for a known version. The returned slice is shared across
// callers and must not be modified — callers handing it back to the public
// API are responsible for taking a defensive copy.
func loadEmbedded(version string) ([]byte, bool, error) {
	loader, ok := embeddedLoaders[version]
	if !ok {
		return nil, false, nil
	}

	data, err := loader()

	return data, true, err
}

// gunzip decompresses gzip-encoded bytes and returns the plaintext.
func gunzip(src []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(src))
	if err != nil {
		return nil, fmt.Errorf("gzip reader: %w", err)
	}

	defer func() { _ = reader.Close() }()

	out, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("gzip read: %w", err)
	}

	return out, nil
}

// RegisterSchema validates that schema bytes compile as a valid JSON Schema,
// then stores a defensive copy in the user registry keyed by version. User
// registrations do not influence the default "newest embedded" fallback used
// by [Builder.Build] when no schema setter is configured; the default path
// considers only versions shipped with the package.
// If the bytes fail to compile, the registry is left unchanged for that version
// and an error wrapping ErrInvalidSchema is returned.
func RegisterSchema(version string, schema []byte) error {
	_, err := jsonschema.NewCompiler().Compile(schema)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidSchema, err)
	}

	stored := make([]byte, len(schema))
	copy(stored, schema)

	userRegistryMu.Lock()
	userRegistry[version] = stored
	userRegistryMu.Unlock()

	return nil
}

// Schema returns a defensive copy of the schema bytes registered for version.
// User registrations take precedence over embedded versions with the same key.
//
// Returns an error wrapping [ErrUnknownSchemaVersion] if the version is not
// known to either registry, or [ErrSchemaLoad] if the embedded payload exists
// but failed to read or decompress.
func Schema(version string) ([]byte, error) {
	userRegistryMu.RLock()

	stored, ok := userRegistry[version]

	userRegistryMu.RUnlock()

	if !ok {
		var err error

		stored, ok, err = loadEmbedded(version)
		if err != nil {
			return nil, err
		}

		if !ok {
			return nil, fmt.Errorf("%w: %q", ErrUnknownSchemaVersion, version)
		}
	}

	out := make([]byte, len(stored))
	copy(out, stored)

	return out, nil
}

// SchemaVersions returns a slice of all version strings known to either the
// embedded or user registry, sorted ascending by semantic version
// (major.minor.patch). Duplicates across registries are collapsed.
func SchemaVersions() []string {
	userRegistryMu.RLock()

	versions := make([]string, 0, len(embeddedVersions)+len(userRegistry))
	seen := make(map[string]struct{}, len(embeddedVersions)+len(userRegistry))

	for _, v := range embeddedVersions {
		versions = append(versions, v)
		seen[v] = struct{}{}
	}

	for v := range userRegistry {
		if _, dup := seen[v]; dup {
			continue
		}

		versions = append(versions, v)
	}

	userRegistryMu.RUnlock()

	sortSemverAsc(versions)

	return versions
}

// User registrations are intentionally ignored: the default fallback must be
// deterministic and unaffected by runtime [RegisterSchema] calls.
//
// Returns an error wrapping [ErrUnknownSchemaVersion] when no embedded
// schemas are available, or [ErrSchemaLoad] when the newest payload fails to
// load.
func newestEmbeddedSchema() (string, []byte, error) {
	if len(embeddedVersions) == 0 {
		return "", nil, fmt.Errorf("%w: no embedded schemas available", ErrUnknownSchemaVersion)
	}

	bestVer := embeddedVersions[len(embeddedVersions)-1]

	stored, _, err := loadEmbedded(bestVer)
	if err != nil {
		return "", nil, err
	}

	out := make([]byte, len(stored))
	copy(out, stored)

	return bestVer, out, nil
}
