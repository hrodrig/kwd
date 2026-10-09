package engine

import (
	"context"
	"testing"

	"github.com/hrodrig/kwd/internal/check"
	"github.com/hrodrig/kwd/internal/config"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestOnceRunsEveryResource(t *testing.T) {
	cs := fake.NewSimpleClientset(
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "a", Namespace: "default"},
			Status:     appsv1.DeploymentStatus{Replicas: 1, ReadyReplicas: 1},
		},
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "b", Namespace: "default"},
			Status:     appsv1.DeploymentStatus{Replicas: 1, ReadyReplicas: 0},
		},
	)

	cfg := &config.Config{
		Resources: []string{"deployment.default/a", "deployment.default/b"},
	}
	eng := New(cfg, cs, check.NewRegistry())

	verdicts := eng.Once(context.Background())
	if len(verdicts) != 2 {
		t.Fatalf("expected 2 verdicts, got %d", len(verdicts))
	}
	if verdicts[0].Status != check.Ready {
		t.Fatalf("resource a should be ready, got %s", verdicts[0].Status)
	}
	if verdicts[1].Status != check.NotReady {
		t.Fatalf("resource b should be not-ready, got %s", verdicts[1].Status)
	}
}
