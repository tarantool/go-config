package config_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tarantool/go-config"
)

type customError string

func (e customError) Error() string {
	return string(e)
}

func TestNewCollectorError(t *testing.T) {
	t.Parallel()

	innerErr := errors.New("inner error") //nolint:err113
	collectorName := "test-collector"
	err := config.NewCollectorError(collectorName, innerErr)

	require.NotNil(t, err)
	assert.Equal(t, collectorName, err.CollectorName)
	assert.Equal(t, innerErr, err.Err)
}

func TestCollectorError_Error(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		collectorName string
		innerErr      error
		expectedMsg   string
	}{
		{
			name:          "simple error",
			collectorName: "map",
			innerErr:      errors.New("failed to read"), //nolint:err113
			expectedMsg:   "collector map: failed to read",
		},
		{
			name:          "empty collector name",
			collectorName: "",
			innerErr:      errors.New("some error"), //nolint:err113
			expectedMsg:   "collector : some error",
		},
		{
			name:          "nil inner error",
			collectorName: "test",
			innerErr:      nil,
			expectedMsg:   "collector test: <nil>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := config.NewCollectorError(tt.collectorName, tt.innerErr)
			assert.Equal(t, tt.expectedMsg, err.Error())
		})
	}
}

func TestCollectorError_Unwrap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		innerErr       error
		expectedUnwrap error
	}{
		{
			name:           "non-nil inner error",
			innerErr:       errors.New("original error"), //nolint:err113
			expectedUnwrap: errors.New("original error"), //nolint:err113
		},
		{
			name:           "nil inner error",
			innerErr:       nil,
			expectedUnwrap: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := config.NewCollectorError("test", tt.innerErr)
			unwrapped := err.Unwrap()
			assert.Equal(t, tt.expectedUnwrap, unwrapped)
		})
	}
}

func TestCollectorError_Unwrap_ErrorsIs(t *testing.T) {
	t.Parallel()

	innerErr := errors.New("inner error") //nolint:err113
	wrappedErr := config.NewCollectorError("test", innerErr)

	assert.ErrorIs(t, wrappedErr, innerErr)
}

func TestCollectorError_Unwrap_ErrorsAs(t *testing.T) {
	t.Parallel()

	innerErr := customError("custom error")
	wrappedErr := config.NewCollectorError("test", innerErr)

	var target customError
	require.ErrorAs(t, wrappedErr, &target)
	assert.Equal(t, innerErr, target)
}
