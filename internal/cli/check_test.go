package cli

import (
	"bytes"
	"context"
	"testing"

	"github.com/hrodrig/kwd/internal/check"
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

func TestBuildAlertText(t *testing.T) {
	verdicts := []check.Verdict{
		{Ref: check.Ref{Kind: "deployment", Namespace: "default", Name: "app"}, Status: check.NotReady},
		{Ref: check.Ref{Kind: "statefulset", Namespace: "default", Name: "db"}, Status: check.Ready},
	}
	s := buildAlertText(verdicts)
	if s == "" {
		t.Fatal("alert text empty")
	}
	if !bytes.Contains([]byte(s), []byte("deployment.default/app")) {
		t.Fatalf("expected ref in alert text: %q", s)
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
