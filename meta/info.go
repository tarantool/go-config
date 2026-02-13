package meta

import (
	"github.com/tarantool/go-config/keypath"
)

// Info contains metadata about a value in the configuration.
// Used to display the actual origin of the obtained value.
type Info struct {
	// Key corresponds to the actual location of the value.
	Key keypath.KeyPath
	// Source information about the source/collector from which the value was obtained.
	Source SourceInfo
	// Revision number, if applicable.
	Revision RevisionType
}
