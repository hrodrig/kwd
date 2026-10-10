package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/hrodrig/kwd/internal/config"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// int32p returns a pointer to v (replica counts are pointers in the k8s API).
func int32p(v int32) *int32 { return &v }

func TestCheckWithClientAllReady(t *testing.T) {
	cs := fake.NewSimpleClientset(&appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default"},
		Spec:       appsv1.DeploymentSpec{Replicas: int32p(1)},
		Status:     appsv1.DeploymentStatus{Replicas: 1, ReadyReplicas: 1},
	})
	cfg := &config.Config{Resources: []string{"deployment.default/app"}}

	var out, errOut bytes.Buffer
	err := checkWithClient(context.Background(), &out, &errOut, cfg, cs)
	if err != nil {
		t.Fatalf("all-ready should return nil, got %v", err)
	}
	if !bytes.Contains(out.Bytes(), []byte("READY")) {
		t.Fatalf("expected READY in output: %q", out.String())
	}
}

func TestCheckWithClientNotReady(t *testing.T) {
	cs := fake.NewSimpleClientset(&appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default"},
		Spec:       appsv1.DeploymentSpec{Replicas: int32p(3)},
		Status:     appsv1.DeploymentStatus{Replicas: 3, ReadyReplicas: 1},
	})
	cfg := &config.Config{Resources: []string{"deployment.default/app"}}

	var out, errOut bytes.Buffer
	err := checkWithClient(context.Background(), &out, &errOut, cfg, cs)
	if err == nil {
		t.Fatal("not-ready should return an error carrying exit code 1")
	}
	if !bytes.Contains(out.Bytes(), []byte("NOT-READY")) {
		t.Fatalf("expected NOT-READY in output: %q", out.String())
	}
}

func TestCheckWithClientDryRunSuppressesNotify(t *testing.T) {
	cs := fake.NewSimpleClientset(&appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default"},
		Spec:       appsv1.DeploymentSpec{Replicas: int32p(1)},
		Status:     appsv1.DeploymentStatus{Replicas: 1, ReadyReplicas: 0},
	})
	// Notifications configured but dry_run true — no env set, but dry-run must
	// not attempt a send (would otherwise fail-closed on missing webhook env).
	cfg := &config.Config{
		Resources: []string{"deployment.default/app"},
		DryRun:    true,
		Notifications: &config.Notifications{
			Sinks: []config.Sink{{Type: "slack"}},
		},
	}

	var out, errOut bytes.Buffer
	err := checkWithClient(context.Background(), &out, &errOut, cfg, cs)
	if err == nil {
		t.Fatal("not-ready should return error")
	}
	if bytes.Contains(errOut.Bytes(), []byte("notify:")) {
		t.Fatalf("dry-run must not notify: %q", errOut.String())
	}
}

func TestCheckWithClientFailClosedMissingWebhook(t *testing.T) {
	cs := fake.NewSimpleClientset(&appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default"},
		Spec:       appsv1.DeploymentSpec{Replicas: int32p(1)},
		Status:     appsv1.DeploymentStatus{Replicas: 1, ReadyReplicas: 0},
	})
	cfg := &config.Config{
		Resources: []string{"deployment.default/app"},
		Notifications: &config.Notifications{
			Sinks: []config.Sink{{Type: "slack"}},
		},
	}
	// No KWD_SLACK_WEBHOOK set -> fail-closed notify error on errOut.

	var out, errOut bytes.Buffer
	_ = checkWithClient(context.Background(), &out, &errOut, cfg, cs)
	if !bytes.Contains(errOut.Bytes(), []byte("fail-closed")) {
		t.Fatalf("expected fail-closed notify error, got: %q", errOut.String())
	}
}

func TestSendNotificationNilNotifications(t *testing.T) {
	cfg := &config.Config{} // nil Notifications
	if err := sendNotification(context.Background(), cfg, "cid", nil); err != nil {
		t.Fatalf("nil notifications should be a no-op, got %v", err)
	}
}

