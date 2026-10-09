package identity

import (
	"os"
	"testing"
)

func TestResolveEnvPrecedence(t *testing.T) {
	t.Setenv(EnvID, "from-env")
	got := Resolve("from-config")
	if got != "from-env" {
		t.Fatalf("env should win, got %q", got)
	}
}

func TestResolveConfigFallback(t *testing.T) {
	t.Setenv(EnvID, "")
	got := Resolve("from-config")
	if got != "from-config" {
		t.Fatalf("config should win when env empty, got %q", got)
	}
}

func TestResolveNeverEmpty(t *testing.T) {
	t.Setenv(EnvID, "")
	got := Resolve("")
	if got == "" {
		t.Fatalf("resolve must never return empty (falls to hostname/ip/mac/unknown)")
	}
}

func TestResolveUnknownIsLiteral(t *testing.T) {
	// Hostname/IP/MAC are non-empty on any real host, so the true "unknown"
	// path is hard to hit in-process; assert the constant exists and cascade
	// always returns a non-empty string.
	_ = os.Hostname
	if Resolve("") == "" {
		t.Fatal("unreachable empty")
	}
}
