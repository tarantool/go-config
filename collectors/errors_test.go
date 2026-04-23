package collectors_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tarantool/go-config/collectors"
)

var errFormatParseTestSentinel = errors.New("unexpected token")

func TestFormatParseError(t *testing.T) {
	t.Parallel()

	err := collectors.NewFormatParseError("/config/invalid", errFormatParseTestSentinel)

	require.NotNil(t, err)
	assert.Equal(t, "/config/invalid", err.Key)
	assert.Same(t, errFormatParseTestSentinel, err.Err)

	msg := err.Error()
	assert.Contains(t, msg, "failed to parse data with format")
	assert.Contains(t, msg, `"/config/invalid"`)
	assert.Contains(t, msg, "unexpected token")

	require.ErrorIs(t, err, errFormatParseTestSentinel)
	assert.Same(t, errFormatParseTestSentinel, errors.Unwrap(err))

	wrapped := error(err)

	var fpErr *collectors.FormatParseError

	require.ErrorAs(t, wrapped, &fpErr)
	assert.Equal(t, "/config/invalid", fpErr.Key)
}
