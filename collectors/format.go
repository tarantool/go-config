package collectors

import (
	"io"

	"github.com/tarantool/go-config/tree"
)

// Format represents way to convert some data into the tree.Node.
type Format interface {
	Name() string
	KeepOrder() bool
	From(r io.Reader) Format
	Parse() (*tree.Node, error)
}
