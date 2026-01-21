package config

import "errors"

var (
	// ErrKeyNotFound is returned when a configuration key is not found.
	ErrKeyNotFound = errors.New("key not found")
	// ErrPathNotFound is returned when a configuration path is not found.
	ErrPathNotFound = errors.New("path not found")
)
