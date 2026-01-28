package config

import (
	"errors"
	"fmt"
)

var (
	// ErrKeyNotFound is returned when a configuration key is not found.
	ErrKeyNotFound = errors.New("key not found")
	// ErrPathNotFound is returned when a configuration path is not found.
	ErrPathNotFound = errors.New("path not found")
	// ErrValidationFailed is returned when configuration validation fails.
	ErrValidationFailed = errors.New("validation failed")
	// ErrSchemaInvalid is returned when schema parsing fails.
	ErrSchemaInvalid = errors.New("schema invalid")
)

// CollectorError wraps an error that occurred while processing a collector,
// providing context about which collector failed.
type CollectorError struct {
	CollectorName string
	Err           error
}

// NewCollectorError creates a new CollectorError.
func NewCollectorError(collectorName string, err error) *CollectorError {
	return &CollectorError{
		CollectorName: collectorName,
		Err:           err,
	}
}

func (e *CollectorError) Error() string {
	return fmt.Sprintf("collector %s: %v", e.CollectorName, e.Err)
}

func (e *CollectorError) Unwrap() error {
	return e.Err
}
