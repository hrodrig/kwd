package check

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// deploymentChecker checks apps/v1 Deployment readiness.
type deploymentChecker struct{}

// Check verifies a Deployment's readiness from its status.
//
// Ready conditions (SPECIFICATIONS.md §6):
//   - spec.replicas == 0 (scale-to-zero) is READY — a deliberate zero, not
//     failure.
//   - otherwise, at least spec.replicas pods must be ready. A Deployment whose
//     status has not observed any pod yet (fresh create, Recreate-strategy
//     rollout window, scale-up from zero) is NOT ready — status.replicas==0
//     means "nothing running", never "scaled to zero".
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

	return workloadVerdict(ref, desiredReplicas(d.Spec.Replicas), d.Status.Replicas, d.Status.ReadyReplicas)
}

// desiredReplicas returns spec.replicas, defaulting to 1 when unset (the API
// server defaults an unspecified replica count to 1).
func desiredReplicas(spec *int32) int32 {
	if spec == nil || *spec < 0 {
		return 1
	}
	return *spec
}

// workloadVerdict folds the replica-based readiness semantics shared by
// Deployment and StatefulSet.
//
// desired comes from spec.replicas (what the operator asked for); observed and
// ready come from status (what the cluster actually has). The distinction is
// load-bearing: comparing status against itself reports READY for a workload
// that wants replicas and has none, which is the exact window an operator is
// watching after a rollout trigger.
func workloadVerdict(ref Ref, desired, observed, ready int32) Verdict {
	// Scale-to-zero: no replicas desired is a valid, ready (idle) state.
	if desired == 0 {
		return Verdict{Ref: ref, Status: Ready, Reason: "scaled to zero (desired replicas 0)"}
	}
	// More ready pods than desired is still ready (a scale-down in progress,
	// e.g. desired 2 with 3 pods still terminating, has not lost capacity).
	if ready >= desired {
		return Verdict{Ref: ref, Status: Ready, Reason: fmt.Sprintf("%d/%d replicas ready", ready, desired)}
	}
	if observed == 0 {
		return Verdict{
			Ref:    ref,
			Status: NotReady,
			Reason: fmt.Sprintf("no pods observed yet (observed 0, ready 0, desired %d)", desired),
		}
	}
	return Verdict{
		Ref:    ref,
		Status: NotReady,
		Reason: fmt.Sprintf("not all replicas ready (ready %d, desired %d)", ready, desired),
	}
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
