package runner

import (
	"context"
	"crypto/tls"
	"errors"
	"time"
)

var errNoAttempts = errors.New("retryWithBackoff: MaxAttempts must be > 0")

// RetryConfig controls the retry behavior of retryWithBackoff.
type RetryConfig struct {
	MaxAttempts int           // required, must be > 0
	BaseDelay   time.Duration // initial backoff delay
	MaxDelay    time.Duration // cap on delay (defaults to 10s if zero)
}

// retryWithBackoff calls fn up to cfg.MaxAttempts times with exponential backoff.
// It stops early if the context is cancelled or if fn returns a ClassifiedError
// with a non-retryable category (Device or Protocol).
func retryWithBackoff(ctx context.Context, cfg RetryConfig, fn func() error) error {
	if cfg.MaxAttempts <= 0 {
		return errNoAttempts
	}
	if cfg.MaxDelay == 0 {
		cfg.MaxDelay = 10 * time.Second
	}

	var lastErr error
	for attempt := 0; attempt < cfg.MaxAttempts; attempt++ {
		lastErr = fn()
		if lastErr == nil {
			return nil
		}

		// Stop immediately on non-retryable errors.
		var ce *ClassifiedError
		if errors.As(lastErr, &ce) && ce.Category != ErrCatInfrastructure {
			return lastErr
		}

		// Don't sleep after the last attempt.
		if attempt == cfg.MaxAttempts-1 {
			break
		}

		// Exponential backoff: baseDelay * 2^attempt, capped at maxDelay.
		delay := min(cfg.BaseDelay<<uint(attempt), cfg.MaxDelay)

		timer := time.NewTimer(delay)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		}
	}
	return lastErr
}

// dialWithRetry wraps a TLS dial function with retryWithBackoff, returning
// the established connection or the last error after exhausting attempts.
func dialWithRetry(ctx context.Context, maxAttempts int, dialFn func() (*tls.Conn, error)) (*tls.Conn, error) {
	var conn *tls.Conn
	err := retryWithBackoff(ctx, RetryConfig{
		MaxAttempts: maxAttempts,
		BaseDelay:   50 * time.Millisecond,
		MaxDelay:    200 * time.Millisecond,
	}, func() error {
		var dialErr error
		conn, dialErr = dialFn()
		return dialErr
	})
	return conn, err
}

// contextSleep waits for the given duration or until the context is done,
// whichever comes first. Returns ctx.Err() if the context was cancelled.
func contextSleep(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		timer.Stop()
		return ctx.Err()
	}
}
