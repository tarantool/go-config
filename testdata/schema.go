// Package testdata provides the shared JSON Schema for syntax tests.
package testdata

import _ "embed"

// Schema is the shared schema fixture. Treat its contents as read-only.
//
//go:embed schema.json
var Schema []byte
