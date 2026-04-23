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

// Sealed after init — read without a mutex on the assumption of no further writes.
//
//nolint:gochecknoglobals // sealed after init; treated as read-only.
var embeddedRegistry = make(map[string][]byte)

//nolint:gochecknoglobals // package-level user registry is part of the public API.
var (
	userRegistryMu sync.RWMutex
	userRegistry   = make(map[string][]byte)
)

//nolint:gochecknoinits // auto-register embedded schemas at package load
func init() {
	entries, err := embeddedSchemas.ReadDir("schemas")
	if err != nil {
		panic("tarantool: failed to read embedded schemas directory: " + err.Error())
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		version := entry.Name()
		path := "schemas/" + version + "/config.schema.json.gz"

		compressed, readErr := fs.ReadFile(embeddedSchemas, path)
		if readErr != nil {
			panic(fmt.Sprintf("tarantool: failed to read embedded schema for version %s: %s", version, readErr))
		}

		data, decompErr := gunzip(compressed)
		if decompErr != nil {
			panic(fmt.Sprintf("tarantool: failed to decompress embedded schema for version %s: %s", version, decompErr))
		}

		_, compErr := jsonschema.NewCompiler().Compile(data)
		if compErr != nil {
			panic(fmt.Sprintf("tarantool: embedded schema for version %s failed to compile: %s", version, compErr))
		}

		embeddedRegistry[version] = data
	}
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
// Returns (nil, false) if the version is not known to either registry.
func Schema(version string) ([]byte, bool) {
	userRegistryMu.RLock()

	stored, ok := userRegistry[version]

	userRegistryMu.RUnlock()

	if !ok {
		stored, ok = embeddedRegistry[version]
		if !ok {
			return nil, false
		}
	}

	out := make([]byte, len(stored))
	copy(out, stored)

	return out, true
}

// SchemaVersions returns a slice of all version strings known to either the
// embedded or user registry, sorted ascending by semantic version
// (major.minor.patch). Duplicates across registries are collapsed.
func SchemaVersions() []string {
	userRegistryMu.RLock()

	versions := make([]string, 0, len(embeddedRegistry)+len(userRegistry))
	seen := make(map[string]struct{}, len(embeddedRegistry)+len(userRegistry))

	for v := range embeddedRegistry {
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

	// Sort using semver comparator, not strings.Sort (lexicographic would
	// misorder e.g. "3.10.0" before "3.5.0").
	n := len(versions)
	for i := 1; i < n; i++ {
		key := versions[i]

		j := i - 1 //nolint:varnamelen // standard insertion-sort index
		for j >= 0 && compareSemver(versions[j], key) > 0 {
			versions[j+1] = versions[j]
			j--
		}

		versions[j+1] = key
	}

	return versions
}

// User registrations are intentionally ignored: the default fallback must be
// deterministic and unaffected by runtime [RegisterSchema] calls.
func newestEmbeddedSchema() (string, []byte, bool) {
	var bestVer string

	var bestBytes []byte

	for v, b := range embeddedRegistry {
		if bestVer == "" || compareSemver(v, bestVer) > 0 {
			bestVer = v
			bestBytes = b
		}
	}

	if bestVer == "" {
		return "", nil, false
	}

	out := make([]byte, len(bestBytes))
	copy(out, bestBytes)

	return bestVer, out, true
}
