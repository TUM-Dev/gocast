package cfg

import (
	"os"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/makasim/sentryhook"
	log "github.com/sirupsen/logrus"
)

var (
	WorkerID       string // authentication token, unique for every worker, used to verify all calls
	TempDir        string // recordings will end up here before they are converted
	StorageDir     string // recordings will end up here after they are converted
	LrzUser        string
	LrzMail        string
	LrzPhone       string
	LrzSubDir      string
	MainBase       string
	LrzUploadUrl   string
	VodURLTemplate string
	LogDir         string
	Hostname       string
	Token          string // setup token. Used to connect initially and to get a "WorkerID"
	PersistDir     string // PersistDir is the directory, tum-live-worker will use to store persistent data
	LogLevel       = log.InfoLevel
)

// SetConfig sets the values of the parameter config and stops the execution
// if any of the required config variables are unset.
func SetConfig() {
	// JoinToken is required to join the main tumlive as a worker
	Token = os.Getenv("Token")
	if Token == "" {
		log.Fatal("Environment variable Token is not set")
	}
	TempDir = "/recordings" // recordings will end up here before they are converted
	StorageDir = os.Getenv("MassStorage")
	if StorageDir == "" {
		StorageDir = "/mass" // recordings will end up here after they are converted
	}
	LrzUser = os.Getenv("LrzUser")
	LrzMail = os.Getenv("LrzMail")
	LrzPhone = os.Getenv("LrzPhone")
	LrzSubDir = os.Getenv("LrzSubDir")
	LrzUploadUrl = os.Getenv("LrzUploadUrl")
	MainBase = os.Getenv("MainBase")             // eg. live.mm.rbg.tum.de
	VodURLTemplate = os.Getenv("VodURLTemplate") // eg. https://stream.lrz.de/vod/_definst_/mp4:tum/RBG/%s.mp4/playlist.m3u8

	// logging
	LogDir = os.Getenv("LogDir")
	if LogDir == "" {
		LogDir = "/var/log/stream"
	}
	switch os.Getenv("LogLevel") {
	case "trace":
		LogLevel = log.TraceLevel
	case "debug":
		LogLevel = log.DebugLevel
	case "info":
		LogLevel = log.InfoLevel
	case "warn":
		LogLevel = log.WarnLevel
	case "error":
		LogLevel = log.ErrorLevel
	case "fatal":
		LogLevel = log.FatalLevel
	case "panic":
		LogLevel = log.PanicLevel
	default:
		LogLevel = log.InfoLevel
	}
	log.SetLevel(LogLevel)

	PersistDir = os.Getenv("PersistDir")
	if PersistDir == "" {
		PersistDir = "."
	}
	err := os.MkdirAll(PersistDir, 0755)
	if err != nil {
		log.Error(err)
	}
	err = os.MkdirAll(LogDir, 0755)
	if err != nil {
		log.Warn("Could not create log directory: ", err)
	}

	// the hostname is required to announce this worker to the main tumlive
	// Usually this is passed as an environment variable using docker. Otherwise, it is set to the hostname of the machine
	Hostname = os.Getenv("Host")
	if Hostname == "" {
		Hostname, err = os.Hostname()
		if err != nil {
			log.Fatalf("Could not get hostname: %v\n", err)
		}
	}

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
}
