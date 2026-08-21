package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/1tomany/recall/internal/daemon"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	config, err := daemon.ParseConfig(args, os.Stderr)
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "recalld: %v\n", err)
		return 2
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := daemon.Run(ctx, config, logger); err != nil {
		logger.Error("recalld stopped", "error", err)
		return 1
	}
	return 0
}
