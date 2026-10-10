package engine

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/hrodrig/kwd/internal/check"
	"github.com/hrodrig/kwd/internal/config"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

// stepClock drives Daemon gaps without wall sleeps. After returns a channel
// that FireAfter delivers on (one waiter at a time).
type stepClock struct {
	mu      sync.Mutex
	now     time.Time
	pending chan time.Time
}

func newStepClock() *stepClock {
	return &stepClock{now: time.Unix(0, 0).UTC()}
}

func (c *stepClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *stepClock) After(d time.Duration) <-chan time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	ch := make(chan time.Time, 1)
	c.pending = ch
	_ = d
	return ch
}

func (c *stepClock) FireAfter() {
	c.mu.Lock()
	ch := c.pending
	c.pending = nil
	c.now = c.now.Add(time.Second)
	now := c.now
	c.mu.Unlock()
	if ch != nil {
		ch <- now
	}
}

func waitEvent(t *testing.T, events <-chan DaemonEvent, want DaemonEventType, timeout time.Duration) DaemonEvent {
	t.Helper()
	select {
	case ev := <-events:
		if ev.Type != want {
			t.Fatalf("expected event %v, got %v", want, ev.Type)
		}
		return ev
	case <-time.After(timeout):
		t.Fatalf("timeout waiting for event %v", want)
		return DaemonEvent{}
	}
}

func TestDaemonMultiTickSameOncePath(t *testing.T) {
	cs := fake.NewSimpleClientset(&appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default"},
		Status:     appsv1.DeploymentStatus{Replicas: 1, ReadyReplicas: 1},
	})
	cfg := &config.Config{
		Resources: []string{"deployment.default/app"},
		Interval:  1,
		Timeout:   5 * time.Second,
	}
	clk := newStepClock()
	events := make(chan DaemonEvent, 8)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eng := New(cfg, cs, check.NewRegistry())
	errCh := make(chan error, 1)
	go func() {
		errCh <- eng.Daemon(ctx, DaemonOptions{Clock: clk, Events: events})
	}()

	ev := waitEvent(t, events, EventPassCompleted, 2*time.Second)
	if len(ev.Verdicts) != 1 {
		t.Fatalf("expected 1 verdict (same Once path), got %d", len(ev.Verdicts))
	}
	if ev.Verdicts[0].Status != check.Ready {
		t.Fatalf("expected ready, got %s", ev.Verdicts[0].Status)
	}

	clk.FireAfter()
	ev2 := waitEvent(t, events, EventPassCompleted, 2*time.Second)
	if len(ev2.Verdicts) != 1 {
		t.Fatalf("second pass: expected 1 verdict, got %d", len(ev2.Verdicts))
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Daemon should return nil on cancel, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Daemon did not exit after cancel")
	}
}

func TestTransitionAlertAndResolve(t *testing.T) {
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default"},
		Status:     appsv1.DeploymentStatus{Replicas: 1, ReadyReplicas: 1},
	}
	cs := fake.NewSimpleClientset(dep)
	cfg := &config.Config{
		Resources: []string{"deployment.default/app"},
		Interval:  1,
		Timeout:   5 * time.Second,
	}
	clk := newStepClock()
	events := make(chan DaemonEvent, 16)

	var (
		mu          sync.Mutex
		transitions []bool // alert values
		tickCount   int
	)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eng := New(cfg, cs, check.NewRegistry())
	errCh := make(chan error, 1)
	go func() {
		errCh <- eng.Daemon(ctx, DaemonOptions{
			Clock:  clk,
			Events: events,
			OnTick: func(_ []check.Verdict, _ bool) {
				mu.Lock()
				tickCount++
				mu.Unlock()
			},
			OnTransition: func(_ context.Context, alert bool, _ []check.Verdict) error {
				mu.Lock()
				transitions = append(transitions, alert)
				mu.Unlock()
				return nil
			},
		})
	}()

	// Tick 1: healthy baseline — no transition.
	waitEvent(t, events, EventPassCompleted, 2*time.Second)

	// Flip unhealthy.
	dep.Status.ReadyReplicas = 0
	if _, err := cs.AppsV1().Deployments("default").UpdateStatus(context.Background(), dep, metav1.UpdateOptions{}); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}
	clk.FireAfter()
	waitEvent(t, events, EventPassCompleted, 2*time.Second)
	tr := waitEvent(t, events, EventTransition, 2*time.Second)
	if !tr.Alert {
		t.Fatal("expected alert=true on ready→unhealthy")
	}

	// Stable unhealthy tick — no extra transition.
	clk.FireAfter()
	waitEvent(t, events, EventPassCompleted, 2*time.Second)
	select {
	case ev := <-events:
		if ev.Type == EventTransition {
			t.Fatal("stable unhealthy tick must not transition")
		}
	case <-time.After(50 * time.Millisecond):
		// no transition event — good
	}

	// Flip back healthy → resolution.
	dep.Status.ReadyReplicas = 1
	if _, err := cs.AppsV1().Deployments("default").UpdateStatus(context.Background(), dep, metav1.UpdateOptions{}); err != nil {
		t.Fatalf("UpdateStatus resolve: %v", err)
	}
	clk.FireAfter()
	waitEvent(t, events, EventPassCompleted, 2*time.Second)
	tr2 := waitEvent(t, events, EventTransition, 2*time.Second)
	if tr2.Alert {
		t.Fatal("expected alert=false on unhealthy→healthy")
	}

	cancel()
	<-errCh

	mu.Lock()
	defer mu.Unlock()
	if len(transitions) != 2 {
		t.Fatalf("expected exactly 2 OnTransition calls, got %d (%v)", len(transitions), transitions)
	}
	if !transitions[0] || transitions[1] {
		t.Fatalf("expected alert then resolve, got %v", transitions)
	}
	if tickCount < 3 {
		t.Fatalf("expected at least 3 ticks, got %d", tickCount)
	}
}

