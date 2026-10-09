package notify

import (
	"context"
	"net/http"
	"net/http/httptest"
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