func TestSendNotificationUnsupportedSink(t *testing.T) {
	cfg := &config.Config{
		Notifications: &config.Notifications{
			Sinks: []config.Sink{{Type: "pagerduty"}}, // unsupported in v0.1
		},
	}
	err := sendNotification(context.Background(), cfg, "cid", nil)
	if err == nil {
		t.Fatal("unsupported sink type should error")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("not supported")) {
		t.Fatalf("expected 'not supported' error, got %v", err)
	}
}

func TestContextLabel(t *testing.T) {
	if got := contextLabel(&config.Config{}); got != "(current-context)" {
		t.Fatalf("empty context should be labeled current-context, got %q", got)
	}
	cfg := &config.Config{Kube: config.Kube{Context: "prod"}}
	if got := contextLabel(cfg); got != "prod" {
		t.Fatalf("expected context name, got %q", got)
	}
}

func TestSinkCount(t *testing.T) {
	if got := sinkCount(&config.Config{}); got != 0 {
		t.Fatalf("nil notifications -> 0, got %d", got)
	}
	cfg := &config.Config{Notifications: &config.Notifications{
		Sinks: []config.Sink{{Type: "slack"}, {Type: "slack"}},
	}}
	if got := sinkCount(cfg); got != 2 {
		t.Fatalf("expected 2 sinks, got %d", got)
	}
}

func TestIntervalFlagChangedOverridesConfig(t *testing.T) {
	root := NewRootCmd()
	checkCmd, _, err := root.Find([]string{"check"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		intervalFlag = 0
		if f := checkCmd.Flags().Lookup("interval"); f != nil {
			f.Changed = false
		}
	})
	if err := checkCmd.Flags().Set("interval", "30"); err != nil {
		t.Fatal(err)
	}
	if !checkCmd.Flags().Changed("interval") {
		t.Fatal("expected interval flag Changed after Set")
	}
	if got, _ := checkCmd.Flags().GetInt("interval"); got != 30 {
		t.Fatalf("interval flag = %d, want 30", got)
	}
	// D-01 apply path: Changed → override YAML/env value.
	cfg := &config.Config{Interval: 10}
	if checkCmd.Flags().Changed("interval") {
		cfg.Interval = intervalFlag
	}
	if cfg.Interval != 30 {
		t.Fatalf("Changed override: got %d, want 30", cfg.Interval)
	}
}

func TestIntervalFlagUnsetLeavesYAML(t *testing.T) {
	root := NewRootCmd()
	checkCmd, _, err := root.Find([]string{"check"})
	if err != nil {
		t.Fatal(err)
	}
	if checkCmd.Flags().Changed("interval") {
		t.Fatal("unset interval must not be Changed")
	}
}

func TestConfirmFlagsChangedOverridesConfig(t *testing.T) {
	root := NewRootCmd()
	checkCmd, _, err := root.Find([]string{"check"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		confirmAlertFlag = 1
		confirmOkFlag = 1
		repeatWhileFiringFlag = false
		for _, name := range []string{"confirm-alert", "confirm-ok", "repeat-while-firing"} {
			if f := checkCmd.Flags().Lookup(name); f != nil {
				f.Changed = false
			}
		}
	})
	if err := checkCmd.Flags().Set("confirm-alert", "5"); err != nil {
		t.Fatal(err)
	}
	if err := checkCmd.Flags().Set("confirm-ok", "4"); err != nil {
		t.Fatal(err)
	}
	if err := checkCmd.Flags().Set("repeat-while-firing", "true"); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{ConfirmAlert: 1, ConfirmOk: 1, RepeatWhileFiring: false}
	if checkCmd.Flags().Changed("confirm-alert") {
		cfg.ConfirmAlert = confirmAlertFlag
	}
	if checkCmd.Flags().Changed("confirm-ok") {
		cfg.ConfirmOk = confirmOkFlag
	}
	if checkCmd.Flags().Changed("repeat-while-firing") {
		cfg.RepeatWhileFiring = repeatWhileFiringFlag
	}
	if cfg.ConfirmAlert != 5 || cfg.ConfirmOk != 4 || !cfg.RepeatWhileFiring {
		t.Fatalf("Changed overrides = %+v", cfg)
	}
}

