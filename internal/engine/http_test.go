package engine

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hrodrig/kwd/internal/check"
)

func TestHealthzBeforeFirstTick503(t *testing.T) {
	var snap TickSnapshot
	srv := NewHTTPServer(&snap, "/healthz", "/metrics")
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	if got := rec.Body.String(); got != "unhealthy" {
		t.Fatalf("body = %q, want unhealthy", got)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Fatalf("Content-Type = %q", ct)
	}
}

func TestHealthzAllReady200(t *testing.T) {
	var snap TickSnapshot
	snap.Store([]check.Verdict{{
		Ref:    check.Ref{Kind: "deployment", Namespace: "default", Name: "app"},
		Status: check.Ready,
	}}, 40*time.Millisecond, 1_700_000_000)
	srv := NewHTTPServer(&snap, "/healthz", "/metrics")
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Body.String(); got != "ok" {
		t.Fatalf("body = %q, want ok", got)
	}
}

func TestHealthzNotReady503(t *testing.T) {
	var snap TickSnapshot
	snap.Store([]check.Verdict{{
		Ref:    check.Ref{Kind: "deployment", Namespace: "default", Name: "app"},
		Status: check.NotReady,
		Reason: "0/1",
	}}, time.Second, 1_700_000_001)
	srv := NewHTTPServer(&snap, "/healthz", "/metrics")
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	if got := rec.Body.String(); got != "unhealthy" {
		t.Fatalf("body = %q, want unhealthy", got)
	}
}

func TestHealthzUsesRawVerdictNotFiring(t *testing.T) {
	// D-03: healthz ignores hysteresis firing. Snapshot has Ready → 200 even
	// though a Hysteresis instance may still be firing from prior ticks.
	hyst := NewHysteresis(3, 1, false)
	_ = hyst.Apply([]check.Verdict{{
		Ref: check.Ref{Kind: "deployment", Namespace: "default", Name: "app"}, Status: check.NotReady,
	}})
	if hyst.Firing() {
		t.Fatal("confirm=3: first unhealthy must not enter firing")
	}
	var snap TickSnapshot
	snap.Store([]check.Verdict{{
		Ref: check.Ref{Kind: "deployment", Namespace: "default", Name: "app"}, Status: check.Ready,
	}}, time.Millisecond, 42)
	// Simulate overall still "confirming" while raw last tick is Ready.
	_ = hyst.Apply([]check.Verdict{{
		Ref: check.Ref{Kind: "deployment", Namespace: "default", Name: "app"}, Status: check.NotReady,
	}})

	srv := NewHTTPServer(&snap, "/healthz", "/metrics")
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("raw Ready must yield 200/ok regardless of hysteresis; got %d %q (firing=%v)",
			rec.Code, rec.Body.String(), hyst.Firing())
	}
}

func TestHealthzStaleOKWithoutDaemon(t *testing.T) {
	var snap TickSnapshot
	snap.Store([]check.Verdict{{
		Ref: check.Ref{Kind: "deployment", Namespace: "ns", Name: "x"}, Status: check.Ready,
	}}, time.Millisecond, 99)
	srv := NewHTTPServer(&snap, "/readyz", "/prom")
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("stale ok: status = %d", rec.Code)
	}
}

func TestMetricsBeforeFirstTickZeroed(t *testing.T) {
	var snap TickSnapshot
	srv := NewHTTPServer(&snap, "/healthz", "/metrics")
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != metricsContentType {
		t.Fatalf("Content-Type = %q", ct)
	}
	body := rec.Body.String()
	for _, name := range []string{
		"kwd_resource_ready",
		"kwd_check_latency_seconds",
		"kwd_last_tick_timestamp_seconds",
		"kwd_healthy",
	} {
		if !strings.Contains(body, name) {
			t.Fatalf("missing %s in:\n%s", name, body)
		}
	}
	if strings.Contains(body, `kwd_resource_ready{`) {
		t.Fatalf("before first tick must not emit per-resource samples:\n%s", body)
	}
	if !strings.Contains(body, "kwd_check_latency_seconds 0\n") {
		t.Fatalf("latency want 0:\n%s", body)
	}
	if !strings.Contains(body, "kwd_last_tick_timestamp_seconds 0\n") {
		t.Fatalf("timestamp want 0:\n%s", body)
	}
	if !strings.Contains(body, "kwd_healthy 0\n") {
		t.Fatalf("healthy want 0:\n%s", body)
	}
}

