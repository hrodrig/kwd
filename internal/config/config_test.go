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
