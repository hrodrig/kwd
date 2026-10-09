// Package notify implements kwd notification sinks (gghstats-school). v0.1
// ships the Slack sink only; webhook/Loki/SMTP are v2.
package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// TypeSlack is the sink type identifier for Slack.
const TypeSlack = "slack"

// SlackWebhookEnv is the environment variable that must resolve for a Slack
// sink to be valid (fail-closed: never inline secrets).
const SlackWebhookEnv = "KWD_SLACK_WEBHOOK"

// Sender delivers one message to a destination.
type Sender interface {
	Send(ctx context.Context, msg Message) error
	Type() string
}

// Message is the notification payload, enriched with client+cluster metadata.
type Message struct {
	// ClientID is the resolved watching identity.
	ClientID string
	// Cluster is cluster.name ("what is watched").
	Cluster string
	// Text is the human-readable alert body.
	Text string
}

// Slack sends via Incoming Webhook (plain text).
type Slack struct {
	WebhookURL string
	Client     *http.Client
}

func (s *Slack) Type() string { return TypeSlack }

func (s *Slack) Send(ctx context.Context, m Message) error {
	raw, err := json.Marshal(map[string]string{"text": m.Text})
	if err != nil {
		return fmt.Errorf("slack payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.WebhookURL, strings.NewReader(string(raw)))
	if err != nil {
		return fmt.Errorf("slack request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := s.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("slack webhook: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("slack webhook: status %s", resp.Status)
	}
	return nil
}

// FanOut sends m to every sender, continuing past failures and joining errors
// (gghstats/kzero style). Returns nil only if all sinks succeeded.
func FanOut(ctx context.Context, senders []Sender, m Message) error {
	if len(senders) == 0 {
		return fmt.Errorf("no notification senders configured")
	}
	var errs []error
	for _, s := range senders {
		if err := s.Send(ctx, m); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", s.Type(), err))
		}
	}
	switch len(errs) {
	case 0:
		return nil
	case 1:
		return errs[0]
	default:
		msgs := make([]string, len(errs))
		for i, e := range errs {
			msgs[i] = e.Error()
		}
		return fmt.Errorf("%s", strings.Join(msgs, "; "))
	}
}
