package cli

import (
	"context"
	"fmt"
	"io"
	"os"

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

	// D-01: YAML → KWD_INTERVAL (via Viper) → flag. Apply only when Changed
	// so an unset --interval default of 0 does not force single-pass over YAML.
	if cmd.Flags().Changed("interval") {
		cfg.Interval = intervalFlag
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
// 0/1. Daemon: transition notify only; returns nil on cancel (D-07 — do not
// map last-tick health to Failure).
func checkWithClient(ctx context.Context, out, errOut io.Writer, cfg *config.Config, cs kubernetes.Interface) error {
	clientID := identity.Resolve(cfg.Client.ID)
	eng := engine.New(cfg, cs, check.NewRegistry())

	if cfg.Interval > 0 {
		return eng.Daemon(ctx, engine.DaemonOptions{
			OnTick: func(verdicts []check.Verdict, transitioned bool) {
				// Full per-tick log polish is plan 02 (D-03); keep quiet here.
				_ = verdicts
				_ = transitioned
			},
			OnTransition: func(ctx context.Context, alert bool, verdicts []check.Verdict) error {
				report.WriteTable(out, verdicts)
				if cfg.DryRun {
					return nil
				}
				if err := sendTransitionNotification(ctx, cfg, clientID, alert, verdicts); err != nil {
					_, _ = fmt.Fprintln(errOut, "notify:", err)
				}
				return nil
			},
		})
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

// sendNotification builds sinks from config + env and sends a single alert for
// the not-ready verdicts. Fail-closed: a missing *_env secret is an error.
func sendNotification(ctx context.Context, cfg *config.Config, clientID string, verdicts []check.Verdict) error {
	return sendTransitionNotification(ctx, cfg, clientID, true, verdicts)
}

// sendTransitionNotification fans out an alert (alert=true) or resolution
// (alert=false). SPEC-full Message fields land in plan 02; text is minimal here.
func sendTransitionNotification(ctx context.Context, cfg *config.Config, clientID string, alert bool, verdicts []check.Verdict) error {
	if cfg.Notifications == nil || len(cfg.Notifications.Sinks) == 0 {
		return nil // notifications disabled
	}

	var senders []notify.Sender
	for _, s := range cfg.Notifications.Sinks {
		if s.Type != notify.TypeSlack {
			return fmt.Errorf("sink type %q not supported in v0.1", s.Type)
		}
		url := os.Getenv(notify.SlackWebhookEnv)
		if url == "" {
			return fmt.Errorf("slack sink requires %s env var (fail-closed)", notify.SlackWebhookEnv)
		}
		senders = append(senders, &notify.Slack{WebhookURL: url})
	}

	text := buildAlertText(verdicts)
	if !alert {
		text = buildResolveText(verdicts)
	}
	msg := notify.Message{
		ClientID: clientID,
		Cluster:  cfg.Cluster.Name,
		Text:     text,
	}
	return notify.FanOut(ctx, senders, msg)
}

func buildAlertText(verdicts []check.Verdict) string {
	notReady := 0
	for _, v := range verdicts {
		if v.Status != check.Ready {
			notReady++
		}
	}
	s := fmt.Sprintf("kwd: %d resource(s) not ready", notReady)
	for _, v := range verdicts {
		if v.Status != check.Ready {
			s += fmt.Sprintf("\n- %s: %s", v.Ref.String(), v.Status)
		}
	}
	return s
}

func buildResolveText(verdicts []check.Verdict) string {
	s := "kwd: all resources ready"
	for _, v := range verdicts {
		s += fmt.Sprintf("\n- %s: %s", v.Ref.String(), v.Status)
	}
	return s
}
