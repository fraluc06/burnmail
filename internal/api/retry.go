package api

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"
)

// RequestTimeout bounds one logical API operation (initial call plus retries).
const RequestTimeout = 30 * time.Second

const (
	retryMaxAttempts = 3
	retryBaseDelay   = 1 * time.Second
	retryMaxDelay    = 10 * time.Second
)

// RetryWithBackoff retries fn with exponential backoff, but only while the
// error looks like a mail.tm rate-limit rejection (429); any other failure
// is returned immediately. It honors ctx cancellation between attempts.
func RetryWithBackoff[T any](ctx context.Context, fn func() (T, error)) (T, error) {
	var zero T
	for attempt := 0; attempt < retryMaxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		default:
		}

		result, err := fn()
		if err == nil {
			return result, nil
		}

		if attempt == retryMaxAttempts-1 {
			return zero, err
		}

		if strings.Contains(err.Error(), "429") || strings.Contains(err.Error(), "rate limit") {
			delay := min(time.Duration(math.Pow(2, float64(attempt)))*retryBaseDelay, retryMaxDelay)

			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return zero, ctx.Err()
			}
		} else {
			return zero, err
		}
	}

	return zero, errors.New("max retries exceeded")
}
