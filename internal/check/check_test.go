package check

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestDeploymentReady(t *testing.T) {
	cs := fake.NewSimpleClientset(&appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "ok", Namespace: "default"},
		Status: appsv1.DeploymentStatus{
			Replicas:      3,
			ReadyReplicas: 3,
		},
	})

	reg := NewRegistry()
	v := reg.Check(context.Background(), cs, Ref{Kind: "deployment", Namespace: "default", Name: "ok"})
	if v.Status != Ready {
		t.Fatalf("expected ready, got %s (%s)", v.Status, v.Reason)
	}
}

func TestDeploymentNotReady(t *testing.T) {
	cs := fake.NewSimpleClientset(&appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "partial", Namespace: "default"},
		Status: appsv1.DeploymentStatus{
			Replicas:      3,
			ReadyReplicas: 1,
		},
	})

	v := NewRegistry().Check(context.Background(), cs, Ref{Kind: "deployment", Namespace: "default", Name: "partial"})
	if v.Status != NotReady {
		t.Fatalf("expected not-ready, got %s", v.Status)
	}
}

func TestDeploymentScaleToZeroIsReady(t *testing.T) {
	cs := fake.NewSimpleClientset(&appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "stopped", Namespace: "default"},
		Status: appsv1.DeploymentStatus{
			Replicas:      0,
			ReadyReplicas: 0,
		},
	})

	v := NewRegistry().Check(context.Background(), cs, Ref{Kind: "deployment", Namespace: "default", Name: "stopped"})
	if v.Status != Ready {
		t.Fatalf("expected scale-to-zero to be ready, got %s (%s)", v.Status, v.Reason)
	}
}

func TestDeploymentAvailableFalse(t *testing.T) {
	cs := fake.NewSimpleClientset(&appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "degraded", Namespace: "default"},
		Status: appsv1.DeploymentStatus{
			Replicas:      2,
			ReadyReplicas: 2,
			Conditions: []appsv1.DeploymentCondition{
				{Type: appsv1.DeploymentAvailable, Status: corev1.ConditionFalse, Message: "rollout failed"},
			},
		},
	})

	v := NewRegistry().Check(context.Background(), cs, Ref{Kind: "deployment", Namespace: "default", Name: "degraded"})
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
		Status: appsv1.StatefulSetStatus{
			Replicas:      2,
			ReadyReplicas: 2,
		},
	})

	v := NewRegistry().Check(context.Background(), cs, Ref{Kind: "statefulset", Namespace: "default", Name: "db"})
	if v.Status != Ready {
		t.Fatalf("expected ready, got %s (%s)", v.Status, v.Reason)
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
