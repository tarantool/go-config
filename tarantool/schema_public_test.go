package tarantool_test

import (
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	config "github.com/tarantool/go-config"
	"github.com/tarantool/go-config/tarantool"
)

var errSchemaTransport = errors.New("schema transport failed")

var errSchemaBodyRead = errors.New("schema body read failed")

func TestBuild_WithSchemaBytes_Public(t *testing.T) {
	t.Parallel()

	schema := []byte(`{
		"$schema":"https://json-schema.org/draft/2020-12/schema",
		"type":"object"
	}`)

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	writeFile(t, cfgPath, "key: value\n")

	cfg, err := tarantool.New().
		WithConfigFile(cfgPath).
		WithSchema(schema).
		Build(t.Context())
	require.NoError(t, err)

	var value string

	_, err = cfg.Get(config.NewKeyPath("key"), &value)
	require.NoError(t, err)
	assert.Equal(t, "value", value)
}

func TestBuild_WithSchemaFile_Public(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	schemaPath := filepath.Join(dir, "schema.json")

	writeFile(t, cfgPath, "key: value\n")
	writeFile(t, schemaPath, `{
		"$schema":"https://json-schema.org/draft/2020-12/schema",
		"type":"object"
	}`)

	cfg, err := tarantool.New().
		WithConfigFile(cfgPath).
		WithSchemaFile(schemaPath).
		Build(t.Context())
	require.NoError(t, err)

	var value string

	_, err = cfg.Get(config.NewKeyPath("key"), &value)
	require.NoError(t, err)
	assert.Equal(t, "value", value)
}

func TestBuild_WithSchemaFile_ReadError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	writeFile(t, cfgPath, "key: value\n")

	_, err := tarantool.New().
		WithConfigFile(cfgPath).
		WithSchemaFile(filepath.Join(dir, "missing-schema.json")).
		Build(t.Context())
	require.Error(t, err)
	require.ErrorIs(t, err, tarantool.ErrSchemaRead)
}

func TestBuild_WithSchemaURL_InvalidURL_UsesDefaultClientPath(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	writeFile(t, cfgPath, "key: value\n")

	_, err := tarantool.New().
		WithConfigFile(cfgPath).
		WithSchemaURL("://bad-url").
		Build(t.Context())
	require.Error(t, err)
	require.ErrorIs(t, err, tarantool.ErrSchemaFetch)
}

func TestBuild_WithSchemaURLDefault_UsesInjectedHTTPClient(t *testing.T) {
	t.Parallel()

	schema := []byte(`{
		"$schema":"https://json-schema.org/draft/2020-12/schema",
		"type":"object",
		"properties":{"key":{"type":"string"}},
		"additionalProperties":false
	}`)

	client := &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			assert.Equal(t, tarantool.DefaultSchemaURL, request.URL.String())
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
	writeFile(t, cfgPath, "key: value\n")

	cfg, err := tarantool.New().
		WithConfigFile(cfgPath).
		WithSchemaURLDefault().
		WithHTTPClient(client).
		WithEnvPrefix("TT_TESTONLY_").
		Build(t.Context())
	require.NoError(t, err)

	var value string

	_, err = cfg.Get(config.NewKeyPath("key"), &value)
	require.NoError(t, err)
	assert.Equal(t, "value", value)
}

func TestBuild_WithSchemaURL_EmptyURL(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	writeFile(t, cfgPath, "key: value\n")

	_, err := tarantool.New().
		WithConfigFile(cfgPath).
		WithSchemaURL("").
		Build(t.Context())
	require.Error(t, err)
	require.ErrorIs(t, err, tarantool.ErrSchemaFetch)
}

func TestBuild_WithSchemaURL_HTTPClientError(t *testing.T) {
	t.Parallel()

	client := &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, errSchemaTransport
		}),
	}

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	writeFile(t, cfgPath, "key: value\n")

	_, err := tarantool.New().
		WithConfigFile(cfgPath).
		WithSchemaURL("https://example.com/schema.json").
		WithHTTPClient(client).
		Build(t.Context())
	require.Error(t, err)
	require.ErrorIs(t, err, tarantool.ErrSchemaFetch)
	require.ErrorIs(t, err, errSchemaTransport)
}

func TestBuild_WithSchemaURL_HTTPStatusError(t *testing.T) {
	t.Parallel()

	client := &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusBadGateway,
				Status:     "502 Bad Gateway",
				Body:       io.NopCloser(strings.NewReader("bad gateway")),
				Header:     make(http.Header),
			}, nil
		}),
	}

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	writeFile(t, cfgPath, "key: value\n")

	_, err := tarantool.New().
		WithConfigFile(cfgPath).
		WithSchemaURL("https://example.com/schema.json").
		WithHTTPClient(client).
		Build(t.Context())
	require.Error(t, err)
	require.ErrorIs(t, err, tarantool.ErrSchemaFetch)
	assert.Contains(t, err.Error(), "502")
}

func TestBuild_WithSchemaURL_BodyReadError(t *testing.T) {
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

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	writeFile(t, cfgPath, "key: value\n")

	_, err := tarantool.New().
		WithConfigFile(cfgPath).
		WithSchemaURL("https://example.com/schema.json").
		WithHTTPClient(client).
		Build(t.Context())
	require.Error(t, err)
	require.ErrorIs(t, err, tarantool.ErrSchemaFetch)
	require.ErrorIs(t, err, errSchemaBodyRead)
}

func TestBuild_WithSchemaURL_InvalidSchema(t *testing.T) {
	t.Parallel()

	client := &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Body:       io.NopCloser(strings.NewReader(`{"type":42}`)),
				Header:     make(http.Header),
			}, nil
		}),
	}

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	writeFile(t, cfgPath, "key: value\n")

	_, err := tarantool.New().
		WithConfigFile(cfgPath).
		WithSchemaURL("https://example.com/schema.json").
		WithHTTPClient(client).
		Build(t.Context())
	require.Error(t, err)
	require.ErrorIs(t, err, tarantool.ErrInvalidSchema)
}

func TestSchemaVersions_InvalidVersionsSortLast(t *testing.T) {
	t.Parallel()

	require.NoError(t, tarantool.RegisterSchema("2.0.0", minimalValidSchema))
	require.NoError(t, tarantool.RegisterSchema("1.two.3", minimalValidSchema))
	require.NoError(t, tarantool.RegisterSchema("aaa", minimalValidSchema))
	require.NoError(t, tarantool.RegisterSchema("zzz", minimalValidSchema))

	versions := tarantool.SchemaVersions()

	assert.Greater(t, indexOf(versions, "1.two.3"), indexOf(versions, "2.0.0"))
	assert.Greater(t, indexOf(versions, "aaa"), indexOf(versions, "2.0.0"))
	assert.Greater(t, indexOf(versions, "zzz"), indexOf(versions, "2.0.0"))
	assert.Greater(t, indexOf(versions, "aaa"), indexOf(versions, "1.two.3"))
	assert.Greater(t, indexOf(versions, "zzz"), indexOf(versions, "aaa"))
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

type failingReader struct{}

func (failingReader) Read(_ []byte) (int, error) {
	return 0, errSchemaBodyRead
}

func indexOf(values []string, want string) int {
	for idx, value := range values {
		if value == want {
			return idx
		}
	}

	return -1
}