func TestMetricsAfterTickExposition(t *testing.T) {
	var snap TickSnapshot
	snap.Store([]check.Verdict{
		{Ref: check.Ref{Kind: "deployment", Namespace: "default", Name: "app"}, Status: check.Ready},
		{Ref: check.Ref{Kind: "statefulset", Namespace: "data", Name: "db"}, Status: check.NotReady},
	}, 42*time.Millisecond, 1_728_000_000)
	srv := NewHTTPServer(&snap, "/healthz", "/metrics")
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := rec.Body.String()
	if !strings.Contains(body, `kwd_resource_ready{kind="deployment",namespace="default",name="app"} 1`) {
		t.Fatalf("ready series missing:\n%s", body)
	}
	if !strings.Contains(body, `kwd_resource_ready{kind="statefulset",namespace="data",name="db"} 0`) {
		t.Fatalf("not-ready series missing:\n%s", body)
	}
	if !strings.Contains(body, "kwd_healthy 0\n") {
		t.Fatalf("overall unhealthy want 0:\n%s", body)
	}
	if !strings.Contains(body, "kwd_last_tick_timestamp_seconds 1728000000\n") {
		t.Fatalf("timestamp missing:\n%s", body)
	}
	if !strings.Contains(body, "kwd_check_latency_seconds 0.042\n") {
		t.Fatalf("latency missing:\n%s", body)
	}
}

func TestMetricsLabelEscape(t *testing.T) {
	if got := escapePromLabel(`a\b"c` + "\n"); got != `a\\b\"c\n` {
		t.Fatalf("escapePromLabel = %q", got)
	}
	var snap TickSnapshot
	snap.Store([]check.Verdict{{
		Ref:    check.Ref{Kind: "deployment", Namespace: `ns"x`, Name: `n\ame`},
		Status: check.Ready,
	}}, time.Millisecond, 1)
	srv := NewHTTPServer(&snap, "/healthz", "/metrics")
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := rec.Body.String()
	if !strings.Contains(body, `namespace="ns\"x"`) {
		t.Fatalf("escaped quote missing:\n%s", body)
	}
	if !strings.Contains(body, `name="n\\ame"`) {
		t.Fatalf("escaped backslash missing:\n%s", body)
	}
}

func TestMetricsStoreRace(t *testing.T) {
	var snap TickSnapshot
	srv := NewHTTPServer(&snap, "/healthz", "/metrics")
	var wg sync.WaitGroup
	stop := make(chan struct{})
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			snap.Store([]check.Verdict{{
				Ref:    check.Ref{Kind: "deployment", Namespace: "default", Name: "app"},
				Status: check.Ready,
			}}, time.Duration(i)*time.Millisecond, int64(i))
		}
		close(stop)
	}()
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				rec := httptest.NewRecorder()
				srv.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
				if rec.Code != http.StatusOK {
					t.Errorf("metrics status %d", rec.Code)
					return
				}
			}
		}
	}()
	wg.Wait()
}

func TestListenTCPFailFast(t *testing.T) {
	ln, err := ListenTCP("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	defer ln.Close()

	_, err = ListenTCP(addr)
	if err == nil {
		t.Fatal("second Listen on same addr must fail")
	}
}

func TestHTTPShutdown(t *testing.T) {
	ln, err := ListenTCP("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	var snap TickSnapshot
	srv := NewHTTPServer(&snap, "/healthz", "/metrics")
	ServeHTTP(ln, srv) // blocks until Serve accept loop is live
	if err := ShutdownHTTP(srv); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		conn, err := net.DialTimeout("tcp", addr, 50*time.Millisecond)
		if err != nil {
			return
		}
		_ = conn.Close()
		if time.Now().After(deadline) {
			t.Fatal("expected dial fail after Shutdown")
		}
		time.Sleep(5 * time.Millisecond)
	}
}
