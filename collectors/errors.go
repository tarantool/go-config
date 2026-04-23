package collectors

import (
	"errors"
	"fmt"
)

var (
	// ErrNoData indicates that there is no data to process.
	ErrNoData = errors.New("no data to process")
	// ErrUnmarshall indicates that unmarshalling failed.
	ErrUnmarshall = errors.New("failed to unmarshall")
	// ErrFile indicates a file processing error.
	ErrFile = errors.New("file processing error")
	// ErrReader indicates a reader processing error.
	ErrReader = errors.New("reader processing error")
	// ErrFetchStream indicates that fetching the stream failed.
	ErrFetchStream = errors.New("failed to fetch the stream")
	// ErrStorageFetch indicates that storage fetch failed.
	ErrStorageFetch = errors.New("storage fetch failed")
	// ErrStorageKeyNotFound indicates that a storage key was not found.
	ErrStorageKeyNotFound = errors.New("storage key not found")
	// ErrStorageRange indicates that a storage range query failed.
	ErrStorageRange = errors.New("storage range query failed")
	// ErrStorageValidation indicates that storage integrity validation failed.
	ErrStorageValidation = errors.New("storage integrity validation failed")
	// ErrDirectoryRead indicates that reading a directory failed.
	ErrDirectoryRead = errors.New("directory read failed")
)

// FormatParseError indicates that parsing a configuration value with the
// configured format failed. Key identifies the offending source (storage key,
// file path, etc.) and Err is the underlying parser error. Callers can match
// it with errors.As(&FormatParseError{}) and inspect Err directly.
type FormatParseError struct {
	Key string
	Err error
}

// NewFormatParseError builds a FormatParseError for the given source key and
// underlying parser error.
func NewFormatParseError(key string, err error) *FormatParseError {
	return &FormatParseError{Key: key, Err: err}
}

// Error renders the full message including the key and the wrapped error.
func (e *FormatParseError) Error() string {
	return fmt.Sprintf("failed to parse data with format: key %q: %v", e.Key, e.Err)
}

// Unwrap exposes the underlying parser error so errors.Is/As can reach it.
func (e *FormatParseError) Unwrap() error {
	return e.Err
}
