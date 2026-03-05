package tarantool

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
)

const defaultSchemaURL = "https://download.tarantool.org/tarantool/schema/config.schema.json"

// resolveSchema returns the JSON Schema bytes according to the builder's
// configuration: explicit bytes, local file, or HTTP fetch from the default URL.
func (b *Builder) resolveSchema(ctx context.Context) ([]byte, error) {
	switch {
	case b.schema != nil:
		return b.schema, nil
	case b.schemaFile != "":
		data, err := os.ReadFile(b.schemaFile)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrSchemaRead, err)
		}

		return data, nil
	default:
		return fetchSchema(ctx, defaultSchemaURL)
	}
}

// fetchSchema performs an HTTP GET for the given URL and returns the
// response body. The request respects the provided context for
// cancellation and timeouts.
func fetchSchema(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrSchemaFetch, err)
	}

	resp, err := http.DefaultClient.Do(req) //nolint:gosec // URL is a package constant, not user input.
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrSchemaFetch, err)
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: unexpected status %d", ErrSchemaFetch, resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrSchemaFetch, err)
	}

	return data, nil
}
