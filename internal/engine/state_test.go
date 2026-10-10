package engine

import (
	"testing"
	"time"

	"github.com/hrodrig/kwd/internal/check"
)

func refApp() check.Ref {
	return check.Ref{Kind: "deployment", Namespace: "default", Name: "app"}
}

func verdict(status check.Status) check.Verdict {
	return check.Verdict{Ref: refApp(), Status: status}
}

func TestHysteresisConfirm1MatchesPhase2(t *testing.T) {
	h := NewHysteresis(1, 1, false)
	if got := h.Apply([]check.Verdict{verdict(check.NotReady)}); got != NotifyAlert {
		t.Fatalf("first unhealthy: got %v, want Alert", got)
	}
	if got := h.Apply([]check.Verdict{verdict(check.NotReady)}); got != NotifyNone {
		t.Fatalf("stable unhealthy: got %v, want None", got)
	}
	if got := h.Apply([]check.Verdict{verdict(check.Ready)}); got != NotifyResolve {
		t.Fatalf("return ready: got %v, want Resolve", got)
	}
	if got := h.Apply([]check.Verdict{verdict(check.Ready)}); got != NotifyNone {
		t.Fatalf("stable ready: got %v, want None", got)
	}
}

func TestHysteresisConfirmAlert3(t *testing.T) {
	h := NewHysteresis(3, 1, false)
	bad := []check.Verdict{verdict(check.NotReady)}
	if got := h.Apply(bad); got != NotifyNone {
		t.Fatalf("tick1: got %v, want None", got)
	}
	if got := h.Apply(bad); got != NotifyNone {
		t.Fatalf("tick2: got %v, want None", got)
	}
	if got := h.Apply(bad); got != NotifyAlert {
		t.Fatalf("tick3: got %v, want Alert", got)
	}
}

func TestHysteresisConfirmOk3(t *testing.T) {
	h := NewHysteresis(1, 3, false)
	if h.Apply([]check.Verdict{verdict(check.NotReady)}) != NotifyAlert {
		t.Fatal("expected enter Alert")
	}
	good := []check.Verdict{verdict(check.Ready)}
	if got := h.Apply(good); got != NotifyNone {
		t.Fatalf("ok tick1 while firing: got %v, want None", got)
	}
	if got := h.Apply(good); got != NotifyNone {
		t.Fatalf("ok tick2 while firing: got %v, want None", got)
	}
	if got := h.Apply(good); got != NotifyResolve {
		t.Fatalf("ok tick3: got %v, want Resolve", got)
	}
}

func TestHysteresisOppositeResetsAlertStreak(t *testing.T) {
	h := NewHysteresis(3, 1, false)
	bad := []check.Verdict{verdict(check.NotReady)}
	good := []check.Verdict{verdict(check.Ready)}
	_ = h.Apply(bad)
	_ = h.Apply(bad)
	if got := h.Apply(good); got != NotifyNone {
		t.Fatalf("ready mid-streak: got %v, want None", got)
	}
	// Streak reset — need three more unhealthy ticks.
	if h.Apply(bad) != NotifyNone || h.Apply(bad) != NotifyNone {
		t.Fatal("expected None on first two unhealthy after reset")
	}
	if got := h.Apply(bad); got != NotifyAlert {
		t.Fatalf("after reset confirm: got %v, want Alert", got)
	}
}

func TestHysteresisErroredCountsUnhealthy(t *testing.T) {
	h := NewHysteresis(2, 1, false)
	errV := []check.Verdict{verdict(check.Errored)}
	if got := h.Apply(errV); got != NotifyNone {
		t.Fatalf("errored tick1: got %v, want None", got)
	}
	if got := h.Apply(errV); got != NotifyAlert {
		t.Fatalf("errored tick2: got %v, want Alert (D-12)", got)
	}
}

func TestHysteresisRepeatWhileFiring(t *testing.T) {
	h := NewHysteresis(1, 1, true)
	bad := []check.Verdict{verdict(check.NotReady)}
	if got := h.Apply(bad); got != NotifyAlert {
		t.Fatalf("enter: got %v, want Alert (A4 — not Repeat)", got)
	}
	if got := h.Apply(bad); got != NotifyRepeat {
		t.Fatalf("next gap: got %v, want Repeat", got)
	}
	if got := h.Apply(bad); got != NotifyRepeat {
		t.Fatalf("still firing: got %v, want Repeat", got)
	}
	if got := h.Apply([]check.Verdict{verdict(check.Ready)}); got != NotifyResolve {
		t.Fatalf("resolve: got %v, want Resolve", got)
	}
	if got := h.Apply([]check.Verdict{verdict(check.Ready)}); got != NotifyNone {
		t.Fatalf("no healthy repeats: got %v, want None", got)
	}
}

func TestHysteresisMultiResourceAnyBadEnterAllGoodLeave(t *testing.T) {
	h := NewHysteresis(1, 1, false)
	a := check.Ref{Kind: "deployment", Namespace: "default", Name: "a"}
	b := check.Ref{Kind: "deployment", Namespace: "default", Name: "b"}
	bothReady := []check.Verdict{
		{Ref: a, Status: check.Ready},
		{Ref: b, Status: check.Ready},
	}
	oneBad := []check.Verdict{
		{Ref: a, Status: check.NotReady},
		{Ref: b, Status: check.Ready},
	}
	if h.Apply(bothReady) != NotifyNone {
		t.Fatal("baseline ready → None")
	}
	if h.Apply(oneBad) != NotifyAlert {
		t.Fatal("any confirmed bad → Alert")
	}
	if h.Apply(bothReady) != NotifyResolve {
		t.Fatal("all confirmed good → Resolve")
	}
}

func TestTickSnapshotStoreAndCopy(t *testing.T) {
	var s TickSnapshot
	if s.HaveCompleted() {
		t.Fatal("empty snapshot must not haveCompleted")
	}
	v := []check.Verdict{verdict(check.Ready)}
	s.Store(v, 42*time.Millisecond, 1700000000)
	if !s.HaveCompleted() {
		t.Fatal("expected haveCompleted after Store")
	}
	got := s.CopyVerdicts()
	if len(got) != 1 || got[0].Status != check.Ready {
		t.Fatalf("CopyVerdicts = %+v", got)
	}
	// Mutating the copy must not affect the store.
	got[0].Status = check.NotReady
	if s.CopyVerdicts()[0].Status != check.Ready {
		t.Fatal("Store must copy verdicts")
	}
	if s.Latency() != 42*time.Millisecond || s.TickUnix() != 1700000000 {
		t.Fatalf("latency/unix = %v / %d", s.Latency(), s.TickUnix())
	}
}
