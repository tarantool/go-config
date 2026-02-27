package collectors

import "errors"

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
	// ErrFormatParse indicates that parsing data with format failed.
	ErrFormatParse = errors.New("failed to parse data with format")
	// ErrStorageFetch indicates that storage fetch failed.
	ErrStorageFetch = errors.New("storage fetch failed")
	// ErrStorageKeyNotFound indicates that a storage key was not found.
	ErrStorageKeyNotFound = errors.New("storage key not found")
	// ErrStorageRange indicates that a storage range query failed.
	ErrStorageRange = errors.New("storage range query failed")
)
