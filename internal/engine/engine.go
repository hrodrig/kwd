// Package engine drives the kwd check. It is the single place that knows the
// configured resources and the cluster client. Daemon (interval > 0) layers a
// serial forever loop on Once with notify hysteresis and a TickSnapshot for
// future HTTP readers. The check itself lives in internal/check — Once only
// runs the registry over the refs (same path for single-pass and daemon).
package engine

import (
	"context"

	"github.com/hrodrig/kwd/internal/check"
	"github.com/hrodrig/kwd/internal/config"
	"k8s.io/client-go/kubernetes"
)

// Engine bundles the resolved config, clientset, and checker registry.
type Engine struct {
	cfg      *config.Config
	cs       kubernetes.Interface
	checks   check.Registry
	snapshot TickSnapshot
}

// New builds an Engine.
func New(cfg *config.Config, cs kubernetes.Interface, registry check.Registry) *Engine {
	return &Engine{cfg: cfg, cs: cs, checks: registry}
}

// Snapshot returns the last-completed tick store (HTTP readers in plan 03-02).
func (e *Engine) Snapshot() *TickSnapshot {
	return &e.snapshot
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
