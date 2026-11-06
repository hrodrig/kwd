package notify

import (
	"fmt"
	"strings"
	"time"

	"github.com/hrodrig/kwd/internal/check"
)

// SPEC §8 level / color for alert vs resolution.
const (
	LevelAlert = "alert"
	LevelOK    = "ok"
	ColorAlert = "#E01E5A" // red — unhealthy
	ColorOK    = "#36a64f" // green — healthy
)

// Message is the notification payload (SPEC §8 + client/cluster metadata).
type Message struct {
	ClientID  string
	Cluster   string
	Level     string
	Color     string
	Title     string
	Body      string
	Timestamp time.Time
	// Text is a plain fallback composition (ComposeText); Slack uses attachments.
	Text string
}

// BuildTransitionMessage builds an alert (alert=true) or resolution payload
// with SPEC-full fields and composed Text.
func BuildTransitionMessage(clientID, cluster string, alert bool, verdicts []check.Verdict, ts time.Time) Message {
	return buildTransitionMessage(clientID, cluster, alert, false, verdicts, ts)
}

// BuildRepeatMessage builds an ongoing unhealthy alert (D-16: ALERT (ongoing)).
// Resolve titles are unchanged — use BuildTransitionMessage(..., false, ...).
func BuildRepeatMessage(clientID, cluster string, verdicts []check.Verdict, ts time.Time) Message {
	return buildTransitionMessage(clientID, cluster, true, true, verdicts, ts)
}

// BuildReportMessage builds a scheduled healthy digest (--force-notification).
func BuildReportMessage(clientID, cluster string, verdicts []check.Verdict, ts time.Time) Message {
	m := buildTransitionMessage(clientID, cluster, false, false, verdicts, ts)
	m.Title = "kwd report: all resources ready"
	m.Text = ComposeText(m)
	return m
}

// BuildTestMessage builds a synthetic payload for `kwd notify test` (no check).
func BuildTestMessage(clientID, cluster string, ts time.Time) Message {
	if ts.IsZero() {
		ts = time.Now().UTC()
	} else {
		ts = ts.UTC()
	}
	m := Message{
		ClientID:  clientID,
		Cluster:   cluster,
		Level:     LevelOK,
		Color:     ColorOK,
		Title:     "kwd notify test",
		Body:      "• synthetic test — no readiness check ran",
		Timestamp: ts,
	}
	m.Text = ComposeText(m)
	return m
}

func buildTransitionMessage(clientID, cluster string, alert, ongoing bool, verdicts []check.Verdict, ts time.Time) Message {
	if ts.IsZero() {
		ts = time.Now().UTC()
	} else {
		ts = ts.UTC()
	}
	m := Message{
		ClientID:  clientID,
		Cluster:   cluster,
		Body:      formatVerdictBody(verdicts),
		Timestamp: ts,
	}
	if alert {
		m.Level = LevelAlert
		m.Color = ColorAlert
		m.Title = alertTitle(verdicts, ongoing)
	} else {
		m.Level = LevelOK
		m.Color = ColorOK
		m.Title = "kwd: all resources ready"
	}
	m.Text = ComposeText(m)
	return m
}

// ComposeText encodes SPEC fields into a plain-text fallback body.
func ComposeText(m Message) string {
	ts := m.Timestamp.UTC().Format(time.RFC3339)
	var b strings.Builder
	b.WriteString(m.Title)
	b.WriteByte('\n')
	if m.Body != "" {
		b.WriteString(m.Body)
		b.WriteByte('\n')
	}
	fmt.Fprintf(&b, "level=%s color=%s client.id=%s cluster=%s ts=%s",
		m.Level, m.Color, m.ClientID, m.Cluster, ts)
	return b.String()
}

// ComposeSlackText builds Incoming Webhook attachment mrkdwn (structured fields).
func ComposeSlackText(m Message) string {
	var b strings.Builder
	fmt.Fprintf(&b, "*%s*\n", m.Title)
	fmt.Fprintf(&b, "• *Cluster:* `%s`\n", emptyDash(m.Cluster))
	fmt.Fprintf(&b, "• *Client:* `%s`\n", emptyDash(m.ClientID))
	fmt.Fprintf(&b, "• *Time:* `%s`\n", m.Timestamp.UTC().Format("2006-01-02 15:04:05 UTC"))
	if m.Body != "" {
		b.WriteString("\n*Details:*\n")
		b.WriteString(m.Body)
	}
	return b.String()
}

// AttachmentTitle is the short Slack attachment header (color bar companion).
func AttachmentTitle(m Message) string {
	if m.Level == LevelAlert {
		return "❌ Alert"
	}
	return "✅ Success"
}

func emptyDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func alertTitle(verdicts []check.Verdict, ongoing bool) string {
	notReady := 0
	for _, v := range verdicts {
		if v.Status != check.Ready {
			notReady++
		}
	}
	if ongoing {
		return fmt.Sprintf("kwd ALERT (ongoing): %d resource(s) not ready", notReady)
	}
	return fmt.Sprintf("kwd: %d resource(s) not ready", notReady)
}

func formatVerdictBody(verdicts []check.Verdict) string {
	if len(verdicts) == 0 {
		return ""
	}
	lines := make([]string, 0, len(verdicts))
	for _, v := range verdicts {
		mark := "❌"
		if v.Status == check.Ready {
			mark = "✅"
		}
		status := strings.ToUpper(string(v.Status))
		line := fmt.Sprintf("• %s *%s*: %s", mark, v.Ref.String(), status)
		if v.Reason != "" {
			line += " (" + v.Reason + ")"
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}
