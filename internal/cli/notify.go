package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newNotifyCmd builds `kwd notify` (parent for `notify test`). The `test`
// subcommand is a v2 feature; v0.1 prints a clear "not yet available" rather
// than failing config.
func newNotifyCmd() *cobra.Command {
	notify := &cobra.Command{
		Use:   "notify",
		Short: "Notification helpers",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("notify test is not available in v0.1 (planned for v0.x)")
		},
	}
	notify.AddCommand(&cobra.Command{
		Use:   "test",
		Short: "Send a test notification to all configured sinks",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("notify test is not available in v0.1 (planned for v0.x)")
		},
	})
	return notify
}
