package tarantool //nolint:testpackage // Tests exercise package-private HTTP schema helpers directly.

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	config "github.com/tarantool/go-config"
)

func TestHTTPFetchSchema_RequestShape(t *testing.T) {
	t.Parallel()

	schema := []byte(`{"type":"object"}`)
	requests := make(chan *http.Request, 1)

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests <- request.Clone(request.Context())

		writer.Header().Set("Content-Type", "application/json")

		_, _ = writer.Write(schema)
	}))
	t.Cleanup(server.Close)

	got, err := httpFetchSchema(t.Context(), server.Client(), server.URL)
	require.NoError(t, err)
	assert.Equal(t, schema, got)

	request := <-requests
	assert.Equal(t, http.MethodGet, request.Method)
	assert.Equal(t, "go-config", request.Header.Get("User-Agent"))
	assert.Equal(t, "application/json", request.Header.Get("Accept"))
}

func TestHTTPFetchSchema_EmptyURL(t *testing.T) {
	t.Parallel()

	_, err := httpFetchSchema(t.Context(), nil, "")
	require.Error(t, err)
	require.ErrorIs(t, err, ErrSchemaFetch)
}

func TestHTTPFetchSchema_StatusError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, "nope", http.StatusBadGateway)
	}))
	t.Cleanup(server.Close)

	_, err := httpFetchSchema(t.Context(), server.Client(), server.URL)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrSchemaFetch)
	assert.Contains(t, err.Error(), "502")
}

func TestHTTPFetchSchema_ReadError(t *testing.T) {
	t.Parallel()

	client := &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Body:       io.NopCloser(failingReader{}),
				Header:     make(http.Header),
			}, nil
		}),
	}

	_, err := httpFetchSchema(t.Context(), client, "https://example.com/schema.json")
	require.Error(t, err)
	require.ErrorIs(t, err, ErrSchemaFetch)
	require.ErrorIs(t, err, errSchemaBodyRead)
}

func TestHTTPFetchSchema_InvalidSchema(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`{"type":42}`))
	}))
	t.Cleanup(server.Close)

	_, err := httpFetchSchema(t.Context(), server.Client(), server.URL)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidSchema)
}

func TestBuild_WithSchemaURL_UsesInjectedHTTPClient(t *testing.T) {
	t.Parallel()

	schema := []byte(`{
		"$schema":"https://json-schema.org/draft/2020-12/schema",
		"type":"object",
		"properties":{"key":{"type":"string"}},
		"additionalProperties":false
	}`)
	usedClient := false
	client := &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			usedClient = true

			assert.Equal(t, "go-config", request.Header.Get("User-Agent"))
			assert.Equal(t, "application/json", request.Header.Get("Accept"))

			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Body:       io.NopCloser(strings.NewReader(string(schema))),
				Header:     make(http.Header),
			}, nil
		}),
	}

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	writeTestFile(t, cfgPath, "key: value\n")

	cfg, err := New().
		WithConfigFile(cfgPath).
		WithSchemaURL("https://example.com/schema.json").
		WithHTTPClient(client).
		WithEnvPrefix("TT_TESTONLY_").
		Build(t.Context())
	require.NoError(t, err)
	assert.True(t, usedClient)

	var value string

	_, err = cfg.Get(config.NewKeyPath("key"), &value)
	require.NoError(t, err)
	assert.Equal(t, "value", value)
}

func TestBuild_WithSchemaURL_EmptyURL(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	writeTestFile(t, cfgPath, "key: value\n")

	_, err := New().
		WithConfigFile(cfgPath).
		WithSchemaURL("").
		Build(t.Context())
	require.Error(t, err)
	require.ErrorIs(t, err, ErrSchemaFetch)
}

var errSchemaBodyRead = errors.New("schema body read failed")

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

type failingReader struct{}

func (failingReader) Read(_ []byte) (int, error) {
	return 0, errSchemaBodyRead
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()

	err := os.WriteFile(path, []byte(content), 0o600)
	require.NoError(t, err)
}
