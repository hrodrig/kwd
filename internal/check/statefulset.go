package check

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// statefulsetChecker checks apps/v1 StatefulSet readiness. Same semantics as
// Deployment: readiness is judged against spec.replicas (desired), so
// spec.replicas == 0 is a deliberate scale-to-zero and a StatefulSet that
// desires replicas but has observed none is not-ready.
type statefulsetChecker struct{}

func (statefulsetChecker) Check(ctx context.Context, cs kubernetes.Interface, ref Ref) Verdict {
	s, err := cs.AppsV1().StatefulSets(ref.Namespace).Get(ctx, ref.Name, metav1.GetOptions{})
	if err != nil {
		return verdictFromError(ref, err)
	}
	return workloadVerdict(ref, desiredReplicas(s.Spec.Replicas), s.Status.Replicas, s.Status.ReadyReplicas)
}

// NewRegistry builds the default checker registry for the supported kinds.
func NewRegistry() Registry {
	return Registry{
		"deployment":  deploymentChecker{},
		"statefulset": statefulsetChecker{},
	}
}
