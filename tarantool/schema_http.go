package tarantool

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/kaptinlin/jsonschema"
)

const (
	// DefaultSchemaURL is the canonical Tarantool schema endpoint used by
	// [Builder.WithSchemaURLDefault].
	DefaultSchemaURL       = "https://download.tarantool.org/tarantool/schema/config.schema.json"
	maxSchemaResponseBytes = 5 * 1024 * 1024
	defaultHTTPTimeout     = 30 * time.Second
)

//nolint:gochecknoglobals,exhaustruct // package-private default client, never mutated
var defaultHTTPClient = &http.Client{Timeout: defaultHTTPTimeout}

func httpFetchSchema(ctx context.Context, client *http.Client, url string) ([]byte, error) {
	if url == "" {
		return nil, fmt.Errorf("%w: empty url", ErrSchemaFetch)
	}

	if client == nil {
		client = defaultHTTPClient
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrSchemaFetch, err)
	}

	req.Header.Set("User-Agent", "go-config")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrSchemaFetch, err)
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: unexpected status %s", ErrSchemaFetch, resp.Status)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxSchemaResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrSchemaFetch, err)
	}

	_, err = jsonschema.NewCompiler().Compile(data)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidSchema, err)
	}

	return data, nil
}
