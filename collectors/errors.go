package collectors

import "errors"

var (
	// errUnmarshallError is returned when no data to process.
	errUnmarshallError = errors.New("failed to unmarshall")
	// errFileError is returned when file processing failed.
	errFileError = errors.New("file processing error")
	// errReaderError is returned when io.Reader processing failed.
	errReaderError = errors.New("reader processing error")
)
