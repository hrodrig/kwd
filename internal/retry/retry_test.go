package retry

import (
	"testing"
	"time"
)

func TestBackoffBounds(t *testing.T) {
	initial := time.Second
	max := 8 * time.Second

	// attempt 0 returns 0 (no wait before first try).
	if got := Backoff(0, initial, max); got != 0 {
		t.Fatalf("attempt 0 should be 0, got %s", got)
	}

	// attempts 1..6 must stay within [0, max].
	for n := 1; n <= 6; n++ {
		got := Backoff(n, initial, max)
		if got < 0 || got > max {
			t.Fatalf("attempt %d out of bounds: %s", n, got)
		}
	}
}

func TestBackoffCapsAtMax(t *testing.T) {
	initial := time.Second
	max := time.Second // max == initial caps growth immediately
	for n := 1; n <= 10; n++ {
		if got := Backoff(n, initial, max); got > max {
			t.Fatalf("attempt %d exceeded max: %s", n, got)
		}
	}
}

func TestBackoffExponentialGrowth(t *testing.T) {
	// At high n, the cap (initial * 2^(n-1)) exceeds max, so the returned
	// value must be bounded by max even though the exponent grows.
	initial := time.Second
	max := 2 * time.Second
	if got := Backoff(3, initial, max); got > max {
		t.Fatalf("expected capped at max, got %s", got)
	}
}

func TestIsRetriable(t *testing.T) {
	if IsRetriable(nil) {
		t.Fatal("nil error should not be retriable")
	}
	if !IsRetriable(errSomething{}) {
		t.Fatal("non-nil error should be retriable (v0 placeholder)")
	}
}

type errSomething struct{}

func (errSomething) Error() string { return "boom" }
