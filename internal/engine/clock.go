package engine

import "time"

// Clock is the minimum time surface Daemon uses for serial post-pass gaps.
// Tests inject a fake so multi-tick cases finish without wall-clock sleeps.
// Production uses RealClock. Deliberately exposes After (not a wall-cadence
// ticker): D-04 requires Once → wait(interval) → Once, never overlapping
// ticks while a slow pass is still running.
type Clock interface {
	Now() time.Time
	After(d time.Duration) <-chan time.Time
}

type realClock struct{}

func (realClock) Now() time.Time                         { return time.Now() }
func (realClock) After(d time.Duration) <-chan time.Time { return time.After(d) }

// RealClock returns a Clock backed by the standard time package.
func RealClock() Clock { return realClock{} }
