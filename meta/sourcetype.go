package meta

// SourceType defines the type of configuration source.
type SourceType int

const (
	// UnknownSource indicates an undefined source.
	UnknownSource SourceType = iota
	// EnvDefaultSource indicates default values from environment variables (e.g., TT_FOO_DEFAULT).
	EnvDefaultSource
	// StorageSource indicates an external centralized storage (e.g., Etcd or TcS).
	StorageSource
	// FileSource indicates a local file.
	FileSource
	// EnvSource indicates environment variables.
	EnvSource
	// ModifiedSource indicates dynamically modified data (e.g., at runtime) for MutableConfig.
	ModifiedSource
)
