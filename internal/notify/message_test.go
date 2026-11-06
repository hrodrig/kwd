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
	if !strings.Contains(m.Title, "EMERGENCY") {
		t.Fatalf("Title = %q, want EMERGENCY headline", m.Title)
	}
	if m.Namespace != "default" {
		t.Fatalf("Namespace = %q, want default", m.Namespace)
	}
	if m.Status != statusInconsistent || m.Action != actionImmediate {
		t.Fatalf("Status/Action = %q/%q", m.Status, m.Action)
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
	if m.Title != "HEALTH CHECK: All resources ready" {
		t.Fatalf("Title = %q, want HEALTH CHECK ready headline", m.Title)
	}
	if strings.Contains(m.Title, "ongoing") {
		t.Fatalf("resolve title must not mark ongoing: %q", m.Title)
	}
	if m.Status != statusAllReady || m.Action != actionNone {
		t.Fatalf("Status/Action = %q/%q", m.Status, m.Action)
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
	if !strings.Contains(m.Title, "EMERGENCY (ongoing)") {
		t.Fatalf("Title = %q, want EMERGENCY (ongoing) marker (D-16)", m.Title)
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

func TestSlackSendUsesAttachment(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(b)
		gotBody = string(b)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ts := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
	m := BuildTransitionMessage("cid", "cl", true, []check.Verdict{
		{Ref: check.Ref{Kind: "deployment", Namespace: "ns", Name: "x"}, Status: check.NotReady},
	}, ts)

	s := &Slack{WebhookURL: srv.URL, Client: srv.Client()}
	if err := s.Send(context.Background(), m); err != nil {
		t.Fatalf("send: %v", err)
	}
	var payload struct {
		Attachments []struct {
			Color  string `json:"color"`
			Title  string `json:"title"`
			Text   string `json:"text"`
			Footer string `json:"footer"`
			Ts     int64  `json:"ts"`
		} `json:"attachments"`
	}
	if err := json.Unmarshal([]byte(gotBody), &payload); err != nil {
		t.Fatalf("slack JSON: %v body=%q", err, gotBody)
	}
	if len(payload.Attachments) != 1 {
		t.Fatalf("attachments len = %d, want 1; body=%q", len(payload.Attachments), gotBody)
	}
	att := payload.Attachments[0]
	if att.Color != ColorAlert {
		t.Fatalf("color = %q, want %q", att.Color, ColorAlert)
	}
	if att.Title != "❌ Error" {
		t.Fatalf("title = %q", att.Title)
	}
	if att.Footer != "kwd" {
		t.Fatalf("footer = %q", att.Footer)
	}
	if att.Ts != ts.Unix() {
		t.Fatalf("ts = %d, want %d", att.Ts, ts.Unix())
	}
	for _, want := range []string{
		"Cluster", "Client", "cid", "cl", "Details", "deployment.ns/x",
		"Namespace", "`ns`", "Status", statusInconsistent, "Action Required", actionImmediate,
	} {
		if !strings.Contains(att.Text, want) {
			t.Fatalf("attachment text missing %q: %q", want, att.Text)
		}
	}
}

func TestBuildReportAndTestMessages(t *testing.T) {
	ts := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	verdicts := []check.Verdict{
		{Ref: check.Ref{Kind: "deployment", Namespace: "default", Name: "app"}, Status: check.Ready, Reason: "1/1"},
	}
	report := BuildReportMessage("cid", "cl", verdicts, ts)
	if report.Level != LevelOK || report.Color != ColorOK {
		t.Fatalf("report level/color = %s/%s", report.Level, report.Color)
	}
	if report.Title != "HEALTH CHECK: All resources ready" {
		t.Fatalf("report title = %q", report.Title)
	}
	if report.Status != statusAllReady || report.Action != actionNone {
		t.Fatalf("report Status/Action = %q/%q", report.Status, report.Action)
	}
	if AttachmentTitle(report) != "✅ Success" {
		t.Fatalf("attachment title = %q", AttachmentTitle(report))
	}

	testMsg := BuildTestMessage("cid", "cl", ts)
	if testMsg.Title != "HEALTH CHECK: notify test" {
		t.Fatalf("test title = %q", testMsg.Title)
	}
	if testMsg.Status != statusNotifyTest || testMsg.Action != actionNone {
		t.Fatalf("test Status/Action = %q/%q", testMsg.Status, testMsg.Action)
	}
	if testMsg.Namespace != "" {
		t.Fatalf("test Namespace = %q, want empty", testMsg.Namespace)
	}
	if !strings.Contains(testMsg.Body, "synthetic") {
		t.Fatalf("test body = %q", testMsg.Body)
	}
}

func TestComposeSlackTextStructured(t *testing.T) {
	m := Message{
		ClientID:  "me",
		Cluster:   "cl",
		Namespace: "default",
		Status:    statusInconsistent,
		Action:    actionImmediate,
		Level:     LevelAlert,
		Color:     ColorAlert,
		Title:     "EMERGENCY: 1 resource(s) not ready",
		Body:      "• ❌ *deployment.default/app*: NOT-READY",
		Timestamp: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
	}
	text := ComposeSlackText(m)
	for _, want := range []string{
		"*Cluster:*", "`cl`", "*Client:*", "`me`",
		"*Namespace:*", "`default`", "*Status:*", statusInconsistent,
		"*Action Required:*", actionImmediate,
		"Details", "deployment.default/app", "EMERGENCY",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("ComposeSlackText missing %q in %q", want, text)
		}
	}
}

func TestNamespacesFromVerdictsMulti(t *testing.T) {
	got := namespacesFromVerdicts([]check.Verdict{
		{Ref: check.Ref{Kind: "deployment", Namespace: "b", Name: "x"}},
		{Ref: check.Ref{Kind: "deployment", Namespace: "a", Name: "y"}},
		{Ref: check.Ref{Kind: "statefulset", Namespace: "a", Name: "z"}},
	})
	if got != "a, b" {
		t.Fatalf("namespaces = %q, want a, b", got)
	}
}
