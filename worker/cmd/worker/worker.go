package main

import (
	"fmt"
	"github.com/getsentry/sentry-go"
	"github.com/joschahenningsen/TUM-Live/worker/api"
	"github.com/joschahenningsen/TUM-Live/worker/rest"
	"github.com/joschahenningsen/TUM-Live/worker/worker"
	"github.com/makasim/sentryhook"
	"github.com/pkg/profile"
	log "github.com/sirupsen/logrus"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"
)

// OsSignal contains the current os signal received.
// Application exits when it's terminating (kill, int, sigusr, term)
var OsSignal chan os.Signal
var VersionTag = "dev"

//prepare checks if the required dependencies are installed
func prepare() {
	//check if ffmpeg is installed
	_, err := exec.LookPath("ffmpeg")
	if err != nil {
		log.Fatal("ffmpeg is not installed")
	}
}

func main() {
	prepare()

	worker.VersionTag = VersionTag
	defer profile.Start(profile.MemProfile).Stop()
	go func() {
		_ = http.ListenAndServe(":8082", nil) // debug endpoint
	}()

	OsSignal = make(chan os.Signal, 1)

	// log with time, fmt "23.09.2021 10:00:00"
	log.SetFormatter(&log.TextFormatter{TimestampFormat: "02.01.2006 15:04:05", FullTimestamp: true})
	log.SetLevel(log.DebugLevel)
	if os.Getenv("SentryDSN") != "" {
		err := sentry.Init(sentry.ClientOptions{
			Dsn:              os.Getenv("SentryDSN"),
			TracesSampleRate: 1,
			Debug:            true,
			AttachStacktrace: true,
			Environment:      "Worker",
		})
		if err != nil {
			log.Fatalf("sentry.Init: %s", err)
		}
		// Flush buffered events before the program terminates.
		defer sentry.Flush(2 * time.Second)
		defer sentry.Recover()
		log.AddHook(sentryhook.New([]log.Level{log.PanicLevel, log.FatalLevel, log.ErrorLevel, log.WarnLevel}))
	}
	// setup apis
	go api.InitApi(":50051")
	go rest.InitApi(":8060")
	worker.Setup()
	awaitSignal()
}

// awaitSignal Keeps the application running until a signal is received.
func awaitSignal() {
	signal.Notify(OsSignal, syscall.SIGINT, syscall.SIGTERM, syscall.SIGUSR1)
	s := <-OsSignal
	fmt.Printf("Exiting, received OsSignal: %s\n", s.String())
}
