package check

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// statefulsetChecker checks apps/v1 StatefulSet readiness (same semantics as
// Deployment: readyReplicas == replicas, replicas==0 = scale-to-zero ready).
type statefulsetChecker struct{}

func (statefulsetChecker) Check(ctx context.Context, cs kubernetes.Interface, ref Ref) Verdict {
	s, err := cs.AppsV1().StatefulSets(ref.Namespace).Get(ctx, ref.Name, metav1.GetOptions{})
	if err != nil {
		return verdictFromError(ref, err)
	}
	return workloadVerdict(ref, s.Status.Replicas, s.Status.ReadyReplicas)
}

// NewRegistry builds the default checker registry for the supported kinds.
func NewRegistry() Registry {
	return Registry{
		"deployment":  deploymentChecker{},
		"statefulset": statefulsetChecker{},
	}
}
