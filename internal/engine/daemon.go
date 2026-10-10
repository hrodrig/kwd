package engine

import (
	"context"
	"time"

	"github.com/hrodrig/kwd/internal/check"
	"github.com/hrodrig/kwd/internal/report"
)

// DaemonEventType discriminates optional DaemonEvent payloads (test sync).
type DaemonEventType int

const (
	// EventPassCompleted fires after a completed (non-discarded) Once pass.
	EventPassCompleted DaemonEventType = iota
	// EventTransition fires when overall health flips (alert or resolution).
	EventTransition
)

// DaemonEvent is an optional signal for tests (kzero watchdog Events style).
// Production leaves Events nil.
type DaemonEvent struct {
	Type     DaemonEventType
	Alert    bool // true = unhealthy transition; only set for EventTransition
	Verdicts []check.Verdict
}

// DaemonOptions configures Engine.Daemon. Zero-value Clock uses RealClock;
// nil Events skips emit; hooks may be nil.
type DaemonOptions struct {
	Clock Clock
	// Events, when non-nil, receives non-blocking pass/transition signals
	// (sibling: kzero Config.Events). Useful for tests; production leaves nil.
	Events chan<- DaemonEvent
	// OnTick runs after every completed pass (transitioned may be false).
	OnTick func(verdicts []check.Verdict, transitioned bool)
	// OnTransition runs once per overall health flip. alert=true means
	// entering unhealthy; alert=false means resolution to all-ready.
	OnTransition func(ctx context.Context, alert bool, verdicts []check.Verdict) error
}

// Daemon runs Forever: Once → (hooks) → serial Clock.After(interval) → Once.
// It returns nil when ctx is canceled (D-07: signal → exit 0 via nil error).
// Cancelled mid-pass verdicts are discarded — no transition update, no notify.
//
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

	var (
		havePrev bool
		prevOK   bool
	)

	for {
		if ctx.Err() != nil {
			return nil
		}

		passCtx, cancel := context.WithTimeout(ctx, timeout)
		verdicts := e.Once(passCtx)
		cancel()

		// Parent canceled mid-pass (or during Once): discard — never a
		// false transition from errored checkers under a dead context (D-07).
		if ctx.Err() != nil {
			return nil
		}

		ok := !report.AnyNotReady(verdicts)
		transitioned := false
		alert := false

		if !havePrev {
			havePrev = true
			prevOK = ok
			// First tick: alert if starting unhealthy (matches single-pass
			// notify-on-not-ready / RESEARCH A1).
			if !ok {
				transitioned = true
				alert = true
			}
		} else if ok != prevOK {
			transitioned = true
			alert = !ok
			prevOK = ok
		}

		e.emit(opts.Events, DaemonEvent{Type: EventPassCompleted, Verdicts: verdicts})
		if transitioned {
			e.emit(opts.Events, DaemonEvent{Type: EventTransition, Alert: alert, Verdicts: verdicts})
		}

		if opts.OnTick != nil {
			opts.OnTick(verdicts, transitioned)
		}
		if transitioned && opts.OnTransition != nil {
			// Log notify errors in the CLI hook; do not kill the daemon loop.
			_ = opts.OnTransition(ctx, alert, verdicts)
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
