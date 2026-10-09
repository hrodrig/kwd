package cli

import (
	"fmt"

	"github.com/hrodrig/kwd/internal/config"
	"github.com/spf13/cobra"
)

// newTargetCmd builds `kwd target`: print the resolved API target
// (context, cluster) without running checks.
func newTargetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "target",
		Short: "Print the resolved Kubernetes API target",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			_, _ = fmt.Fprintf(out, "cluster: %s\n", cfg.Cluster.Name)
			_, _ = fmt.Fprintf(out, "context: %s\n", contextLabel(cfg))
			return nil
		},
	}
}

func contextLabel(cfg *config.Config) string {
	if cfg.Kube.Context == "" {
		return "(current-context)"
	}
	return cfg.Kube.Context
}
