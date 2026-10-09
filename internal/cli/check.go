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

// checkWithClient runs the single-pass check against cs, renders the report,
// sends notifications (unless dry-run), and returns a non-nil error (carrying
// exit code 1) when any resource is not ready.
func checkWithClient(ctx context.Context, out, errOut io.Writer, cfg *config.Config, cs kubernetes.Interface) error {
	clientID := identity.Resolve(cfg.Client.ID)

	eng := engine.New(cfg, cs, check.NewRegistry())
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

	msg := notify.Message{
		ClientID: clientID,
		Cluster:  cfg.Cluster.Name,
		Text:     buildAlertText(verdicts),
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
