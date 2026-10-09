package check

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// deploymentChecker checks apps/v1 Deployment readiness.
type deploymentChecker struct{}

// Check verifies a Deployment's readiness from its status.
//
// Ready conditions (SPECIFICATIONS.md §6):
//   - readyReplicas == replicas && replicas > 0, with no Available=False
//   - replicas == 0 (scale-to-zero) is READY — a deliberate zero, not failure.
func (deploymentChecker) Check(ctx context.Context, cs kubernetes.Interface, ref Ref) Verdict {
	d, err := cs.AppsV1().Deployments(ref.Namespace).Get(ctx, ref.Name, metav1.GetOptions{})
	if err != nil {
		return verdictFromError(ref, err)
	}

	// Check for an Available=False condition (a rollout that is progressing or
	// failing reports this even while some replicas may already be ready).
	for _, c := range d.Status.Conditions {
		if c.Type == "Available" && c.Status == "False" {
			return Verdict{
				Ref:    ref,
				Status: NotReady,
				Reason: "Available=False: " + c.Message,
			}
		}
	}

	return workloadVerdict(ref, d.Status.Replicas, d.Status.ReadyReplicas)
}

// workloadVerdict folds common replica-based semantics shared by Deployment
// and StatefulSet.
func workloadVerdict(ref Ref, replicas, readyReplicas int32) Verdict {
	// Scale-to-zero: no replicas desired is a valid, ready (idle) state.
	if replicas == 0 {
		return Verdict{Ref: ref, Status: Ready, Reason: "scaled to zero"}
	}
	if readyReplicas != replicas {
		return Verdict{
			Ref:    ref,
			Status: NotReady,
			Reason: "not all replicas ready",
		}
	}
	return Verdict{Ref: ref, Status: Ready, Reason: "ready"}
}

// verdictFromError maps API errors to an Errored verdict (distinct from
// NotReady — an error is a check failure, not a resource health state).
func verdictFromError(ref Ref, err error) Verdict {
	switch {
	case apierrors.IsNotFound(err):
		return Verdict{Ref: ref, Status: Errored, Reason: "not found"}
	case apierrors.IsForbidden(err):
		return Verdict{Ref: ref, Status: Errored, Reason: "forbidden (check RBAC, including /status subresource)"}
	default:
		return Verdict{Ref: ref, Status: Errored, Reason: err.Error()}
	}
}
