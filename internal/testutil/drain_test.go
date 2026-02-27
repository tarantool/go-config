package testutil_test

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/tarantool/go-config/internal/testutil"
)

func TestDrain_ClosedChannel(t *testing.T) {
	t.Parallel()

	channel := make(chan int)
	close(channel)

	testutil.Drain(t, channel)
}

func TestDrain_ValueReady(t *testing.T) {
	t.Parallel()

	channel := make(chan int, 1)
	channel <- 42

	close(channel)

	testutil.Drain(t, channel)

	_, ok := <-channel
	assert.False(t, ok)
}

func TestDrain_GenericTypes(t *testing.T) {
	t.Parallel()

	t.Run("string", func(t *testing.T) {
		t.Parallel()

		channel := make(chan string, 1)
		channel <- "hello"

		close(channel)

		testutil.Drain(t, channel)
	})

	t.Run("struct", func(t *testing.T) {
		t.Parallel()

		type myStruct struct{ x int }

		channel := make(chan myStruct, 1)
		channel <- myStruct{x: 42}

		close(channel)

		testutil.Drain(t, channel)
	})
}

func TestDrain_WithTimeout(t *testing.T) {
	t.Parallel()

	t.Run("custom_timeout_with_value", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			channel := make(chan string, 1)
			channel <- "test"

			close(channel)

			testutil.Drain(t, channel, testutil.WithTimeout(100*time.Millisecond))
		})
	})

	t.Run("multiple_options", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			channel := make(chan int, 1)
			channel <- 42

			close(channel)

			testutil.Drain(t, channel, testutil.WithTimeout(50*time.Millisecond))
		})
	})
}

func TestDrain_MultipleValues(t *testing.T) {
	t.Parallel()

	t.Run("buffered_multiple_values", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			channel := make(chan int, 3)
			channel <- 1

			channel <- 2

			channel <- 3

			close(channel)

			testutil.Drain(t, channel)
		})
	})

	t.Run("unbuffered_with_goroutine", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			channel := make(chan int)

			go func() {
				channel <- 1

				channel <- 2

				channel <- 3

				close(channel)
			}()

			testutil.Drain(t, channel)
		})
	})
}

type mockTB struct {
	helperCalled bool
	errorfCalled bool
	errorfFormat string
	errorfArgs   []any
}

func (m *mockTB) Helper() {
	m.helperCalled = true
}

func (m *mockTB) Errorf(format string, args ...any) {
	m.errorfCalled = true
	m.errorfFormat = format
	m.errorfArgs = args
}

func newMockTB() *mockTB {
	return &mockTB{
		helperCalled: false,
		errorfCalled: false,
		errorfFormat: "",
		errorfArgs:   nil,
	}
}

func TestDrain_TimeoutBehavior(t *testing.T) {
	t.Parallel()

	t.Run("timeout_on_unclosed_channel", func(t *testing.T) {
		t.Parallel()

		mock := newMockTB()

		synctest.Test(t, func(_ *testing.T) {
			channel := make(chan int)
			testutil.Drain(mock, channel)
		})

		assert.True(t, mock.helperCalled, "Helper should have been called")
		assert.True(t, mock.errorfCalled, "Errorf should have been called for timeout")
		assert.NotEmpty(t, mock.errorfFormat, "error message should not be empty")
	})

	t.Run("timeout_after_partial_drain", func(t *testing.T) {
		t.Parallel()

		mock := newMockTB()

		synctest.Test(t, func(_ *testing.T) {
			channel := make(chan int, 2)
			channel <- 1

			channel <- 2

			testutil.Drain(mock, channel)
		})

		assert.True(t, mock.helperCalled, "Helper should have been called")
		assert.True(t, mock.errorfCalled, "Errorf should have been called for timeout")
	})

	t.Run("zero_timeout_fails_immediately", func(t *testing.T) {
		t.Parallel()

		mock := newMockTB()

		synctest.Test(t, func(_ *testing.T) {
			channel := make(chan int)
			testutil.Drain(mock, channel, testutil.WithTimeout(0))
		})

		assert.True(t, mock.errorfCalled, "Errorf should have been called with zero timeout")
	})
}
