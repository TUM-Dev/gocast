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
		os.Exit(1)
	}()

	r.Drain(context.Background())

	// let drainage propagate
	time.Sleep(time.Second)

	// Exit once the jobs, the uploads being received and the notifications that report
	// their results are done.
	r.WaitIdle()
	slog.Info("No jobs left, shutting down")
	os.Exit(0)
}
