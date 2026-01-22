package tree

import "errors"

var (
	// ErrDestinationMustBePointer is returned when destination is not a non-nil pointer.
	ErrDestinationMustBePointer = errors.New("destination must be a non-nil pointer")
	// ErrUnsupportedDestinationType is returned when destination type is unsupported.
	ErrUnsupportedDestinationType = errors.New("unsupported destination type")
	// ErrParseDuration is returned when a duration string cannot be parsed.
	ErrParseDuration = errors.New("cannot parse duration from string")
	// ErrConvertToDuration is returned when a value cannot be converted to time.Duration.
	ErrConvertToDuration = errors.New("cannot convert to time.Duration")
	// ErrConvertToBool is returned when a value cannot be converted to bool.
	ErrConvertToBool = errors.New("cannot convert to bool")
	// ErrConvertToInt is returned when a value cannot be converted to int.
	ErrConvertToInt = errors.New("cannot convert to int")
	// ErrOverflow is returned when a value overflows the destination type.
	ErrOverflow = errors.New("overflow")
	// ErrNegativeToUnsigned is returned when a negative value cannot be converted to unsigned.
	ErrNegativeToUnsigned = errors.New("cannot convert negative value to unsigned")
	// ErrConvertToUint is returned when a value cannot be converted to uint.
	ErrConvertToUint = errors.New("cannot convert to uint")
	// ErrConvertToFloat is returned when a value cannot be converted to float.
	ErrConvertToFloat = errors.New("cannot convert to float")
	// ErrSourceNotSliceOrArray is returned when source is not a slice or array.
	ErrSourceNotSliceOrArray = errors.New("source is not a slice or array")
	// ErrSourceNotMap is returned when source is not a map.
	ErrSourceNotMap = errors.New("source is not a map")
	// ErrDestinationMapStringKeys is returned when destination map does not have string keys.
	ErrDestinationMapStringKeys = errors.New("destination map must have string keys")
	// ErrSourceMapKeyNotString is returned when source map key is not string.
	ErrSourceMapKeyNotString = errors.New("source map key is not string")
	// ErrSourceForStructMustBeMap is returned when source for struct is not a map.
	ErrSourceForStructMustBeMap = errors.New("source for struct must be a map")
	// ErrSourceMapMustHaveStringKeys is returned when source map does not have string keys.
	ErrSourceMapMustHaveStringKeys = errors.New("source map must have string keys")
)
