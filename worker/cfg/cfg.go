package cfg

import (
	"context"
	"fmt"
	"github.com/joschahenningsen/TUM-Live/worker/pb"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"os"
	"time"
)

var (
	WorkerID     string // authentication token, unique for every worker, used to verify all calls
	TempDir      string // recordings will end up here before they are converted
	StorageDir   string // recordings will end up here after they are converted
	LrzUser      string
	LrzMail      string
	LrzPhone     string
	LrzSubDir    string
	MainBase     string
	LrzUploadUrl string
	LogDir       string
	Hostname     string
	Token        string // setup token. Used to connect initially and to get a "WorkerID"
	PersistDir   string // PersistDir is the directory, tum-live-worker will use to store persistent data
	LogLevel     = log.InfoLevel
)

// Initialise stops the execution if any of the required config variables are unset.
func Initialise() {
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
	MainBase = os.Getenv("MainBase") // eg. live.mm.rbg.tum.de

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

	// join main tumlive:
	var conn *grpc.ClientConn
	// retry connecting to tumlive every 5 seconds until successful
	for {
		conn, err = grpc.Dial(fmt.Sprintf("%s:50052", MainBase), grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err == nil {
			break
		} else {
			log.Warnf("Could not connect to main tumlive: %v\n", err)
			time.Sleep(time.Second * 5)
		}
	}

	client := pb.NewFromWorkerClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	resp, err := client.JoinWorkers(ctx, &pb.JoinWorkersRequest{
		Token:    Token,
		Hostname: Hostname,
	})
	if err != nil {
		log.Warnf("Could not join main tumlive: %v\n", err)
		return
	}
	WorkerID = resp.WorkerId
	log.Infof("Joined main tumlive with worker id: %s\n", WorkerID)
}
