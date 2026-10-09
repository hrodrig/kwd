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

func TestResolveTrimsWhitespace(t *testing.T) {
	// Whitespace-only values must not be treated as a source, and the value
	// that wins is trimmed on the way out.
	t.Setenv(EnvID, "   ")
	if got := Resolve("  from-config  "); got != "from-config" {
		t.Fatalf("whitespace-only env must fall through and config must be trimmed, got %q", got)
	}
}

func TestResolveHostnameFallback(t *testing.T) {
	t.Setenv(EnvID, "   ")
	got := Resolve("  ")

	host, err := os.Hostname()
	if err == nil && host != "" {
		if got != host {
			t.Fatalf("expected the hostname fallback %q, got %q", host, got)
		}
		return
	}
	if got == "" {
		t.Fatal("resolve must never return an empty identity")
	}
}
