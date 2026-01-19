package testutil

import (
	"time"

	"github.com/shoenig/test"
)

const (
	drainTimeout = 1 * time.Minute
)

type options struct {
	timeout time.Duration
}

// Option configures Drain behavior.
// Use WithTimeout to customize the timeout duration.
type Option func(*options)

// WithTimeout returns an Option that sets a custom timeout for draining the channel.
// The default timeout is 1ms.
func WithTimeout(timeout time.Duration) Option {
	return func(o *options) {
		o.timeout = timeout
	}
}

// Drain reads values from the channel until it is closed, expecting each read to complete within a short timeout.
// If a read operation times out, the test fails with an error.
// This is useful for draining remaining values from a channel after cancellation in tests.
// The default timeout is 1ms per read attempt; use WithTimeout to customize.
//
// Example:
//
//	ch := make(chan int, 2)
//	ch <- 42
//	ch <- 99
//	close(ch)
//	testutil.Drain(t, ch) // Drains both values, uses default 1ms timeout per read
//
//	// With custom timeout
//	testutil.Drain(t, ch, testutil.WithTimeout(100*time.Millisecond))
func Drain[T any](tb test.T, channel <-chan T, opts ...Option) {
	tb.Helper()

	// Apply options.
	cfg := &options{timeout: drainTimeout}
	for _, opt := range opts {
		opt(cfg)
	}

	for {
		select {
		case _, ok := <-channel:
			if !ok {
				return
			}

		case <-time.After(cfg.timeout):
			tb.Errorf("failed to drain channel within %s", cfg.timeout)
			return
		}
	}
}
