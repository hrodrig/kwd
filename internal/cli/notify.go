package cli

import (
	"fmt"
	"time"

	"github.com/hrodrig/kwd/internal/exitcode"
	"github.com/hrodrig/kwd/internal/identity"
	"github.com/hrodrig/kwd/internal/notify"
	"github.com/spf13/cobra"
)

// newNotifyCmd builds `kwd notify` (parent for `notify test`).
func newNotifyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "notify",
		Short: "Notification helpers",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "test",
		Short: "Send a test notification to all configured sinks",
		Args:  cobra.NoArgs,
		RunE:  runNotifyTest,
	})
	return cmd
}

func runNotifyTest(cmd *cobra.Command, _ []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	senders, err := buildSenders(cfg)
	if err != nil {
		return exitcode.New(exitcode.Failure, err)
	}
	if senders == nil {
		return exitcode.New(exitcode.Failure, fmt.Errorf("no notification sinks configured"))
	}
	clientID := identity.Resolve(cfg.Client.ID)
	msg := notify.BuildTestMessage(clientID, cfg.Cluster.Name, time.Now().UTC())
	if err := notify.FanOut(ensureContext(cmd), senders, msg); err != nil {
		return exitcode.New(exitcode.Failure, fmt.Errorf("notify test: %w", err))
	}
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "kwd: notify test sent to", len(senders), "sink(s)")
	return nil
}
