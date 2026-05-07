package config

import (
	"github.com/tarantool/go-config/meta"
	"github.com/tarantool/go-config/value"
)

// SourceType defines the type of configuration source.
type SourceType = meta.SourceType

const (
	// UnknownSource indicates an undefined source.
	UnknownSource = meta.UnknownSource
	// EnvDefaultSource indicates default values from environment variables (e.g., TT_FOO_DEFAULT).
	EnvDefaultSource = meta.EnvDefaultSource
	// StorageSource indicates an external centralized storage (e.g., Etcd or TcS).
	StorageSource = meta.StorageSource
	// FileSource indicates a local file.
	FileSource = meta.FileSource
	// EnvSource indicates environment variables.
	EnvSource = meta.EnvSource
	// ModifiedSource indicates dynamically modified data (e.g., at runtime) for MutableConfig.
	ModifiedSource = meta.ModifiedSource
)

// RevisionType defines a revision identifier of configuration, if applicable.
// Typically a string (e.g., commit hash, timestamp). If revision is not supported, it should be empty.
type RevisionType = meta.RevisionType

// SourceInfo contains information about the source where the value originated.
type SourceInfo = meta.SourceInfo

// MetaInfo contains metadata about a value in the configuration.
// Used to display the actual origin of the obtained value.
type MetaInfo = meta.Info

// Value represents a single value in the configuration.
type Value = value.Value
