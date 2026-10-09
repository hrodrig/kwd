package report

import (
	"bytes"
	"strings"
	"testing"

	"github.com/hrodrig/kwd/internal/check"
)

func TestStatusSymbol(t *testing.T) {
	cases := map[check.Status]string{
		check.Ready:    "READY",
		check.NotReady: "NOT-READY",
		check.Errored:  "ERRORED",
	}
	for s, want := range cases {
		if got := StatusSymbol(s); got != want {
			t.Fatalf("StatusSymbol(%s) = %q, want %q", s, got, want)
		}
	}
}

func TestWriteTableGroupsErroredFirst(t *testing.T) {
	verdicts := []check.Verdict{
		{Ref: check.Ref{Kind: "deployment", Namespace: "default", Name: "ok"}, Status: check.Ready, Reason: "ready"},
		{Ref: check.Ref{Kind: "deployment", Namespace: "default", Name: "bad"}, Status: check.Errored, Reason: "boom"},
		{Ref: check.Ref{Kind: "statefulset", Namespace: "default", Name: "db"}, Status: check.NotReady, Reason: "partial"},
	}
	var buf bytes.Buffer
	WriteTable(&buf, verdicts)

	out := buf.String()
	// Errored must come before NotReady before Ready.
	erroredIdx := strings.Index(out, "ERRORED")
	notReadyIdx := strings.Index(out, "NOT-READY")
	readyIdx := strings.Index(out, "READY")
	if !(erroredIdx < notReadyIdx && notReadyIdx < readyIdx) {
		t.Fatalf("ordering wrong:\n%s", out)
	}
}

func TestAnyNotReady(t *testing.T) {
	if AnyNotReady([]check.Verdict{{Status: check.Ready}, {Status: check.Ready}}) {
		t.Fatal("all-ready should be false")
	}
	if !AnyNotReady([]check.Verdict{{Status: check.Ready}, {Status: check.Errored}}) {
		t.Fatal("errored should count as not-ready")
	}
	if !AnyNotReady([]check.Verdict{{Status: check.NotReady}}) {
		t.Fatal("not-ready should be true")
	}
	if AnyNotReady(nil) {
		t.Fatal("empty should be false")
	}
}
