package collectors

import "errors"

var (
	// errFileError is returned when file processing failed.
	errFileError = errors.New("file processing error")
	// errReaderError is returned when io.Reader processing failed.
	errReaderError = errors.New("reader processing error")
)
