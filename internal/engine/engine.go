// Package engine drives the kwd check. It is the single place that knows the
// configured resources and the cluster client; it does NOT know about ticks,
// hysteresis, or HTTP (those are v2, layered on Run.Daemon). The check itself
// lives in internal/check — the engine just runs the registry over the refs.
package engine

import (
	"context"

	"github.com/hrodrig/kwd/internal/check"
	"github.com/hrodrig/kwd/internal/config"
	"k8s.io/client-go/kubernetes"
)

// Engine bundles the resolved config, clientset, and checker registry.
type Engine struct {
	cfg    *config.Config
	cs     kubernetes.Interface
	checks check.Registry
}

// New builds an Engine.
func New(cfg *config.Config, cs kubernetes.Interface, registry check.Registry) *Engine {
	return &Engine{cfg: cfg, cs: cs, checks: registry}
}

// Once runs a single check pass over every declared resource and returns the
// verdicts (one per resource, in config order).
func (e *Engine) Once(ctx context.Context) []check.Verdict {
	verdicts := make([]check.Verdict, 0, len(e.cfg.Resources))
	for _, ref := range e.cfg.Resources {
		r := check.ParseRef(ref)
		verdicts = append(verdicts, e.checks.Check(ctx, e.cs, r))
	}
	return verdicts
}
