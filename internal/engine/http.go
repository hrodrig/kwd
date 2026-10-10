package engine

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/hrodrig/kwd/internal/check"
	"github.com/hrodrig/kwd/internal/report"
)

// HTTPShutdownTimeout bounds Server.Shutdown after Daemon returns (D-19).
const HTTPShutdownTimeout = 5 * time.Second

// httpReadHeaderTimeout guards slowloris on the observability listener (T-03-05).
const httpReadHeaderTimeout = 5 * time.Second

const metricsContentType = "text/plain; version=0.0.4; charset=utf-8"

// NewHTTPServer builds a stdlib Server with healthz + metrics handlers over snap.
// Paths default to /healthz and /metrics when empty (config.applyDefaults normally fills them).
func NewHTTPServer(snap *TickSnapshot, healthPath, metricsPath string) *http.Server {
	if healthPath == "" {
		healthPath = "/healthz"
	}
	if metricsPath == "" {
		metricsPath = "/metrics"
	}
	mux := http.NewServeMux()
	mux.HandleFunc(healthPath, healthzHandler(snap))
	mux.HandleFunc(metricsPath, metricsHandler(snap))
	return &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: httpReadHeaderTimeout,
	}
}

// ListenTCP binds addr for fail-fast before Daemon (D-18).
func ListenTCP(addr string) (net.Listener, error) {
	return net.Listen("tcp", addr)
}

// ServeHTTP serves srv on ln in a new goroutine and waits until Serve has
// entered its accept loop (via BaseContext) so ShutdownHTTP is never raced
// against a not-yet-started Server.
// Errors from Serve (other than http.ErrServerClosed) are discarded — bind already succeeded.
func ServeHTTP(ln net.Listener, srv *http.Server) {
	started := make(chan struct{})
	prev := srv.BaseContext
	srv.BaseContext = func(l net.Listener) context.Context {
		close(started)
		if prev != nil {
			return prev(l)
		}
		return context.Background()
	}
	go func() { _ = srv.Serve(ln) }()
	<-started
}

// ShutdownHTTP stops accept and drains handlers with HTTPShutdownTimeout (D-19).
func ShutdownHTTP(srv *http.Server) error {
	if srv == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), HTTPShutdownTimeout)
	defer cancel()
	return srv.Shutdown(ctx)
}

func healthzHandler(snap *TickSnapshot) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		have, verdicts, _, _ := snap.Load()
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		if !have || report.AnyNotReady(verdicts) {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = io.WriteString(w, "unhealthy")
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
	}
}

func metricsHandler(snap *TickSnapshot) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		have, verdicts, latency, tickUnix := snap.Load()
		w.Header().Set("Content-Type", metricsContentType)
		w.WriteHeader(http.StatusOK)
		writeMetrics(w, have, verdicts, latency, tickUnix)
	}
}

func writeMetrics(w io.Writer, have bool, verdicts []check.Verdict, latency time.Duration, tickUnix int64) {
	_, _ = io.WriteString(w, "# HELP kwd_resource_ready Whether the resource was Ready on the last completed tick (1=ready, 0=not).\n")
	_, _ = io.WriteString(w, "# TYPE kwd_resource_ready gauge\n")
	if have {
		for _, v := range verdicts {
			ready := 0
			if v.Status == check.Ready {
				ready = 1
			}
			_, _ = fmt.Fprintf(w, "kwd_resource_ready{kind=\"%s\",namespace=\"%s\",name=\"%s\"} %d\n",
				escapePromLabel(v.Ref.Kind),
				escapePromLabel(v.Ref.Namespace),
				escapePromLabel(v.Ref.Name),
				ready)
		}
	}

	latSec := 0.0
	ts := int64(0)
	healthy := 0
	if have {
		latSec = latency.Seconds()
		ts = tickUnix
		if !report.AnyNotReady(verdicts) {
			healthy = 1
		}
	}

	_, _ = io.WriteString(w, "# HELP kwd_check_latency_seconds Wall time of the last completed check pass.\n")
	_, _ = io.WriteString(w, "# TYPE kwd_check_latency_seconds gauge\n")
	_, _ = fmt.Fprintf(w, "kwd_check_latency_seconds %s\n", formatFloat(latSec))

	_, _ = io.WriteString(w, "# HELP kwd_last_tick_timestamp_seconds Unix time of the last completed check pass.\n")
	_, _ = io.WriteString(w, "# TYPE kwd_last_tick_timestamp_seconds gauge\n")
	_, _ = fmt.Fprintf(w, "kwd_last_tick_timestamp_seconds %d\n", ts)

	_, _ = io.WriteString(w, "# HELP kwd_healthy 1 if every resource was Ready on the last completed tick.\n")
	_, _ = io.WriteString(w, "# TYPE kwd_healthy gauge\n")
	_, _ = fmt.Fprintf(w, "kwd_healthy %d\n", healthy)
}

func formatFloat(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// escapePromLabel escapes \, newline, and " for Prometheus text label values (Pitfall 6).
// Used by tests and any writer that does not go through fmt %q.
func escapePromLabel(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}
