package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "kwd.yaml")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadValid(t *testing.T) {
	p := writeTempConfig(t, `
cluster:
  name: test
resources:
  - deployment.default/app
  - statefulset.default/db
interval: 0
`)
	cfg, err := Load(p)
	if err != nil {
		t.Fatalf("expected valid load, got %v", err)
	}
	if cfg.Cluster.Name != "test" {
		t.Fatalf("cluster name = %q", cfg.Cluster.Name)
	}
	if len(cfg.Resources) != 2 {
		t.Fatalf("resources = %d", len(cfg.Resources))
	}
	if cfg.Interval != 0 {
		t.Fatalf("interval = %d", cfg.Interval)
	}
	// Defaults applied.
	if cfg.Color != "auto" {
		t.Fatalf("color default = %q", cfg.Color)
	}
	if cfg.LogFormat != "text" {
		t.Fatalf("log_format default = %q", cfg.LogFormat)
	}
	if cfg.Retry.Attempts != 3 {
		t.Fatalf("retry.attempts default = %d", cfg.Retry.Attempts)
	}
	if cfg.ConfirmAlert != 1 || cfg.ConfirmOk != 1 {
		t.Fatalf("confirm defaults = %d/%d, want 1/1", cfg.ConfirmAlert, cfg.ConfirmOk)
	}
	if cfg.RepeatWhileFiring {
		t.Fatal("repeat_while_firing default must be false")
	}
	if cfg.HTTP.HealthPath != "/healthz" || cfg.HTTP.MetricsPath != "/metrics" {
		t.Fatalf("http path defaults = %q / %q", cfg.HTTP.HealthPath, cfg.HTTP.MetricsPath)
	}
	if cfg.HTTP.Listen != "" {
		t.Fatalf("http.listen default must be empty, got %q", cfg.HTTP.Listen)
	}
}

func TestLoadConfirmAndHTTPFields(t *testing.T) {
	p := writeTempConfig(t, `
resources:
  - deployment.default/app
confirm_alert: 3
confirm_ok: 2
repeat_while_firing: true
http:
  listen: ":9090"
  health_path: /readyz
  metrics_path: /prom
`)
	cfg, err := Load(p)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.ConfirmAlert != 3 || cfg.ConfirmOk != 2 || !cfg.RepeatWhileFiring {
		t.Fatalf("confirm/repeat = %d/%d/%v", cfg.ConfirmAlert, cfg.ConfirmOk, cfg.RepeatWhileFiring)
	}
	if cfg.HTTP.Listen != ":9090" || cfg.HTTP.HealthPath != "/readyz" || cfg.HTTP.MetricsPath != "/prom" {
		t.Fatalf("http = %+v", cfg.HTTP)
	}
}

func TestLoadRejectsConfirmAlertBelowOne(t *testing.T) {
	// applyDefaults turns 0 → 1; negative values hit validate.
	p := writeTempConfig(t, `
resources:
  - deployment.default/app
confirm_alert: -1
`)
	if _, err := Load(p); err == nil {
		t.Fatal("expected error for confirm_alert < 1")
	}
}

func TestLoadRejectsConfirmOkBelowOne(t *testing.T) {
	p := writeTempConfig(t, `
resources:
  - deployment.default/app
confirm_ok: -2
`)
	if _, err := Load(p); err == nil {
		t.Fatal("expected error for confirm_ok < 1")
	}
}

func TestLoadRejectsMissingResources(t *testing.T) {
	p := writeTempConfig(t, "cluster:\n  name: test\n")
	if _, err := Load(p); err == nil {
		t.Fatal("expected error for empty resources")
	}
}

func TestLoadRejectsBadKind(t *testing.T) {
	p := writeTempConfig(t, `
resources:
  - daemonset.default/x
`)
	if _, err := Load(p); err == nil {
		t.Fatal("expected error for unsupported kind in v0")
	}
}

func TestLoadRejectsBadRefShape(t *testing.T) {
	p := writeTempConfig(t, `
resources:
  - not-a-valid-ref
`)
	if _, err := Load(p); err == nil {
		t.Fatal("expected error for malformed ref")
	}
}

func TestLoadRejectsInitialBackoffGreaterThanMax(t *testing.T) {
	p := writeTempConfig(t, `
resources:
  - deployment.default/app
retry:
  initial_backoff: 10s
  max_backoff: 1s
`)
	if _, err := Load(p); err == nil {
		t.Fatal("expected error for initial_backoff > max_backoff")
	}
}

func TestLoadRejectsBadColor(t *testing.T) {
	p := writeTempConfig(t, `
resources:
  - deployment.default/app
color: neon
`)
	if _, err := Load(p); err == nil {
		t.Fatal("expected error for invalid color")
	}
}

func TestLoadRejectsNegativeInterval(t *testing.T) {
	p := writeTempConfig(t, `
resources:
  - deployment.default/app
interval: -1
`)
	if _, err := Load(p); err == nil {
		t.Fatal("expected error for interval < 0")
	}
}

func TestLoadAcceptsZeroInterval(t *testing.T) {
	p := writeTempConfig(t, `
resources:
  - deployment.default/app
interval: 0
`)
	cfg, err := Load(p)
	if err != nil {
		t.Fatalf("interval 0 should load: %v", err)
	}
	if cfg.Interval != 0 {
		t.Fatalf("interval = %d", cfg.Interval)
	}
}
