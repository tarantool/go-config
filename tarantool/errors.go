package tarantool

import "errors"

var (
	// ErrMutuallyExclusive is returned when both WithConfigFile and
	// WithConfigDir are set on the same Builder.
	ErrMutuallyExclusive = errors.New("tarantool: configFile and configDir are mutually exclusive")

	// ErrSchemaFetch is returned when an HTTP schema fetch fails.
	ErrSchemaFetch = errors.New("tarantool: failed to fetch config schema")

	// ErrSchemaRead is returned when a local schema file cannot be read.
	ErrSchemaRead = errors.New("tarantool: failed to read schema file")

	// ErrSchemaLoad is returned when an embedded schema payload cannot be
	// read from the embed FS or fails gzip decompression — a build-time
	// corruption, surfaced rather than panicked.
	ErrSchemaLoad = errors.New("tarantool: failed to load embedded schema")

	// ErrInvalidSchema is returned when schema bytes fail JSON Schema compilation.
	ErrInvalidSchema = errors.New("tarantool: invalid json schema")

	// ErrUnknownSchemaVersion is returned when the requested schema version
	// is not present in the registry.
	ErrUnknownSchemaVersion = errors.New("tarantool: unknown schema version")

	// ErrConflictingSchemaOptions is returned when mutually exclusive schema
	// options are set on the same Builder.
	ErrConflictingSchemaOptions = errors.New("tarantool: conflicting schema options")

	// ErrBadEnvIgnorePattern is returned when [Builder.WithEnvIgnore]
	// receives a pattern that path.Match rejects.
	ErrBadEnvIgnorePattern = errors.New("tarantool: invalid env-ignore pattern")
)
