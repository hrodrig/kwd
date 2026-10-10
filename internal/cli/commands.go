// Package cli implements the kwd Cobra command tree.
package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/hrodrig/kwd/configs"
	"github.com/hrodrig/kwd/internal/config"
	"github.com/spf13/cobra"
)

// errPrintSampleDone stops command execution after sample YAML was written to stdout.
var errPrintSampleDone = errors.New("sample config printed")

// flag values resolved at PersistentPreRun time.
var (
	configPath        string
	printSampleConfig bool
	// intervalFlag is the --interval override on check only (D-01). Applied
	// after loadConfig only when Flags().Changed("interval") so YAML →
	// KWD_INTERVAL → flag precedence holds. Not registered on root —
	// bare `kwd` uses YAML/env interval only.
	intervalFlag int
	// Phase 3 D-20 confirm/repeat overrides (Changed only). --listen is plan 03-02.
	confirmAlertFlag      int
	confirmOkFlag         int
	repeatWhileFiringFlag bool
)

// Execute runs the root command and returns any error (mapped to an exit code
// by cmd/kwd/main.go). Prefer ExecuteContext from main so SIGINT/SIGTERM
// cancel Daemon cleanly (D-07).
func Execute() error {
	return ExecuteContext(context.Background())
}

// ExecuteContext runs the root command with ctx (typically signal.NotifyContext).
func ExecuteContext(ctx context.Context) error {
	err := NewRootCmd().ExecuteContext(ctx)
	if errors.Is(err, errPrintSampleDone) {
		return nil
	}
	return err
}

// NewRootCmd builds the kwd command tree. Bare `kwd` behaves as `kwd check`.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "kwd",
		Short:         "Kube Watch Dog — Kubernetes workload readiness watchdog",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return checkCommand.RunE(checkCommand, args)
		},
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			if !printSampleConfig {
				return nil
			}
			_, err := fmt.Fprint(cmd.OutOrStdout(), configs.SampleYAML())
			if err != nil {
				return err
			}
			return errPrintSampleDone
		},
	}

	root.PersistentFlags().StringVar(&configPath, "config", config.DefaultConfigPath, "path to config file")
	root.PersistentFlags().BoolVar(&printSampleConfig, "print-sample-config", false, "print sample kwd.yml to stdout and exit")

	// --interval on check only (D-01). No --daemon flag (D-02): interval alone
	// selects single-pass (0) vs forever loop (>0). Guard Lookup: checkCommand
	// is a package-level var reused across NewRootCmd calls in tests.
	// D-20 confirm/repeat flags; --listen deferred to plan 03-02. Full YAML↔CLI
	// matrix for every config key is backlog (D-21).
	if checkCommand.Flags().Lookup("interval") == nil {
		checkCommand.Flags().IntVar(&intervalFlag, "interval", 0, "override interval seconds (0 = single-pass; >0 = daemon loop)")
	}
	if checkCommand.Flags().Lookup("confirm-alert") == nil {
		checkCommand.Flags().IntVar(&confirmAlertFlag, "confirm-alert", 1, "consecutive unhealthy ticks before alert notify")
	}
	if checkCommand.Flags().Lookup("confirm-ok") == nil {
		checkCommand.Flags().IntVar(&confirmOkFlag, "confirm-ok", 1, "consecutive ready ticks before resolve notify")
	}
	if checkCommand.Flags().Lookup("repeat-while-firing") == nil {
		checkCommand.Flags().BoolVar(&repeatWhileFiringFlag, "repeat-while-firing", false, "re-notify unhealthy every completed gap while firing")
	}

	root.AddCommand(checkCommand)
	root.AddCommand(newAnalyzeCmd())
	root.AddCommand(newTargetCmd())
	root.AddCommand(newNotifyCmd())
	root.AddCommand(newVersionCmd())
	root.AddCommand(newCompletionCmd())

	return root
}

// loadConfig loads and validates config from the --config flag.
func loadConfig() (*config.Config, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	return cfg, nil
}
