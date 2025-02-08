package main

import (
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
	// ...

	// Init EnvConfig
	r := runner.NewRunner(V)
	go r.Run()

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

	osSignal := make(chan os.Signal, 1)
	signal.Notify(osSignal, syscall.SIGINT, syscall.SIGTERM, syscall.SIGUSR1)
	s := <-osSignal
	slog.Info("Received signal", "signal", s)
	shouldShutdown = true
	r.Drain()

	//let drainage propagate
	time.Sleep(time.Second * 10)

	if currentCount == 0 {
		slog.Info("No jobs left, shutting down")
		os.Exit(0)
	}

	blocking := make(chan struct{})
	_ = <-blocking
}
