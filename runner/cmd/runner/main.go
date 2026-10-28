package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tum-dev/gocast/runner"
	"github.com/tum-dev/gocast/runner/pkg/tracing"
)

// V (Version) is bundled into binary with -ldflags "-X ..."
var V = "dev"

func main() {
	ctx := context.Background()
	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM, syscall.SIGUSR1)
	defer cancel()

	shutdownTracing, err := tracing.Setup(context.Background(), "runner", V)
	if err != nil {
		// traces are a diagnostic aid, a runner that can't export them still has jobs to run
		slog.Error("tracing disabled", "error", err)
		shutdownTracing = func(context.Context) error { return nil }
	}
	// os.Exit skips defers, so the exits below flush the spans themselves. The runner's fatal
	// startup errors exit without, there is nothing worth flushing then.
	flushTracing := func() {
		flushCtx, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		if err := shutdownTracing(flushCtx); err != nil {
			slog.Error("flushing traces failed", "error", err)
		}
	}

	r := runner.NewRunner(V)
	// Run gets a context of its own, not the signal one. Everything it starts has to keep
	// working while the jobs finish after a SIGTERM: delivering their StreamEnd/VODReady
	// (an unread r.notifications blocks the job forever), the heartbeats that keep the
	// runner marked as draining rather than gone, and the uploads already in flight.
	// The process ends through os.Exit below.
	go r.Run(context.Background())

	<-ctx.Done()
	slog.Info("Received signal")
	// Stop swallowing signals and listen for the second one before anything else: it is
	// the way out of the wait below, and one that arrives while draining must not be lost.
	cancel()
	osSignal := make(chan os.Signal, 1)
	signal.Notify(osSignal, syscall.SIGINT, syscall.SIGTERM, syscall.SIGUSR1)
	go func() {
		<-osSignal
		// second signal, force shutdown
		slog.Info("Received second signal, shutting down immediately")
		r.Cleanup()
		flushTracing()
		os.Exit(1)
	}()

	r.Drain(context.Background())

	// let drainage propagate
	time.Sleep(time.Second)

	// Exit once the jobs, the uploads being received and the notifications that report
	// their results are done.
	r.WaitIdle()
	slog.Info("No jobs left, shutting down")
	flushTracing()
	os.Exit(0)
}
