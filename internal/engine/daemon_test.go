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
			t.Fatalf("expected event %v, got %v (action=%v)", want, ev.Type, ev.Action)
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
		Resources:    []string{"deployment.default/app"},
		Interval:     1,
		Timeout:      5 * time.Second,
		ConfirmAlert: 1,
		ConfirmOk:    1,
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
	if !eng.Snapshot().HaveCompleted() {
		t.Fatal("snapshot must Store after completed tick")
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
		Resources:    []string{"deployment.default/app"},
		Interval:     1,
		Timeout:      5 * time.Second,
		ConfirmAlert: 1,
		ConfirmOk:    1,
	}
	clk := newStepClock()
	events := make(chan DaemonEvent, 16)

	var (
		mu      sync.Mutex
		actions []NotifyAction
		ticks   int
	)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eng := New(cfg, cs, check.NewRegistry())
	errCh := make(chan error, 1)
	go func() {
		errCh <- eng.Daemon(ctx, DaemonOptions{
			Clock:  clk,
			Events: events,
			OnTick: func(_ []check.Verdict, _ NotifyAction) {
				mu.Lock()
				ticks++
				mu.Unlock()
			},
			OnNotify: func(_ context.Context, action NotifyAction, _ []check.Verdict) error {
				mu.Lock()
				actions = append(actions, action)
				mu.Unlock()
				return nil
			},
		})
	}()

	// Tick 1: healthy baseline — no notify.
	waitEvent(t, events, EventPassCompleted, 2*time.Second)

	// Flip unhealthy.
	dep.Status.ReadyReplicas = 0
	if _, err := cs.AppsV1().Deployments("default").UpdateStatus(context.Background(), dep, metav1.UpdateOptions{}); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}
	clk.FireAfter()
	waitEvent(t, events, EventPassCompleted, 2*time.Second)
	tr := waitEvent(t, events, EventTransition, 2*time.Second)
	if !tr.Alert || tr.Action != NotifyAlert {
		t.Fatalf("expected Alert transition, got alert=%v action=%v", tr.Alert, tr.Action)
	}

	// Stable unhealthy tick — no extra transition.
	clk.FireAfter()
	waitEvent(t, events, EventPassCompleted, 2*time.Second)
	select {
	case ev := <-events:
		if ev.Type == EventTransition || ev.Type == EventRepeat {
			t.Fatalf("stable unhealthy tick must not notify, got %v", ev.Type)
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
	if tr2.Alert || tr2.Action != NotifyResolve {
		t.Fatalf("expected Resolve, got alert=%v action=%v", tr2.Alert, tr2.Action)
	}

	cancel()
	<-errCh

	mu.Lock()
	defer mu.Unlock()
	if len(actions) != 2 {
		t.Fatalf("expected exactly 2 OnNotify calls, got %d (%v)", len(actions), actions)
	}
	if actions[0] != NotifyAlert || actions[1] != NotifyResolve {
		t.Fatalf("expected Alert then Resolve, got %v", actions)
	}
	if ticks < 3 {
		t.Fatalf("expected at least 3 ticks, got %d", ticks)
	}
}

func TestDaemonConfirmAlert3DelaysNotify(t *testing.T) {
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default"},
		Status:     appsv1.DeploymentStatus{Replicas: 1, ReadyReplicas: 0},
	}
	cs := fake.NewSimpleClientset(dep)
	cfg := &config.Config{
		Resources:    []string{"deployment.default/app"},
		Interval:     1,
		Timeout:      5 * time.Second,
		ConfirmAlert: 3,
		ConfirmOk:    1,
	}
	clk := newStepClock()
	events := make(chan DaemonEvent, 16)
	var (
		mu      sync.Mutex
		actions []NotifyAction
	)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eng := New(cfg, cs, check.NewRegistry())
	errCh := make(chan error, 1)
	go func() {
		errCh <- eng.Daemon(ctx, DaemonOptions{
			Clock:  clk,
			Events: events,
			OnNotify: func(_ context.Context, action NotifyAction, _ []check.Verdict) error {
				mu.Lock()
				actions = append(actions, action)
				mu.Unlock()
				return nil
			},
		})
	}()

	waitEvent(t, events, EventPassCompleted, 2*time.Second) // tick 1
	clk.FireAfter()
	waitEvent(t, events, EventPassCompleted, 2*time.Second) // tick 2
	clk.FireAfter()
	waitEvent(t, events, EventPassCompleted, 2*time.Second) // tick 3
	tr := waitEvent(t, events, EventTransition, 2*time.Second)
	if tr.Action != NotifyAlert {
		t.Fatalf("tick3 action=%v, want Alert", tr.Action)
	}

	cancel()
	<-errCh

	mu.Lock()
	defer mu.Unlock()
	if len(actions) != 1 || actions[0] != NotifyAlert {
		t.Fatalf("confirm_alert=3: expected single Alert, got %v", actions)
	}
}

