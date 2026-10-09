package cli

import (
	"os"

	"github.com/spf13/cobra"
)

// newCompletionCmd builds `kwd completion` (cobra built-in shell completion).
func newCompletionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "completion",
		Short: "Generate the autocompletion script for the specified shell",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			_ = cmd.Root().GenBashCompletion(os.Stdout)
		},
	}
}
