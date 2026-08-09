package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tum-dev/gocast/runner"
)

// V (Version) is bundled into binary with -ldflags "-X ..."
var V = "dev"

func main() {
	ctx := context.Background()
	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM, syscall.SIGUSR1)
	defer cancel()

	r := runner.NewRunner(V)
	go r.Run(ctx)

	shouldShutdown := false // set to true once we receive a shutdown signal

	currentCount := 0

	go func() {
		for {
			currentCount += <-r.JobCount // count Job start/stop
			slog.Info("current job count", "count", currentCount)
			if shouldShutdown && currentCount == 0 { // if we should shut down and no jobs are running, exit.
				slog.Info("No jobs left, shutting down")
				os.Exit(0)
			}
		}
	}()

	<-ctx.Done()
	slog.Info("Received signal")
	shouldShutdown = true
	r.Drain(ctx)

	// let drainage propagate
	time.Sleep(time.Second)

	go func() {
		osSignal := make(chan os.Signal, 1)
		signal.Notify(osSignal, syscall.SIGINT, syscall.SIGTERM, syscall.SIGUSR1)

		<-osSignal
		// second signal, force shutdown
		slog.Info("Received second signal, shutting down immediately")
		r.Cleanup()
		os.Exit(1)
	}()

	if currentCount == 0 {
		slog.Info("No jobs left, shutting down")
		//nolint:all
		os.Exit(0)
	}

	blocking := make(chan struct{})
	//nolint:all
	_ = <-blocking
}