func TestDaemonRepeatWhileFiring(t *testing.T) {
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default"},
		Status:     appsv1.DeploymentStatus{Replicas: 1, ReadyReplicas: 0},
	}
	cs := fake.NewSimpleClientset(dep)
	cfg := &config.Config{
		Resources:         []string{"deployment.default/app"},
		Interval:          1,
		Timeout:           5 * time.Second,
		ConfirmAlert:      1,
		ConfirmOk:         1,
		RepeatWhileFiring: true,
	}
	clk := newStepClock()
	events := make(chan DaemonEvent, 16)
	var (
		mu      sync.Mutex
		actions []NotifyAction
	)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eng := New(cfg, cs, check.NewRegistry())
	errCh := make(chan error, 1)
	go func() {
		errCh <- eng.Daemon(ctx, DaemonOptions{
			Clock:  clk,
			Events: events,
			OnNotify: func(_ context.Context, action NotifyAction, _ []check.Verdict) error {
				mu.Lock()
				actions = append(actions, action)
				mu.Unlock()
				return nil
			},
		})
	}()

	waitEvent(t, events, EventPassCompleted, 2*time.Second)
	waitEvent(t, events, EventTransition, 2*time.Second) // Alert

	clk.FireAfter()
	waitEvent(t, events, EventPassCompleted, 2*time.Second)
	rep := waitEvent(t, events, EventRepeat, 2*time.Second)
	if rep.Action != NotifyRepeat {
		t.Fatalf("expected Repeat, got %v", rep.Action)
	}

	cancel()
	<-errCh

	mu.Lock()
	defer mu.Unlock()
	if len(actions) != 2 || actions[0] != NotifyAlert || actions[1] != NotifyRepeat {
		t.Fatalf("expected Alert then Repeat, got %v", actions)
	}
}

func TestDaemonCancelReturnsNil(t *testing.T) {
	cs := fake.NewSimpleClientset(&appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default"},
		Status:     appsv1.DeploymentStatus{Replicas: 1, ReadyReplicas: 0},
	})
	cfg := &config.Config{
		Resources:    []string{"deployment.default/app"},
		Interval:     1,
		Timeout:      5 * time.Second,
		ConfirmAlert: 1,
		ConfirmOk:    1,
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
			OnNotify: func(_ context.Context, _ NotifyAction, _ []check.Verdict) error {
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
		t.Fatalf("expected 1 notify, got %d", before)
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
		Resources:    []string{"deployment.default/app"},
		Interval:     1,
		Timeout:      5 * time.Second,
		ConfirmAlert: 1,
		ConfirmOk:    1,
	}
	clk := newStepClock()
	var notifies int
	var mu sync.Mutex

	ctx, cancel := context.WithCancel(context.Background())
	eng := New(cfg, cs, reg)
	errCh := make(chan error, 1)
	go func() {
		errCh <- eng.Daemon(ctx, DaemonOptions{
			Clock: clk,
			OnNotify: func(_ context.Context, _ NotifyAction, _ []check.Verdict) error {
				mu.Lock()
				notifies++
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
	if notifies != 0 {
		t.Fatalf("cancelled pass must not OnNotify, got %d", notifies)
	}
	if eng.Snapshot().HaveCompleted() {
		t.Fatal("cancelled pass must not Store snapshot")
	}
}
