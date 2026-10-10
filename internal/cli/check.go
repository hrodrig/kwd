package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/hrodrig/kwd/internal/check"
	"github.com/hrodrig/kwd/internal/cluster"
	"github.com/hrodrig/kwd/internal/config"
	"github.com/hrodrig/kwd/internal/engine"
	"github.com/hrodrig/kwd/internal/exitcode"
	"github.com/hrodrig/kwd/internal/identity"
	"github.com/hrodrig/kwd/internal/notify"
	"github.com/hrodrig/kwd/internal/report"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
)

// notifySendTimeout bounds an in-flight transition FanOut after parent cancel
// (RESEARCH Pitfall 3 / A2: WithoutCancel + short timeout).
const notifySendTimeout = 10 * time.Second

// checkCommand is the primary verb (also the default for bare `kwd`).
var checkCommand = &cobra.Command{
	Use:   "check",
	Short: "Run a readiness check over declared resources",
	Args:  cobra.NoArgs,
	RunE:  runCheck,
}

// runCheck wires config + cluster + engine, then delegates to checkWithClient
// (which is the testable core).
func runCheck(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	// YAML → KWD_* (Viper) → flag. Apply only when Changed so unset defaults
	// do not clobber YAML/env (D-01 interval; D-20 listen/confirm/repeat).
	// http.health_path / metrics_path stay YAML-only (D-21).
	if cmd.Flags().Changed("interval") {
		cfg.Interval = intervalFlag
	}
	if cmd.Flags().Changed("listen") {
		cfg.HTTP.Listen = listenFlag
	}
	if cmd.Flags().Changed("confirm-alert") {
		cfg.ConfirmAlert = confirmAlertFlag
	}
	if cmd.Flags().Changed("confirm-ok") {
		cfg.ConfirmOk = confirmOkFlag
	}
	if cmd.Flags().Changed("repeat-while-firing") {
		cfg.RepeatWhileFiring = repeatWhileFiringFlag
	}

	restCfg, err := cluster.LoadRESTConfig("", cfg.Kube.Context)
	if err != nil {
		return exitcode.New(exitcode.Failure, fmt.Errorf("cluster: %w", err))
	}
	cs, err := cluster.NewClientset(restCfg)
	if err != nil {
		return exitcode.New(exitcode.Failure, fmt.Errorf("clientset: %w", err))
	}

	return checkWithClient(cmd.Context(), cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg, cs)
}

// checkWithClient runs a single-pass check (Interval == 0) or Engine.Daemon
// (Interval > 0) against cs. Single-pass: table + notify-on-not-ready + exit
// 0/1. Daemon: table+notify on post-hysteresis Alert/Resolve/Repeat; nil on cancel.
func checkWithClient(ctx context.Context, out, errOut io.Writer, cfg *config.Config, cs kubernetes.Interface) error {
	clientID := identity.Resolve(cfg.Client.ID)
	eng := engine.New(cfg, cs, check.NewRegistry())

	if cfg.Interval > 0 {
		var httpSrv *http.Server
		// D-17/D-18: HTTP only when listen non-empty; fail-fast bind before Daemon.
		if cfg.HTTP.Listen != "" {
			ln, err := engine.ListenTCP(cfg.HTTP.Listen)
			if err != nil {
				return exitcode.New(exitcode.Failure, fmt.Errorf("http listen: %w", err))
			}
			httpSrv = engine.NewHTTPServer(eng.Snapshot(), cfg.HTTP.HealthPath, cfg.HTTP.MetricsPath)
			engine.ServeHTTP(ln, httpSrv)
		}
		err := eng.Daemon(ctx, engine.DaemonOptions{
			OnTick: func(verdicts []check.Verdict, action engine.NotifyAction) {
				writeTickLog(errOut, cfg, verdicts, action)
				// Table on Alert / Resolve / Repeat (D-15); not on NotifyNone.
				if action != engine.NotifyNone {
					report.WriteTable(out, verdicts)
				}
			},
			OnNotify: func(ctx context.Context, action engine.NotifyAction, verdicts []check.Verdict) error {
				if cfg.DryRun {
					return nil
				}
				// Finish in-flight notify after signal; never start if parent done.
				if ctx.Err() != nil {
					return nil
				}
				notifyCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), notifySendTimeout)
				defer cancel()
				if err := sendNotifyAction(notifyCtx, cfg, clientID, action, verdicts); err != nil {
					_, _ = fmt.Fprintln(errOut, "notify:", err)
				}
				return nil
			},
		})
		// D-19: shared NotifyContext cancel → short Shutdown; Daemon nil → exit 0.
		_ = engine.ShutdownHTTP(httpSrv)
		return err
	}

	verdicts := eng.Once(ctx)

	report.WriteTable(out, verdicts)

	if !cfg.DryRun && report.AnyNotReady(verdicts) {
		if err := sendNotification(ctx, cfg, clientID, verdicts); err != nil {
			_, _ = fmt.Fprintln(errOut, "notify:", err)
		}
	}

	if report.AnyNotReady(verdicts) {
		return exitcode.New(exitcode.Failure, fmt.Errorf("one or more resources are not ready"))
	}
	return nil
}