func TestConfirmFlagsUnsetLeaveYAML(t *testing.T) {
	root := NewRootCmd()
	checkCmd, _, err := root.Find([]string{"check"})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"confirm-alert", "confirm-ok", "repeat-while-firing"} {
		if checkCmd.Flags().Changed(name) {
			t.Fatalf("unset %s must not be Changed", name)
		}
		if checkCmd.Flags().Lookup(name) == nil {
			t.Fatalf("check must register --%s (D-20)", name)
		}
	}
	if checkCmd.Flags().Lookup("listen") != nil {
		t.Fatal("--listen must not be registered until plan 03-02")
	}
}

func TestNoDaemonFlag(t *testing.T) {
	root := NewRootCmd()
	if root.Flags().Lookup("daemon") != nil || root.PersistentFlags().Lookup("daemon") != nil {
		t.Fatal("root must not register --daemon (D-02)")
	}
	checkCmd, _, err := root.Find([]string{"check"})
	if err != nil {
		t.Fatal(err)
	}
	if checkCmd.Flags().Lookup("daemon") != nil {
		t.Fatal("check must not register --daemon (D-02)")
	}
	if checkCmd.Flags().Lookup("interval") == nil {
		t.Fatal("check must register --interval (D-01)")
	}
}

func TestCheckWithClientDaemonCancelNil(t *testing.T) {
	cs := fake.NewSimpleClientset(&appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default"},
		Spec:       appsv1.DeploymentSpec{Replicas: int32p(1)},
		Status:     appsv1.DeploymentStatus{Replicas: 1, ReadyReplicas: 1},
	})
	cfg := &config.Config{
		Resources: []string{"deployment.default/app"},
		Interval:  60, // large; we cancel before second tick
		Timeout:   2 * time.Second,
	}
	ctx, cancel := context.WithCancel(context.Background())
	var out, errOut bytes.Buffer
	errCh := make(chan error, 1)
	go func() {
		errCh <- checkWithClient(ctx, &out, &errOut, cfg, cs)
	}()
	// First pass prints table on transition only when unhealthy; healthy
	// baseline is silent. Give the first Once a moment, then cancel.
	time.Sleep(50 * time.Millisecond)
	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("daemon cancel must return nil, got %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("checkWithClient daemon did not exit on cancel")
	}
}

func TestDaemonTableOnlyOnTransition(t *testing.T) {
	// Stay not-ready across two ticks: table once (first-tick alert), not every tick.
	cs := fake.NewSimpleClientset(&appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default"},
		Spec:       appsv1.DeploymentSpec{Replicas: int32p(1)},
		Status:     appsv1.DeploymentStatus{Replicas: 1, ReadyReplicas: 0},
	})
	cfg := &config.Config{
		Resources: []string{"deployment.default/app"},
		Interval:  1,
		Timeout:   2 * time.Second,
		LogFormat: "text",
	}
	ctx, cancel := context.WithCancel(context.Background())
	var out, errOut bytes.Buffer
	errCh := make(chan error, 1)
	go func() {
		errCh <- checkWithClient(ctx, &out, &errOut, cfg, cs)
	}()
	// Allow first pass + serial gap + second stable pass.
	time.Sleep(1500 * time.Millisecond)
	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("daemon cancel must return nil, got %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("daemon did not exit")
	}
	tableHits := strings.Count(out.String(), "NOT-READY")
	if tableHits != 1 {
		t.Fatalf("table on transition only: NOT-READY count=%d out=%q", tableHits, out.String())
	}
	if !strings.Contains(errOut.String(), "kwd: tick") {
		t.Fatalf("expected tick log on errOut: %q", errOut.String())
	}
}

