package check

import (
	"context"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// int32p returns a pointer to v (replica counts are pointers in the k8s API).
func int32p(v int32) *int32 { return &v }

// checkDeployment builds a fake cluster holding one Deployment and checks it.
func checkDeployment(t *testing.T, d *appsv1.Deployment) Verdict {
	t.Helper()
	cs := fake.NewSimpleClientset(d)
	return NewRegistry().Check(context.Background(), cs, Ref{Kind: "deployment", Namespace: d.Namespace, Name: d.Name})
}

func TestDeploymentReady(t *testing.T) {
	v := checkDeployment(t, &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "ok", Namespace: "default"},
		Spec:       appsv1.DeploymentSpec{Replicas: int32p(3)},
		Status:     appsv1.DeploymentStatus{Replicas: 3, ReadyReplicas: 3},
	})
	if v.Status != Ready {
		t.Fatalf("expected ready, got %s (%s)", v.Status, v.Reason)
	}
}

func TestDeploymentNotReady(t *testing.T) {
	v := checkDeployment(t, &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "partial", Namespace: "default"},
		Spec:       appsv1.DeploymentSpec{Replicas: int32p(3)},
		Status:     appsv1.DeploymentStatus{Replicas: 3, ReadyReplicas: 1},
	})
	if v.Status != NotReady {
		t.Fatalf("expected not-ready, got %s (%s)", v.Status, v.Reason)
	}
	if !strings.Contains(v.Reason, "ready 1, desired 3") {
		t.Fatalf("reason should report ready/desired, got %q", v.Reason)
	}
}

func TestDeploymentScaleToZeroIsReady(t *testing.T) {
	v := checkDeployment(t, &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "stopped", Namespace: "default"},
		Spec:       appsv1.DeploymentSpec{Replicas: int32p(0)},
		Status:     appsv1.DeploymentStatus{Replicas: 0, ReadyReplicas: 0},
	})
	if v.Status != Ready {
		t.Fatalf("expected scale-to-zero to be ready, got %s (%s)", v.Status, v.Reason)
	}
}

// TestDeploymentRolloutWindowIsNotReady pins the regression: a Deployment that
// wants replicas but whose status has not observed any pod yet (fresh create,
// Recreate-strategy rollout, scale-up from zero) must NOT read as ready.
func TestDeploymentRolloutWindowIsNotReady(t *testing.T) {
	v := checkDeployment(t, &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "rolling", Namespace: "default"},
		Spec:       appsv1.DeploymentSpec{Replicas: int32p(3)},
		Status:     appsv1.DeploymentStatus{Replicas: 0, ReadyReplicas: 0},
	})
	if v.Status != NotReady {
		t.Fatalf("expected not-ready while no pod is observed, got %s (%s)", v.Status, v.Reason)
	}
	if !strings.Contains(v.Reason, "no pods observed") {
		t.Fatalf("reason should name the unobserved window, got %q", v.Reason)
	}
}

// TestDeploymentScaleDownInProgressIsReady: desired 2 with 3 pods still ready
// has not lost capacity — readiness must not flap during a scale-down.
func TestDeploymentScaleDownInProgressIsReady(t *testing.T) {
	v := checkDeployment(t, &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "shrinking", Namespace: "default"},
		Spec:       appsv1.DeploymentSpec{Replicas: int32p(2)},
		Status:     appsv1.DeploymentStatus{Replicas: 3, ReadyReplicas: 3},
	})
	if v.Status != Ready {
		t.Fatalf("expected ready during scale-down, got %s (%s)", v.Status, v.Reason)
	}
}

func TestDeploymentUnsetSpecDefaultsToOne(t *testing.T) {
	ready := checkDeployment(t, &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "defaulted", Namespace: "default"},
		Status:     appsv1.DeploymentStatus{Replicas: 1, ReadyReplicas: 1},
	})
	if ready.Status != Ready {
		t.Fatalf("unset spec.replicas with 1/1 ready should be ready, got %s (%s)", ready.Status, ready.Reason)
	}

	none := checkDeployment(t, &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "nothing", Namespace: "default"},
	})
	if none.Status != NotReady {
		t.Fatalf("unset spec.replicas with nothing observed should be not-ready, got %s", none.Status)
	}
}

