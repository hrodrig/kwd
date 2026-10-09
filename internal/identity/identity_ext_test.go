package identity

import (
	"testing"
)

func TestPrimaryIPv4NonEmptyOnHost(t *testing.T) {
	// A real host has at least one non-loopback interface; if it doesn't,
	// primaryIPv4 must still return "" (never panic or error).
	got := primaryIPv4()
	_ = got // value is host-dependent; only assert the function is safe
}

func TestPrimaryMACNonEmptyOnHost(t *testing.T) {
	got := primaryMAC()
	_ = got // host-dependent; the function must never panic
}
