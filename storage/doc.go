// Package storage defines a unified key-value storage abstraction.
// It provides interfaces for transactional and range-based access,
// enabling integration with various storage backends (e.g., etcd, Consul, etc.).
//
// The primary interface is Storage, which can start transactions (Tx)
// and perform range queries. Transactions support conditional execution
// via If/Then/Else clauses.
//
// This package is designed for use by go-config collectors that need to
// read configuration data from a key-value store.
package storage
