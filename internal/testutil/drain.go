package testutil

import (
	"time"
)

const (
	drainTimeout = 1 * time.Minute
)

type options struct {
	timeout time.Duration
}

// Option configures Drain behavior.
type Option func(*options)

// WithTimeout returns an Option that sets a custom timeout for draining the channel.
func WithTimeout(timeout time.Duration) Option {
	return func(o *options) {
		o.timeout = timeout
	}
}

// TB is a minimal interface for testing.TB.
type TB interface {
	Helper()
	Errorf(format string, args ...any)
}

// Drain reads values from the channel until it is closed, expecting each read to complete within a short timeout.
func Drain[T any](tb TB, channel <-chan T, opts ...Option) {
	tb.Helper()

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
