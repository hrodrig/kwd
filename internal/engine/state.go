package engine

import (
	"sync"
	"time"

	"github.com/hrodrig/kwd/internal/check"
)

// NotifyAction is the post-hysteresis notify decision for one completed tick.
type NotifyAction int

const (
	// NotifyNone means no FanOut this tick.
	NotifyNone NotifyAction = iota
	// NotifyAlert means enter overall firing (first confirmed unhealthy).
	NotifyAlert
	// NotifyResolve means leave overall firing (all confirmed good).
	NotifyResolve
	// NotifyRepeat means still firing with repeat_while_firing (not enter-firing).
	NotifyRepeat
)

// String returns a short name for logs/tests.
func (a NotifyAction) String() string {
	switch a {
	case NotifyAlert:
		return "alert"
	case NotifyResolve:
		return "resolve"
	case NotifyRepeat:
		return "repeat"
	default:
		return "none"
	}
}

// resourceStreak tracks per-resource confirm counters (D-09, D-11).
type resourceStreak struct {
	alert int
	ok    int
}

// Hysteresis holds per-resource alert/ok streaks and the overall firing bit.
// Constructed from cfg ConfirmAlert / ConfirmOk / RepeatWhileFiring.
type Hysteresis struct {
	confirmAlert int
	confirmOk    int
	repeat       bool
	firing       bool
	streaks      map[string]*resourceStreak
}

// NewHysteresis builds a Hysteresis from confirm/repeat knobs.
// Values below 1 are clamped to 1 so hand-built test Configs match Load defaults.
func NewHysteresis(confirmAlert, confirmOk int, repeatWhileFiring bool) *Hysteresis {
	if confirmAlert < 1 {
		confirmAlert = 1
	}
	if confirmOk < 1 {
		confirmOk = 1
	}
	return &Hysteresis{
		confirmAlert: confirmAlert,
		confirmOk:    confirmOk,
		repeat:       repeatWhileFiring,
		streaks:      make(map[string]*resourceStreak),
	}
}

// Firing reports whether overall post-hysteresis state is firing (tests / HTTP later).
func (h *Hysteresis) Firing() bool { return h.firing }

// Apply updates streaks from a completed pass and returns the notify action
// (RESEARCH Pattern 2 steps 1–6 / D-09…D-16). Opposite verdict resets the
// opposite streak to 0 (D-11). Unhealthy = Status != Ready (D-12).
func (h *Hysteresis) Apply(verdicts []check.Verdict) NotifyAction {
	anyConfirmedBad := false
	allConfirmedGood := len(verdicts) > 0

	for _, v := range verdicts {
		key := v.Ref.String()
		st := h.streaks[key]
		if st == nil {
			st = &resourceStreak{}
			h.streaks[key] = st
		}
		if v.Status != check.Ready {
			st.alert++
			st.ok = 0
		} else {
			st.ok++
			st.alert = 0
		}
		if st.alert >= h.confirmAlert {
			anyConfirmedBad = true
		}
		if st.ok < h.confirmOk {
			allConfirmedGood = false
		}
	}
	if len(verdicts) == 0 {
		allConfirmedGood = false
	}

	action := NotifyNone
	switch {
	case !h.firing && anyConfirmedBad:
		h.firing = true
		action = NotifyAlert
	case h.firing && allConfirmedGood:
		h.firing = false
		action = NotifyResolve
	case h.firing && h.repeat:
		// RESEARCH A4: Repeat only when still firing and not enter/leave this pass.
		action = NotifyRepeat
	}
	return action
}

// TickSnapshot is the last completed Once result for HTTP readers (D-01/D-02).
// Daemon is the sole writer; plan 03-02 handlers RLock via snapshot getters.
type TickSnapshot struct {
	mu            sync.RWMutex
	haveCompleted bool
	verdicts      []check.Verdict
	latency       time.Duration
	tickUnix      int64
}

// Store records a completed (non-discarded) pass under the write lock.
func (s *TickSnapshot) Store(verdicts []check.Verdict, latency time.Duration, tickUnix int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.haveCompleted = true
	s.verdicts = append([]check.Verdict(nil), verdicts...)
	s.latency = latency
	s.tickUnix = tickUnix
}

// HaveCompleted reports whether any completed pass has been stored (D-01).
func (s *TickSnapshot) HaveCompleted() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.haveCompleted
}

// CopyVerdicts returns a copy of the last completed verdicts (or nil).
func (s *TickSnapshot) CopyVerdicts() []check.Verdict {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.haveCompleted {
		return nil
	}
	return append([]check.Verdict(nil), s.verdicts...)
}

// Latency returns the last completed pass duration.
func (s *TickSnapshot) Latency() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.latency
}

// TickUnix returns the unix seconds of the last completed pass.
func (s *TickSnapshot) TickUnix() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.tickUnix
}
