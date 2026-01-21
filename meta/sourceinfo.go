package meta

// SourceInfo contains information about the source where the value originated.
type SourceInfo struct {
	// Name of the source where the value was obtained.
	Name string
	// Type of the source (file, environment variable, etc.).
	Type SourceType
}
