package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/hrodrig/kwd/internal/cli"
	"github.com/hrodrig/kwd/internal/exitcode"
)

func main() {
	os.Exit(runMain())
}

// runMain runs the CLI under a signal-aware context and returns a process
// exit code (see exitcode.Of). SIGINT/SIGTERM cancel the context so Daemon
// returns nil and the process exits 0 (D-07).
func runMain() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := cli.ExecuteContext(ctx); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		return exitcode.Of(err)
	}
	return exitcode.Success
}
