package main

import (
	"github.com/tum-dev/gocast/runner"
	"github.com/tum-dev/gocast/runner/config"
	"github.com/tum-dev/gocast/runner/pkg/logging"
	"github.com/tum-dev/gocast/runner/pkg/server"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

// V (Version) is bundled into binary with -ldflags "-X ..."
var V = "dev"

func main() {
	// ...
	logger := logging.GetLogger(V)

	// Init EnvConfig
	config.Init(logger)
	server.InitServer(config.Cfg, logger)

	runner.InitRunner(V, server.Instance.GRPCServer)
	go runner.Instance.Run()

	shouldShutdown := false // set to true once we receive a shutdown signal

	currentCount := 0
	go func() {
		for {
			currentCount = <-runner.Instance.JobCount // wait for a job to finish
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
	runner.Instance.Drain()

	if currentCount == 0 {
		slog.Info("No jobs left, shutting down")
		os.Exit(0)
	}

	blocking := make(chan struct{})
	_ = <-blocking

}
