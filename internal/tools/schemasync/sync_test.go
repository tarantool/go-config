package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testDirPerm  os.FileMode = 0o750
	testFilePerm os.FileMode = 0o600
)

func TestMissingVersions(t *testing.T) {
	t.Parallel()

	type fsEntry struct {
		version   string
		hasGzFile bool
	}

	tests := []struct {
		name     string
		fs       []fsEntry
		upstream []string
		want     []string
	}{
		{
			name: "disjoint sets — all upstream missing",
			fs: []fsEntry{
				{version: "3.0.0", hasGzFile: true},
			},
			upstream: []string{"3.7.1", "4.0.0"},
			want:     []string{"3.7.1", "4.0.0"},
		},
		{
			name: "full overlap — nothing missing",
			fs: []fsEntry{
				{version: "3.0.0", hasGzFile: true},
				{version: "3.7.1", hasGzFile: true},
			},
			upstream: []string{"3.0.0", "3.7.1"},
			want:     []string{},
		},
		{
			name:     "empty upstream — nothing missing",
			fs:       []fsEntry{{version: "3.0.0", hasGzFile: true}},
			upstream: []string{},
			want:     []string{},
		},
		{
			name:     "empty filesystem — everything missing",
			fs:       nil,
			upstream: []string{"3.0.0", "3.7.1"},
			want:     []string{"3.0.0", "3.7.1"},
		},
		{
			name: "version directory without gz is counted as missing",
			fs: []fsEntry{
				{version: "3.0.0", hasGzFile: false},
			},
			upstream: []string{"3.0.0"},
			want:     []string{"3.0.0"},
		},
		{
			name: "preserves upstream input order, not alphabetical",
			fs: []fsEntry{
				{version: "3.7.1", hasGzFile: true},
			},
			upstream: []string{"4.0.0", "3.0.0", "3.7.1"},
			want:     []string{"4.0.0", "3.0.0"},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()

			for _, e := range testCase.fs {
				verDir := filepath.Join(dir, e.version)
				require.NoError(t, os.MkdirAll(verDir, testDirPerm))

				if e.hasGzFile {
					gzPath := filepath.Join(verDir, "config.schema.json.gz")
					require.NoError(t, os.WriteFile(gzPath, []byte("placeholder"), testFilePerm))
				}
			}

			got, err := missingVersions(dir, testCase.upstream)
			require.NoError(t, err)
			assert.Equal(t, testCase.want, got)
		})
	}
}

// repoRootRelativeSchema uses runtime.Caller because it is stable across
// `go test` invocations from different cwd, while a relative filepath is not.
func repoRootRelativeSchema(t *testing.T, version string) string {
	t.Helper()

	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok, "runtime.Caller failed")
	// schemasync -> tools -> internal -> <repo>.
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", "..", ".."))

	return filepath.Join(repoRoot, "tarantool", "schemas", version, "config.schema.json.gz")
}

func TestValidateSchema(t *testing.T) {
	t.Parallel()

	t.Run("real embedded schema compiles", func(t *testing.T) {
		t.Parallel()

		gzPath := repoRootRelativeSchema(t, "3.3.0")

		gzBytes, err := os.ReadFile(gzPath) //nolint:gosec
		require.NoError(t, err)

		reader, err := gzip.NewReader(bytes.NewReader(gzBytes))
		require.NoError(t, err)

		plain, err := io.ReadAll(reader)
		require.NoError(t, err)
		require.NoError(t, reader.Close())

		require.NoError(t, validateSchema(plain))
	})

	t.Run("garbage bytes fail with wrapped error", func(t *testing.T) {
		t.Parallel()

		err := validateSchema([]byte("not json"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "schema failed to compile")
	})
}

func TestWriteSchemaGz_Deterministic(t *testing.T) {
	t.Parallel()

	payload := []byte(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object"}`)

	dirA := t.TempDir()
	dirB := t.TempDir()

	require.NoError(t, writeSchemaGz(dirA, "3.9.9", payload))
	require.NoError(t, writeSchemaGz(dirB, "3.9.9", payload))

	gzA, err := os.ReadFile(filepath.Join(dirA, "3.9.9", schemaFileName)) //nolint:gosec
	require.NoError(t, err)

	gzB, err := os.ReadFile(filepath.Join(dirB, "3.9.9", schemaFileName)) //nolint:gosec
	require.NoError(t, err)

	assert.Equal(t, gzA, gzB, "same inputs must produce byte-identical gzip output")

	reader, err := gzip.NewReader(bytes.NewReader(gzA))
	require.NoError(t, err)

	round, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.NoError(t, reader.Close())

	assert.Equal(t, payload, round)
}

func TestFetchSchema_SoftSkip404(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	data, ok, err := fetchSchema(context.Background(), srv.Client(), srv.URL+"/missing.json")
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Nil(t, data)
}

func TestFetchSchema_Hard500(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	data, ok, err := fetchSchema(context.Background(), srv.Client(), srv.URL+"/boom.json")
	require.Error(t, err)
	assert.False(t, ok)
	assert.Nil(t, data)
}
