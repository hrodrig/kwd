package cli

import (
	"fmt"
	"sort"

	"github.com/hrodrig/kwd/internal/config"
	"github.com/spf13/cobra"
)

// newAnalyzeCmd builds `kwd analyze`: load + validate config, print a
// normalized plan of what would be checked. Never contacts the cluster.
func newAnalyzeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "analyze",
		Short: "Validate config and print the normalized check plan",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			printAnalyze(cmd, cfg)
			return nil
		},
	}
}

func printAnalyze(cmd *cobra.Command, cfg *config.Config) {
	out := cmd.OutOrStdout()
	_, _ = fmt.Fprintln(out, "cluster:", cfg.Cluster.Name)
	_, _ = fmt.Fprintf(out, "environment: %s\n", cfg.Cluster.Environment)
	_, _ = fmt.Fprintf(out, "interval: %ds\n", cfg.Interval)
	_, _ = fmt.Fprintf(out, "sinks: %d\n", sinkCount(cfg))
	_, _ = fmt.Fprintln(out, "resources:")
	refs := append([]string(nil), cfg.Resources...)
	sort.Strings(refs)
	for _, ref := range refs {
		_, _ = fmt.Fprintf(out, "  - %s\n", ref)
	}
}

func sinkCount(cfg *config.Config) int {
	if cfg.Notifications == nil {
		return 0
	}
	return len(cfg.Notifications.Sinks)
}