func TestDaemonDryRunSuppressesNotify(t *testing.T) {
	cs := fake.NewSimpleClientset(&appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default"},
		Spec:       appsv1.DeploymentSpec{Replicas: int32p(1)},
		Status:     appsv1.DeploymentStatus{Replicas: 1, ReadyReplicas: 0},
	})
	cfg := &config.Config{
		Resources: []string{"deployment.default/app"},
		Interval:  60,
		Timeout:   2 * time.Second,
		DryRun:    true,
		Notifications: &config.Notifications{
			Sinks: []config.Sink{{Type: "slack"}},
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	var out, errOut bytes.Buffer
	errCh := make(chan error, 1)
	go func() {
		errCh <- checkWithClient(ctx, &out, &errOut, cfg, cs)
	}()
	time.Sleep(80 * time.Millisecond)
	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("daemon cancel must return nil, got %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("daemon did not exit")
	}
	if bytes.Contains(errOut.Bytes(), []byte("notify:")) {
		t.Fatalf("dry-run must not FanOut: %q", errOut.String())
	}
	if !bytes.Contains(out.Bytes(), []byte("NOT-READY")) {
		t.Fatalf("dry-run still prints table on transition: %q", out.String())
	}
}

func TestDaemonRepeatDryRunPrintsTableMutesFanOut(t *testing.T) {
	cs := fake.NewSimpleClientset(&appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default"},
		Spec:       appsv1.DeploymentSpec{Replicas: int32p(1)},
		Status:     appsv1.DeploymentStatus{Replicas: 1, ReadyReplicas: 0},
	})
	cfg := &config.Config{
		Resources:         []string{"deployment.default/app"},
		Interval:          1,
		Timeout:           2 * time.Second,
		ConfirmAlert:      1,
		ConfirmOk:         1,
		RepeatWhileFiring: true,
		DryRun:            true,
		Notifications: &config.Notifications{
			Sinks: []config.Sink{{Type: "slack"}},
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	var out, errOut bytes.Buffer
	errCh := make(chan error, 1)
	go func() {
		errCh <- checkWithClient(ctx, &out, &errOut, cfg, cs)
	}()
	// First Alert + at least one Repeat gap.
	time.Sleep(2200 * time.Millisecond)
	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("daemon cancel must return nil, got %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("daemon did not exit")
	}
	if bytes.Contains(errOut.Bytes(), []byte("notify:")) {
		t.Fatalf("dry-run must mute Alert/Repeat FanOut: %q", errOut.String())
	}
	tableHits := strings.Count(out.String(), "NOT-READY")
	if tableHits < 2 {
		t.Fatalf("table on Alert and Repeat: NOT-READY count=%d out=%q", tableHits, out.String())
	}
}

func TestDaemonHealthyBaselineNoTable(t *testing.T) {
	cs := fake.NewSimpleClientset(&appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default"},
		Spec:       appsv1.DeploymentSpec{Replicas: int32p(1)},
		Status:     appsv1.DeploymentStatus{Replicas: 1, ReadyReplicas: 1},
	})
	cfg := &config.Config{
		Resources: []string{"deployment.default/app"},
		Interval:  60,
		Timeout:   2 * time.Second,
	}
	ctx, cancel := context.WithCancel(context.Background())
	var out, errOut bytes.Buffer
	errCh := make(chan error, 1)
	go func() {
		errCh <- checkWithClient(ctx, &out, &errOut, cfg, cs)
	}()
	time.Sleep(80 * time.Millisecond)
	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("got %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout")
	}
	if out.Len() != 0 {
		t.Fatalf("healthy baseline must not print table: %q", out.String())
	}
}
