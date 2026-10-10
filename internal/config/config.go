// Package config loads and validates the kwd YAML configuration.
package config

import (
	"fmt"
	"strings"
	"time"
)

// Config is the root kwd configuration.
type Config struct {
	// Cluster holds "what is being watched" metadata (kzero-style).
	Cluster Cluster `yaml:"cluster" mapstructure:"cluster"`
	// Kube selects the Kubernetes target.
	Kube Kube `yaml:"kube" mapstructure:"kube"`
	// Client holds the watching identity ("who watches").
	Client Client `yaml:"client" mapstructure:"client"`
	// Resources is the declared set of kind.namespace/name references.
	Resources []string `yaml:"resources" mapstructure:"resources"`
	// Interval is the check interval in seconds; 0 = run once (single-pass).
	Interval int `yaml:"interval" mapstructure:"interval"`
	// Retry bounds the per-check retry backoff.
	Retry Retry `yaml:"retry" mapstructure:"retry"`
	// Timeout bounds a single check attempt.
	Timeout time.Duration `yaml:"timeout" mapstructure:"timeout"`
	// Color controls output coloring: auto, always, never.
	Color string `yaml:"color" mapstructure:"color"`
	// LogFormat controls engine log line format: text or json.
	LogFormat string `yaml:"log_format" mapstructure:"log_format"`
	// DryRun prints the verdict without sending notifications.
	DryRun bool `yaml:"dry_run" mapstructure:"dry_run"`
	// Notifications configures sinks; a nil Sinks means notifications disabled.
	Notifications *Notifications `yaml:"notifications" mapstructure:"notifications"`
	// HTTP is the optional daemon observability surface (listen empty = off).
	// Handlers are started in plan 03-02; schema/defaults land here.
	HTTP HTTPConfig `yaml:"http" mapstructure:"http"`
	// ConfirmAlert is consecutive unhealthy ticks before NotifyAlert (D-09).
	ConfirmAlert int `yaml:"confirm_alert" mapstructure:"confirm_alert"`
	// ConfirmOk is consecutive Ready ticks before NotifyResolve (D-09).
	ConfirmOk int `yaml:"confirm_ok" mapstructure:"confirm_ok"`
	// RepeatWhileFiring re-sends unhealthy alerts each completed gap (D-13).
	RepeatWhileFiring bool `yaml:"repeat_while_firing" mapstructure:"repeat_while_firing"`
}

// HTTPConfig holds listen address and probe/metrics paths (SPEC §5 / §9).
// http.health_path / http.metrics_path stay YAML-only this phase (D-20/D-21);
// --listen is registered in plan 03-02.
type HTTPConfig struct {
	Listen      string `yaml:"listen" mapstructure:"listen"`
	HealthPath  string `yaml:"health_path" mapstructure:"health_path"`
	MetricsPath string `yaml:"metrics_path" mapstructure:"metrics_path"`
}

// Cluster is "what is being watched" metadata.
type Cluster struct {
	Name        string `yaml:"name" mapstructure:"name"`
	Environment string `yaml:"environment" mapstructure:"environment"`
	Description string `yaml:"description" mapstructure:"description"`
}

// Kube selects the Kubernetes target by kubeconfig context.
type Kube struct {
	// Context is the kubeconfig context; empty means current-context.
	Context string `yaml:"context" mapstructure:"context"`
}

// Client is the watching identity. ID empty resolves via the cascade
// (env → hostname → IP → MAC → unknown).
type Client struct {
	ID string `yaml:"id" mapstructure:"id"`
}

// Retry bounds the per-check retry backoff (pgwd-style explicit bounds).
type Retry struct {
	Attempts       int           `yaml:"attempts" mapstructure:"attempts"`
	InitialBackoff time.Duration `yaml:"initial_backoff" mapstructure:"initial_backoff"`
	MaxBackoff     time.Duration `yaml:"max_backoff" mapstructure:"max_backoff"`
}

// Notifications holds the sink configuration.
type Notifications struct {
	Sinks []Sink `yaml:"sinks" mapstructure:"sinks"`
}

// Sink is a single notification destination (gghstats model).
type Sink struct {
	Type string `yaml:"type" mapstructure:"type"`
}

// validKinds is the set of supported readiness kinds in v0.
var validKinds = map[string]bool{
	"deployment":  true,
	"statefulset": true,
}

// validateResourceRef validates a single kind.namespace/name reference.
func validateResourceRef(ref string) error {
	parts := strings.SplitN(ref, ".", 2)
	if len(parts) != 2 {
		return fmt.Errorf("resource %q: expected kind.namespace/name", ref)
	}
	kind := parts[0]
	if !validKinds[kind] {
		return fmt.Errorf("resource %q: unsupported kind %q (v0 supports deployment, statefulset)", ref, kind)
	}
	tail := parts[1]
	slash := strings.Index(tail, "/")
	if slash <= 0 || slash == len(tail)-1 {
		return fmt.Errorf("resource %q: expected kind.namespace/name", ref)
	}
	ns, name := tail[:slash], tail[slash+1:]
	if ns == "" || name == "" {
		return fmt.Errorf("resource %q: namespace and name must be non-empty", ref)
	}
	return nil
}
