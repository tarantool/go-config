package collectors

import "errors"

var (
	// ErrNoData is returned when no data to process.
	ErrNoData = errors.New("no data to process")
	// ErrUnmarshall is returned when unmarshalling failed.
	ErrUnmarshall = errors.New("failed to unmarshall")
	// ErrFile is returned when file processing failed.
	ErrFile = errors.New("file processing error")
	// ErrReader is returned when io.Reader processing failed.
	ErrReader = errors.New("reader processing error")
	// ErrFetchStream is returned when io.Reader creation failed.
	ErrFetchStream = errors.New("failed to fetch the stream")
	// ErrFormatParse is returned when data processing failed.
	ErrFormatParse = errors.New("failed to parse data with format")
)
