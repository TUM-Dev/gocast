package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/joschahenningsen/TUM-Live/worker-v2"
	log "github.com/sirupsen/logrus"
)

func main() {
	// ...
	worker := worker.NewWorker()

	go worker.Run()

	shouldShutdown := false // set to true once we receive a shutdown signal

	currentCount := 0
	go func() {
		for {
			currentCount = <-worker.JobCount         // wait for a job to finish
			if shouldShutdown && currentCount == 0 { // if we should shut down and no jobs are running, exit.
				log.Info("No jobs left, shutting down")
				os.Exit(0)
			}
		}
	}()

	osSignal := make(chan os.Signal, 1)
	signal.Notify(osSignal, syscall.SIGINT, syscall.SIGTERM, syscall.SIGUSR1)
	s := <-osSignal
	log.Info("Received signal ", s)
	shouldShutdown = true
	worker.Drain()

	if currentCount == 0 {
		log.Info("No jobs left, shutting down")
		os.Exit(0)
	}

	blocking := make(chan struct{})
	_ = <-blocking

}
