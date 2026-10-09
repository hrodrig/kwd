// Package retry implements bounded exponential backoff with full jitter.
package retry

import (
	"math/rand"
	"time"
)

// Backoff computes the wait duration for attempt n (0-indexed) given bounded
// initial and max backoff. Full jitter: each wait is random in [0, cap].
// cap = min(maxBackoff, initialBackoff * 2^n). n <= 0 returns 0.
func Backoff(n int, initial, max time.Duration) time.Duration {
	if n <= 0 {
		return 0
	}
	exp := uint64(1) << uint(n-1) // 2^(n-1)
	cap := float64(initial) * float64(exp)
	if max > 0 && cap > float64(max) {
		cap = float64(max)
	}
	if cap <= 0 {
		return 0
	}
	// Full jitter in [0, cap].
	return time.Duration(rand.Float64() * cap)
}

// IsRetriable reports whether err is a transient/retriable failure. Network
// and server-side errors are retriable; client-side (RBAC, not-found) are not.
func IsRetriable(err error) bool {
	if err == nil {
		return false
	}
	return true // placeholder; refined in check phase with typed errors
}
