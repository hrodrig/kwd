package notify

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSlackSend(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(b)
		gotBody = string(b)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := &Slack{WebhookURL: srv.URL, Client: srv.Client()}
	err := s.Send(context.Background(), Message{ClientID: "me", Cluster: "test", Text: "alert"})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if gotBody == "" || gotBody == "\"\"" {
		t.Fatalf("expected JSON body, got %q", gotBody)
	}
}

func TestSlackSendNon2xxIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	s := &Slack{WebhookURL: srv.URL, Client: srv.Client()}
	if err := s.Send(context.Background(), Message{Text: "x"}); err == nil {
		t.Fatal("expected error on non-2xx")
	}
}

func TestFanOutContinuesPastFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer good.Close()

	senders := []Sender{
		&Slack{WebhookURL: srv.URL, Client: srv.Client()},
		&Slack{WebhookURL: good.URL, Client: good.Client()},
	}
	err := FanOut(context.Background(), senders, Message{Text: "x"})
	// FanOut joins errors: one failure means non-nil, but the good one still ran.
	if err == nil {
		t.Fatal("expected joined error (one sink failed)")
	}
}

func TestFanOutEmptySendersError(t *testing.T) {
	if err := FanOut(context.Background(), nil, Message{}); err == nil {
		t.Fatal("expected error for empty senders")
	}
}

func TestSlackTypeIsSlack(t *testing.T) {
	if got := (&Slack{}).Type(); got != TypeSlack {
		t.Fatalf("Type() = %q, want %q", got, TypeSlack)
	}
}

func TestSlackSendUsesDefaultClientWhenNil(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// A nil Client must fall back to http.DefaultClient.
	s := &Slack{WebhookURL: srv.URL}
	if err := s.Send(context.Background(), Message{Text: "x"}); err != nil {
		t.Fatalf("send with default client: %v", err)
	}
}

func TestSlackSendMalformedURLIsError(t *testing.T) {
	s := &Slack{WebhookURL: "://no-scheme"}
	err := s.Send(context.Background(), Message{Text: "x"})
	if err == nil {
		t.Fatal("expected an error for a malformed webhook URL")
	}
	if !strings.Contains(err.Error(), "slack request") {
		t.Fatalf("expected the request-building context, got %v", err)
	}
}

func TestSlackSendTransportErrorIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url, client := srv.URL, srv.Client()
	srv.Close() // nothing is listening now: the transport must fail, not panic

	s := &Slack{WebhookURL: url, Client: client}
	err := s.Send(context.Background(), Message{Text: "x"})
	if err == nil {
		t.Fatal("expected a transport error against a closed server")
	}
	if !strings.Contains(err.Error(), "slack webhook") {
		t.Fatalf("expected the webhook context, got %v", err)
	}
}

func TestFanOutAllSuccessIsNil(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	senders := []Sender{
		&Slack{WebhookURL: srv.URL, Client: srv.Client()},
		&Slack{WebhookURL: srv.URL, Client: srv.Client()},
	}
	if err := FanOut(context.Background(), senders, Message{Text: "x"}); err != nil {
		t.Fatalf("every sink succeeded, want nil, got %v", err)
	}
}

func TestFanOutJoinsEveryError(t *testing.T) {
	bad1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer bad1.Close()
	bad2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer bad2.Close()

	senders := []Sender{
		&Slack{WebhookURL: bad1.URL, Client: bad1.Client()},
		&Slack{WebhookURL: bad2.URL, Client: bad2.Client()},
	}
	err := FanOut(context.Background(), senders, Message{Text: "x"})
	if err == nil {
		t.Fatal("expected a joined error for two failing sinks")
	}
	msg := err.Error()
	if !strings.Contains(msg, "500") || !strings.Contains(msg, "502") {
		t.Fatalf("expected both sink failures in %q", msg)
	}
	if !strings.Contains(msg, "; ") {
		t.Fatalf("expected the errors to be joined in %q", msg)
	}
}
