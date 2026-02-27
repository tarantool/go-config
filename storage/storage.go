package storage

import (
	"context"
	"errors"
)

var (
	// ErrRangeFailed indicates that a range query operation failed.
	ErrRangeFailed = errors.New("range query failed")
	// ErrTxFailed indicates that a transaction operation failed.
	ErrTxFailed = errors.New("transaction failed")
)

// RangeOption configures the behavior of a Range query.
type RangeOption func(*rangeOptions)

type rangeOptions struct {
	prefix []byte
}

// WithPrefix returns a RangeOption that restricts the range query
// to keys that have the given prefix.
func WithPrefix(prefix []byte) RangeOption {
	return func(o *rangeOptions) {
		o.prefix = prefix
	}
}

// Storage is a unified key-value storage abstraction.
// Implementations may provide transactional and range-based access.
type Storage interface {
	// Tx starts a new transaction.
	Tx(ctx context.Context) Tx
	// Range retrieves key-value pairs according to the given options.
	Range(ctx context.Context, opts ...RangeOption) ([]KeyValue, error)
}

// KeyValue represents a single key-value entry in storage.
type KeyValue struct {
	Key         []byte // The key.
	Value       []byte // The value.
	ModRevision int64  // Modification revision (for optimistic concurrency).
}

// Response holds the results of a committed transaction.
type Response struct {
	Results []Result // Individual operation results.
}

// Result contains the values produced by a single operation in a transaction.
type Result struct {
	Values []KeyValue // Values returned by the operation.
}

// Predicate represents a condition that can be used in a transaction's If clause.
type Predicate any

// Operation represents a storage operation that can be performed in a transaction.
type Operation interface {
	isOperation()
}

type getOp struct {
	key []byte
}

func (getOp) isOperation() {}

// Get creates an Operation that fetches the value for the given key.
func Get(key []byte) Operation {
	return getOp{key: key}
}

// Tx is a storage transaction that supports conditional execution.
type Tx interface {
	// If sets predicates that must be satisfied for the transaction to succeed.
	If(predicates ...Predicate) Tx
	// Then sets operations to be executed if all predicates are satisfied.
	Then(operations ...Operation) Tx
	// Else sets operations to be executed if any predicate is not satisfied.
	Else(operations ...Operation) Tx
	// Commit attempts to commit the transaction, returning the results or an error.
	Commit() (Response, error)
}
