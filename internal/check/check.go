// Package check implements per-kind readiness checks. It is PURE: a checker
// reads Kubernetes state and returns a Verdict. It knows nothing about ticks,
// hysteresis, notifications, or HTTP — the engine drives it.
package check

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/client-go/kubernetes"
)

// Status is the outcome of a single resource check.
type Status string

const (
	// Ready means the resource is healthy.
	Ready Status = "ready"
	// NotReady means the resource exists but is not healthy.
	NotReady Status = "not-ready"
	// Errored means the check itself failed (API error, RBAC denied, etc.)
	// — distinct from a resource that is simply not ready.
	Errored Status = "errored"
)

// Ref identifies a single resource as kind/namespace/name.
type Ref struct {
	Kind      string
	Namespace string
	Name      string
}

// Verdict is the result of checking one resource.
type Verdict struct {
	Ref    Ref
	Status Status
	// Reason is a human-readable explanation (condition, error, etc.).
	Reason string
}

// String renders the canonical kind.namespace/name reference.
func (r Ref) String() string {
	return fmt.Sprintf("%s.%s/%s", r.Kind, r.Namespace, r.Name)
}

// ParseRef parses a kind.namespace/name reference. The caller has already
// shape-validated it (config.validateResourceRef); this splits the pieces.
func ParseRef(s string) Ref {
	kind, tail, _ := strings.Cut(s, ".")
	ns, name, _ := strings.Cut(tail, "/")
	return Ref{Kind: kind, Namespace: ns, Name: name}
}

// Checker performs a readiness check for one kind against the cluster.
type Checker interface {
	// Check returns the verdict for a single resource. It MUST read /status
	// subresources (never a zero-valued bare Get).
	Check(ctx context.Context, clientset kubernetes.Interface, ref Ref) Verdict
}

// Registry maps a resource kind to its Checker. Kinds absent from the map
// are rejected at config load (see config.validateResourceRef).
type Registry map[string]Checker

// Check runs a single resource check through the registry.
func (r Registry) Check(ctx context.Context, cs kubernetes.Interface, ref Ref) Verdict {
	ck, ok := r[ref.Kind]
	if !ok {
		return Verdict{Ref: ref, Status: Errored, Reason: "no checker registered for kind"}
	}
	return ck.Check(ctx, cs, ref)
}
