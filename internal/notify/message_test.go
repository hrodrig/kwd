package notify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hrodrig/kwd/internal/check"
)

func TestMessageAlertFields(t *testing.T) {
	ts := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	verdicts := []check.Verdict{
		{Ref: check.Ref{Kind: "deployment", Namespace: "default", Name: "app"}, Status: check.NotReady, Reason: "0/1"},
		{Ref: check.Ref{Kind: "statefulset", Namespace: "default", Name: "db"}, Status: check.Ready, Reason: "1/1"},
	}
	m := BuildTransitionMessage("bastion-1", "prod-cluster", true, verdicts, ts)
	if m.Level != LevelAlert {
		t.Fatalf("Level = %q, want %q", m.Level, LevelAlert)
	}
	if m.Color != ColorAlert {
		t.Fatalf("Color = %q, want %q", m.Color, ColorAlert)
	}
	if m.Title == "" || !strings.Contains(strings.ToLower(m.Title), "not ready") {
		t.Fatalf("Title = %q, want alert title", m.Title)
	}
	if !strings.Contains(m.Body, "deployment.default/app") || !strings.Contains(m.Body, "statefulset.default/db") {
		t.Fatalf("Body missing per-resource status: %q", m.Body)
	}
	if !m.Timestamp.Equal(ts) {
		t.Fatalf("Timestamp = %v, want %v", m.Timestamp, ts)
	}
	if m.ClientID != "bastion-1" || m.Cluster != "prod-cluster" {
		t.Fatalf("ClientID/Cluster = %q/%q", m.ClientID, m.Cluster)
	}
	if m.Text == "" || !strings.Contains(m.Text, m.Title) {
		t.Fatalf("Text should be composed from SPEC fields: %q", m.Text)
	}
}

func TestMessageResolutionFields(t *testing.T) {
	ts := time.Date(2026, 10, 10, 12, 5, 0, 0, time.UTC)
	verdicts := []check.Verdict{
		{Ref: check.Ref{Kind: "deployment", Namespace: "default", Name: "app"}, Status: check.Ready, Reason: "1/1"},
	}
	m := BuildTransitionMessage("cid", "c1", false, verdicts, ts)
	if m.Level != LevelOK {
		t.Fatalf("Level = %q, want %q", m.Level, LevelOK)
	}
	if m.Color != ColorOK {
		t.Fatalf("Color = %q, want %q", m.Color, ColorOK)
	}
	if !strings.Contains(strings.ToLower(m.Title), "ready") {
		t.Fatalf("Title = %q, want resolution title", m.Title)
	}
	if strings.Contains(m.Title, "ongoing") {
		t.Fatalf("resolve title must not mark ongoing: %q", m.Title)
	}
	if !strings.Contains(m.Body, "deployment.default/app") {
		t.Fatalf("Body = %q", m.Body)
	}
	if m.Level == LevelAlert {
		t.Fatal("resolution must not use alert level")
	}
}

func TestMessageRepeatOngoingTitle(t *testing.T) {
	ts := time.Date(2026, 10, 10, 12, 10, 0, 0, time.UTC)
	verdicts := []check.Verdict{
		{Ref: check.Ref{Kind: "deployment", Namespace: "default", Name: "app"}, Status: check.NotReady},
	}
	m := BuildRepeatMessage("cid", "cl", verdicts, ts)
	if m.Level != LevelAlert {
		t.Fatalf("Level = %q, want alert", m.Level)
	}
	if !strings.Contains(m.Title, "ALERT (ongoing)") {
		t.Fatalf("Title = %q, want ALERT (ongoing) marker (D-16)", m.Title)
	}
	alert := BuildTransitionMessage("cid", "cl", true, verdicts, ts)
	if strings.Contains(alert.Title, "ongoing") {
		t.Fatalf("initial alert must not use ongoing title: %q", alert.Title)
	}
}

func TestComposeTextIncludesSPECFields(t *testing.T) {
	m := Message{
		ClientID:  "me",
		Cluster:   "cl",
		Level:     LevelAlert,
		Color:     ColorAlert,
		Title:     "kwd alert",
		Body:      "- deployment.default/app: NOT-READY",
		Timestamp: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
	}
	text := ComposeText(m)
	for _, want := range []string{"kwd alert", "deployment.default/app", "me", "cl", "2026-01-02T03:04:05Z", LevelAlert, ColorAlert} {
		if !strings.Contains(text, want) {
			t.Fatalf("ComposeText missing %q in %q", want, text)
		}
	}
}

func TestSlackSendUsesComposedText(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(b)
		gotBody = string(b)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	m := BuildTransitionMessage("cid", "cl", true, []check.Verdict{
		{Ref: check.Ref{Kind: "deployment", Namespace: "ns", Name: "x"}, Status: check.NotReady},
	}, time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC))

	s := &Slack{WebhookURL: srv.URL, Client: srv.Client()}
	if err := s.Send(context.Background(), m); err != nil {
		t.Fatalf("send: %v", err)
	}
	var payload map[string]string
	if err := json.Unmarshal([]byte(gotBody), &payload); err != nil {
		t.Fatalf("slack JSON: %v body=%q", err, gotBody)
	}
	if payload["text"] != m.Text {
		t.Fatalf("text field = %q, want composed Text %q", payload["text"], m.Text)
	}
	if strings.Contains(gotBody, "attachments") {
		t.Fatal("Slack must stay plain {\"text\":...} (no attachments)")
	}
}