// writeTickLog emits a minimal per-pass line to errOut (D-03). Full table
// stays on stdout and only on Alert/Resolve/Repeat.
func writeTickLog(w io.Writer, cfg *config.Config, verdicts []check.Verdict, action engine.NotifyAction) {
	ok := !report.AnyNotReady(verdicts)
	transitioned := action != engine.NotifyNone
	if cfg != nil && cfg.LogFormat == "json" {
		_, _ = fmt.Fprintf(w, `{"msg":"tick","ok":%t,"transitioned":%t,"action":%q,"resources":%d}`+"\n",
			ok, transitioned, action.String(), len(verdicts))
		return
	}
	_, _ = fmt.Fprintf(w, "kwd: tick ok=%t transitioned=%t action=%s resources=%d\n",
		ok, transitioned, action.String(), len(verdicts))
}

// sendNotification builds sinks from config + env and sends a single alert for
// the not-ready verdicts. Fail-closed: a missing *_env secret is an error.
func sendNotification(ctx context.Context, cfg *config.Config, clientID string, verdicts []check.Verdict) error {
	return sendTransitionNotification(ctx, cfg, clientID, true, verdicts)
}

// sendNotifyAction fans out Alert / Resolve / Repeat (D-14, D-16).
func sendNotifyAction(ctx context.Context, cfg *config.Config, clientID string, action engine.NotifyAction, verdicts []check.Verdict) error {
	switch action {
	case engine.NotifyAlert:
		return sendTransitionNotification(ctx, cfg, clientID, true, verdicts)
	case engine.NotifyResolve:
		return sendTransitionNotification(ctx, cfg, clientID, false, verdicts)
	case engine.NotifyRepeat:
		return sendRepeatNotification(ctx, cfg, clientID, verdicts)
	default:
		return nil
	}
}

// sendTransitionNotification fans out an alert (alert=true) or resolution
// (alert=false) with SPEC-full Message fields (D-05); one notify per flip (D-06).
func sendTransitionNotification(ctx context.Context, cfg *config.Config, clientID string, alert bool, verdicts []check.Verdict) error {
	senders, err := buildSenders(cfg)
	if err != nil || senders == nil {
		return err
	}
	msg := notify.BuildTransitionMessage(clientID, cfg.Cluster.Name, alert, verdicts, time.Now().UTC())
	return notify.FanOut(ctx, senders, msg)
}

func sendRepeatNotification(ctx context.Context, cfg *config.Config, clientID string, verdicts []check.Verdict) error {
	senders, err := buildSenders(cfg)
	if err != nil || senders == nil {
		return err
	}
	msg := notify.BuildRepeatMessage(clientID, cfg.Cluster.Name, verdicts, time.Now().UTC())
	return notify.FanOut(ctx, senders, msg)
}

func buildSenders(cfg *config.Config) ([]notify.Sender, error) {
	if cfg.Notifications == nil || len(cfg.Notifications.Sinks) == 0 {
		return nil, nil // notifications disabled
	}

	var senders []notify.Sender
	for _, s := range cfg.Notifications.Sinks {
		if s.Type != notify.TypeSlack {
			return nil, fmt.Errorf("sink type %q not supported in v0.1", s.Type)
		}
		url := os.Getenv(notify.SlackWebhookEnv)
		if url == "" {
			return nil, fmt.Errorf("slack sink requires %s env var (fail-closed)", notify.SlackWebhookEnv)
		}
		senders = append(senders, &notify.Slack{WebhookURL: url})
	}
	return senders, nil
}
