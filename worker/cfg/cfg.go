package cfg

import (
	"os"

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
	err := os.MkdirAll(PersistDir, 0o755)
	if err != nil {
		log.Error(err)
	}
	err = os.MkdirAll(LogDir, 0o755)
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
}
