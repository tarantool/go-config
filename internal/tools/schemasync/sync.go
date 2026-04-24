package main

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kaptinlin/jsonschema"
)

// Presence of this file — not the directory — defines whether a version is
// considered present on disk.
const schemaFileName = "config.schema.json.gz"

const maxSchemaBodyBytes = 32 << 20

const dirPerm os.FileMode = 0o750

var (
	errNilHTTPClient = errors.New("schemasync: Config.HTTPClient is nil")
	errNilLogger     = errors.New("schemasync: Config.Logger is nil")
)

var errUnexpectedStatus = errors.New("unexpected http status")

type Config struct {
	Repo        string
	SchemasDir  string
	SchemaURL   string
	GitHubToken string
	MinMajor    int
	DryRun      bool
	HTTPClient  *http.Client
	Logger      *slog.Logger
}

// Run prints exactly one stdout line of the form "added=<csv>" as the
// machine-readable handoff to the workflow step that opens the PR.
func Run(ctx context.Context, cfg Config) error {
	if cfg.HTTPClient == nil {
		return errNilHTTPClient
	}

	if cfg.Logger == nil {
		return errNilLogger
	}

	tags, err := fetchTags(ctx, cfg.HTTPClient, cfg.Repo, cfg.GitHubToken)
	if err != nil {
		return fmt.Errorf("fetch tags: %w", err)
	}

	stable := filterStableTags(tags, cfg.MinMajor)

	missing, err := missingVersions(cfg.SchemasDir, stable)
	if err != nil {
		return fmt.Errorf("diff missing versions: %w", err)
	}

	added := make([]string, 0, len(missing))

	for _, version := range missing {
		url := fmt.Sprintf(cfg.SchemaURL, version)

		data, ok, fetchErr := fetchSchema(ctx, cfg.HTTPClient, url)
		if fetchErr != nil {
			return fmt.Errorf("fetch schema %s: %w", version, fetchErr)
		}

		if !ok {
			cfg.Logger.Info("skip", "version", version, "reason", "upstream 404", "url", url)

			continue
		}

		validateErr := validateSchema(data)
		if validateErr != nil {
			return fmt.Errorf("validate schema %s: %w", version, validateErr)
		}

		if cfg.DryRun {
			cfg.Logger.Info("add", "version", version, "dry_run", true, "bytes", len(data))

			added = append(added, version)

			continue
		}

		writeErr := writeSchemaGz(cfg.SchemasDir, version, data)
		if writeErr != nil {
			return fmt.Errorf("write schema %s: %w", version, writeErr)
		}

		cfg.Logger.Info("add", "version", version, "bytes", len(data))

		added = append(added, version)
	}

	_, err = fmt.Fprintf(os.Stdout, "added=%s\n", strings.Join(added, ","))
	if err != nil {
		return fmt.Errorf("write stdout: %w", err)
	}

	return nil
}

// fetchSchema soft-skips 404 because upstream publishes schemas asynchronously
// relative to tags.
func fetchSchema(ctx context.Context, client *http.Client, url string) ([]byte, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, false, fmt.Errorf("build request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, false, fmt.Errorf("http do: %w", err)
	}

	defer func() { _ = resp.Body.Close() }()

	switch resp.StatusCode {
	case http.StatusOK:
		data, readErr := io.ReadAll(io.LimitReader(resp.Body, maxSchemaBodyBytes))
		if readErr != nil {
			return nil, false, fmt.Errorf("read body: %w", readErr)
		}

		return data, true, nil
	case http.StatusNotFound:
		return nil, false, nil
	default:
		return nil, false, fmt.Errorf("%w: %d for %s", errUnexpectedStatus, resp.StatusCode, url)
	}
}

// validateSchema must succeed before writing — a schema that doesn't compile
// would poison the embedded registry at package init time.
func validateSchema(data []byte) error {
	_, err := jsonschema.NewCompiler().Compile(data)
	if err != nil {
		return fmt.Errorf("schema failed to compile: %w", err)
	}

	return nil
}

// writeSchemaGz writes atomically (temp + rename) and deterministically so
// reruns on the same inputs produce byte-identical output.
func writeSchemaGz(schemasDir, version string, data []byte) error {
	versionDir := filepath.Join(schemasDir, version)

	mkdirErr := os.MkdirAll(versionDir, dirPerm)
	if mkdirErr != nil {
		return fmt.Errorf("mkdir %q: %w", versionDir, mkdirErr)
	}

	finalPath := filepath.Join(versionDir, schemaFileName)

	tmp, err := os.CreateTemp(versionDir, schemaFileName+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}

	tmpPath := tmp.Name()

	writeErr := writeGzipBody(tmp, data)
	if writeErr != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)

		return fmt.Errorf("write gzip body: %w", writeErr)
	}

	err = tmp.Close()
	if err != nil {
		_ = os.Remove(tmpPath)

		return fmt.Errorf("tmp close: %w", err)
	}

	err = os.Rename(tmpPath, finalPath)
	if err != nil {
		_ = os.Remove(tmpPath)

		return fmt.Errorf("rename %q -> %q: %w", tmpPath, finalPath, err)
	}

	return nil
}

// writeGzipBody does NOT close w; the caller owns the underlying file.
func writeGzipBody(w io.Writer, data []byte) error {
	writer, err := gzip.NewWriterLevel(w, gzip.BestCompression)
	if err != nil {
		return fmt.Errorf("gzip writer: %w", err)
	}

	// Zero non-deterministic fields so reruns produce byte-identical output.
	const gzipOSUnknown byte = 255 // RFC 1952 "unknown".

	writer.ModTime = time.Time{}
	writer.Name = ""
	writer.Comment = ""
	writer.Extra = nil
	writer.OS = gzipOSUnknown

	_, err = writer.Write(data)
	if err != nil {
		_ = writer.Close()

		return fmt.Errorf("gzip write: %w", err)
	}

	err = writer.Close()
	if err != nil {
		return fmt.Errorf("gzip close: %w", err)
	}

	return nil
}

// missingVersions preserves the input order of upstream.
func missingVersions(schemasDir string, upstream []string) ([]string, error) {
	present, err := presentVersions(schemasDir)
	if err != nil {
		return nil, err
	}

	missing := make([]string, 0, len(upstream))

	for _, v := range upstream {
		_, ok := present[v]
		if !ok {
			missing = append(missing, v)
		}
	}

	return missing, nil
}

func presentVersions(schemasDir string) (map[string]struct{}, error) {
	entries, err := os.ReadDir(schemasDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]struct{}{}, nil
		}

		return nil, fmt.Errorf("read schemas dir %q: %w", schemasDir, err)
	}

	present := make(map[string]struct{}, len(entries))

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		gzPath := filepath.Join(schemasDir, entry.Name(), schemaFileName)

		info, err := os.Stat(gzPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}

			return nil, fmt.Errorf("stat %q: %w", gzPath, err)
		}

		if info.Mode().IsRegular() {
			present[entry.Name()] = struct{}{}
		}
	}

	return present, nil
}
