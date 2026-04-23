package tarantool

import (
	"context"
	"fmt"
	"os"
)

// resolveSchema returns the JSON Schema bytes according to the builder's
// configuration:
//  1. Explicit bytes set via [Builder.WithSchema].
//  2. Local file set via [Builder.WithSchemaFile].
//  3. A specific registered version set via [Builder.WithSchemaVersion].
//  4. URL set via [Builder.WithSchemaURL].
//  5. Default HTTP endpoint via [Builder.WithSchemaURLDefault].
//  6. Default: the newest version available in the embedded schema registry.
//
// This function is only called when schema validation is wanted (skipSchema is
// false). Conflict detection happens earlier in [Builder.validate].
func (b *Builder) resolveSchema(ctx context.Context) ([]byte, error) {
	if b.schema != nil {
		return b.schema, nil
	}

	if b.schemaFile != "" {
		data, err := os.ReadFile(b.schemaFile)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrSchemaRead, err)
		}

		return data, nil
	}

	if b.schemaVersion != "" {
		schema, ok := Schema(b.schemaVersion)
		if !ok {
			return nil, fmt.Errorf("%w: %q", ErrUnknownSchemaVersion, b.schemaVersion)
		}

		return schema, nil
	}

	if b.schemaURLSet {
		return httpFetchSchema(ctx, b.httpClient, b.schemaURL)
	}

	if b.schemaHTTP {
		return httpFetchSchema(ctx, b.httpClient, DefaultSchemaURL)
	}

	// Default: newest embedded version.
	_, schema, ok := newestEmbeddedSchema()
	if !ok {
		return nil, fmt.Errorf("%w: no embedded schemas available", ErrUnknownSchemaVersion)
	}

	return schema, nil
}
