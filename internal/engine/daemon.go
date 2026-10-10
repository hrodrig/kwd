package engine

import (
	"context"
	"time"

	"github.com/hrodrig/kwd/internal/check"
)

// DaemonEventType discriminates optional DaemonEvent payloads (test sync).
type DaemonEventType int

const (
	// EventPassCompleted fires after a completed (non-discarded) Once pass.
	EventPassCompleted DaemonEventType = iota
	// EventTransition fires on NotifyAlert or NotifyResolve (overall flip).
	EventTransition
	// EventRepeat fires when NotifyRepeat is emitted (repeat_while_firing).
	EventRepeat
)

// DaemonEvent is an optional signal for tests (kzero watchdog Events style).
// Production leaves Events nil.
type DaemonEvent struct {
	Type     DaemonEventType
	Alert    bool // true = unhealthy (Alert/Repeat); false = Resolve
	Action   NotifyAction
	Verdicts []check.Verdict
}

// DaemonOptions configures Engine.Daemon. Zero-value Clock uses RealClock;
// nil Events skips emit; hooks may be nil.
type DaemonOptions struct {
	Clock Clock
	// Events, when non-nil, receives non-blocking pass/notify signals
	// (sibling: kzero Config.Events). Useful for tests; production leaves nil.
	Events chan<- DaemonEvent
	// OnTick runs after every completed pass (action may be NotifyNone).
	OnTick func(verdicts []check.Verdict, action NotifyAction)
	// OnNotify runs when action != NotifyNone (Alert / Resolve / Repeat).
	OnNotify func(ctx context.Context, action NotifyAction, verdicts []check.Verdict) error
}

// Daemon runs Forever: Once → hysteresis Apply → (hooks) → serial
// Clock.After(interval) → Once. It returns nil when ctx is canceled
// (D-07: signal → exit 0 via nil error). Cancelled mid-pass verdicts are
// discarded — no Apply, Store, or notify (Pitfall 5 / T-03-01).
//
// Hysteresis and TickSnapshot are owned here; HTTP listen is plan 03-02.
// D-04: never concurrent Once; never wall-cadence ticker for the gap.
func (e *Engine) Daemon(ctx context.Context, opts DaemonOptions) error {
	clk := opts.Clock
	if clk == nil {
		clk = RealClock()
	}

	interval := time.Duration(e.cfg.Interval) * time.Second
	timeout := e.cfg.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	hyst := NewHysteresis(e.cfg.ConfirmAlert, e.cfg.ConfirmOk, e.cfg.RepeatWhileFiring)

	for {
		if ctx.Err() != nil {
			return nil
		}

		passStart := clk.Now()
		passCtx, cancel := context.WithTimeout(ctx, timeout)
		verdicts := e.Once(passCtx)
		cancel()
		latency := clk.Now().Sub(passStart)

		// Parent canceled mid-pass (or during Once): discard — never poison
		// streaks / snapshot from errored checkers under a dead context.
		if ctx.Err() != nil {
			return nil
		}

		e.snapshot.Store(verdicts, latency, passStart.Unix())
		action := hyst.Apply(verdicts)

		e.emit(opts.Events, DaemonEvent{Type: EventPassCompleted, Action: action, Verdicts: verdicts})
		switch action {
		case NotifyAlert:
			e.emit(opts.Events, DaemonEvent{Type: EventTransition, Alert: true, Action: action, Verdicts: verdicts})
		case NotifyResolve:
			e.emit(opts.Events, DaemonEvent{Type: EventTransition, Alert: false, Action: action, Verdicts: verdicts})
		case NotifyRepeat:
			e.emit(opts.Events, DaemonEvent{Type: EventRepeat, Alert: true, Action: action, Verdicts: verdicts})
		}

		if opts.OnTick != nil {
			opts.OnTick(verdicts, action)
		}
		if action != NotifyNone && opts.OnNotify != nil {
			// Log notify errors in the CLI hook; do not kill the daemon loop.
			_ = opts.OnNotify(ctx, action, verdicts)
		}

		// Serial gap after a completed pass (D-04). Interruptible on cancel.
		select {
		case <-ctx.Done():
			return nil
		case <-clk.After(interval):
		}
	}
}

func (e *Engine) emit(ch chan<- DaemonEvent, ev DaemonEvent) {
	if ch == nil {
		return
	}
	select {
	case ch <- ev:
	default:
	}
}
