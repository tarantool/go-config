package tarantool

import "errors"

var (
	// ErrMutuallyExclusive is returned when both WithConfigFile and
	// WithConfigDir are set on the same Builder.
	ErrMutuallyExclusive = errors.New("tarantool: configFile and configDir are mutually exclusive")

	// ErrSchemaFetch is returned when the JSON Schema cannot be fetched
	// from the remote URL.
	ErrSchemaFetch = errors.New("tarantool: failed to fetch config schema")

	// ErrSchemaRead is returned when a local schema file cannot be read.
	ErrSchemaRead = errors.New("tarantool: failed to read schema file")
)