func TestDeploymentAvailableFalse(t *testing.T) {
	v := checkDeployment(t, &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "degraded", Namespace: "default"},
		Spec:       appsv1.DeploymentSpec{Replicas: int32p(2)},
		Status: appsv1.DeploymentStatus{
			Replicas:      2,
			ReadyReplicas: 2,
			Conditions: []appsv1.DeploymentCondition{
				{Type: appsv1.DeploymentAvailable, Status: corev1.ConditionFalse, Message: "rollout failed"},
			},
		},
	})
	if v.Status != NotReady {
		t.Fatalf("expected Available=False to be not-ready, got %s", v.Status)
	}
}

func TestDeploymentNotFoundIsErrored(t *testing.T) {
	cs := fake.NewSimpleClientset()

	v := NewRegistry().Check(context.Background(), cs, Ref{Kind: "deployment", Namespace: "default", Name: "missing"})
	if v.Status != Errored {
		t.Fatalf("expected not-found to be errored, got %s", v.Status)
	}
}

func TestStatefulSetReady(t *testing.T) {
	cs := fake.NewSimpleClientset(&appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: "default"},
		Spec:       appsv1.StatefulSetSpec{Replicas: int32p(2)},
		Status:     appsv1.StatefulSetStatus{Replicas: 2, ReadyReplicas: 2},
	})

	v := NewRegistry().Check(context.Background(), cs, Ref{Kind: "statefulset", Namespace: "default", Name: "db"})
	if v.Status != Ready {
		t.Fatalf("expected ready, got %s (%s)", v.Status, v.Reason)
	}
}

func TestStatefulSetRolloutWindowIsNotReady(t *testing.T) {
	cs := fake.NewSimpleClientset(&appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: "default"},
		Spec:       appsv1.StatefulSetSpec{Replicas: int32p(2)},
		Status:     appsv1.StatefulSetStatus{Replicas: 0, ReadyReplicas: 0},
	})

	v := NewRegistry().Check(context.Background(), cs, Ref{Kind: "statefulset", Namespace: "default", Name: "db"})
	if v.Status != NotReady {
		t.Fatalf("expected not-ready while no pod is observed, got %s (%s)", v.Status, v.Reason)
	}
}

func TestStatefulSetScaleToZeroIsReady(t *testing.T) {
	cs := fake.NewSimpleClientset(&appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: "default"},
		Spec:       appsv1.StatefulSetSpec{Replicas: int32p(0)},
		Status:     appsv1.StatefulSetStatus{Replicas: 0, ReadyReplicas: 0},
	})

	v := NewRegistry().Check(context.Background(), cs, Ref{Kind: "statefulset", Namespace: "default", Name: "db"})
	if v.Status != Ready {
		t.Fatalf("expected scale-to-zero to be ready, got %s (%s)", v.Status, v.Reason)
	}
}

func TestDesiredReplicas(t *testing.T) {
	cases := []struct {
		name string
		spec *int32
		want int32
	}{
		{"unset defaults to 1", nil, 1},
		{"explicit zero", int32p(0), 0},
		{"explicit value", int32p(4), 4},
		{"negative is invalid, defaults to 1", int32p(-1), 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := desiredReplicas(tc.spec); got != tc.want {
				t.Fatalf("desiredReplicas(%v) = %d, want %d", tc.spec, got, tc.want)
			}
		})
	}
}

func TestUnknownKindIsErrored(t *testing.T) {
	cs := fake.NewSimpleClientset()

	v := NewRegistry().Check(context.Background(), cs, Ref{Kind: "daemonset", Namespace: "default", Name: "x"})
	if v.Status != Errored {
		t.Fatalf("expected unregistered kind to be errored, got %s", v.Status)
	}
}

func TestParseRef(t *testing.T) {
	r := ParseRef("deployment.default/my-app")
	if r.Kind != "deployment" || r.Namespace != "default" || r.Name != "my-app" {
		t.Fatalf("got %+v", r)
	}
	if r.String() != "deployment.default/my-app" {
		t.Fatalf("round-trip failed: %q", r.String())
	}
}
