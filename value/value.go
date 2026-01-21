package value

import "github.com/tarantool/go-config/meta"

// Value represents a single value in the configuration.
type Value interface {
	// Get is the main method for data extraction. It attempts
	// to convert and write the internal value into the variable `dest`,
	// passed by pointer. The type conversion logic is similar
	// to standard libraries such as `json.Unmarshal`.
	Get(dest any) error

	// Meta returns metadata (source name, revision) for this
	// particular value.
	Meta() meta.Info
}