func TestDaemonCancelReturnsNil(t *testing.T) {
	cs := fake.NewSimpleClientset(&appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default"},
		Status:     appsv1.DeploymentStatus{Replicas: 1, ReadyReplicas: 0},
	})
	cfg := &config.Config{
		Resources: []string{"deployment.default/app"},
		Interval:  1,
		Timeout:   5 * time.Second,
	}
	clk := newStepClock()
	events := make(chan DaemonEvent, 8)
	var notifyCount int
	var mu sync.Mutex

	ctx, cancel := context.WithCancel(context.Background())
	eng := New(cfg, cs, check.NewRegistry())
	errCh := make(chan error, 1)
	go func() {
		errCh <- eng.Daemon(ctx, DaemonOptions{
			Clock:  clk,
			Events: events,
			OnTransition: func(_ context.Context, _ bool, _ []check.Verdict) error {
				mu.Lock()
				notifyCount++
				mu.Unlock()
				return nil
			},
		})
	}()

	// First tick unhealthy → one alert.
	waitEvent(t, events, EventPassCompleted, 2*time.Second)
	waitEvent(t, events, EventTransition, 2*time.Second)
	mu.Lock()
	before := notifyCount
	mu.Unlock()
	if before != 1 {
		t.Fatalf("expected 1 transition notify, got %d", before)
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("DaemonCancel: want nil, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Daemon did not return after cancel")
	}

	mu.Lock()
	after := notifyCount
	mu.Unlock()
	if after != before {
		t.Fatalf("notify count changed after cancel: before=%d after=%d", before, after)
	}
}

// discardChecker blocks until ctx is canceled, then returns Errored.
type discardChecker struct {
	started chan struct{}
	once    sync.Once
}

func (d *discardChecker) Check(ctx context.Context, _ kubernetes.Interface, ref check.Ref) check.Verdict {
	d.once.Do(func() { close(d.started) })
	<-ctx.Done()
	return check.Verdict{Ref: ref, Status: check.Errored, Reason: "canceled"}
}

func TestDiscardCanceled(t *testing.T) {
	started := make(chan struct{})
	reg := check.Registry{
		"deployment": &discardChecker{started: started},
	}
	cs := fake.NewSimpleClientset()
	cfg := &config.Config{
		Resources: []string{"deployment.default/app"},
		Interval:  1,
		Timeout:   5 * time.Second,
	}
	clk := newStepClock()
	var transitions int
	var mu sync.Mutex

	ctx, cancel := context.WithCancel(context.Background())
	eng := New(cfg, cs, reg)
	errCh := make(chan error, 1)
	go func() {
		errCh <- eng.Daemon(ctx, DaemonOptions{
			Clock: clk,
			OnTransition: func(_ context.Context, _ bool, _ []check.Verdict) error {
				mu.Lock()
				transitions++
				mu.Unlock()
				return nil
			},
		})
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("checker did not start")
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("DiscardCanceled: want nil, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Daemon did not return after mid-pass cancel")
	}

	mu.Lock()
	defer mu.Unlock()
	if transitions != 0 {
		t.Fatalf("cancelled pass must not transition, got %d", transitions)
	}
}
