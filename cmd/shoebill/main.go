// Command shoebill is the CLI for the chaotic-shoebill-stork Security Chaos
// Engineering platform. The full command tree is assembled in internal/cli.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/dibakshya01/chaotic-shoebill-stork/internal/cli"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := cli.Execute(ctx, os.Args[1:]); err != nil {
		// cli.Execute already prints user-facing errors; use a non-zero exit.
		os.Exit(1)
	}
}
