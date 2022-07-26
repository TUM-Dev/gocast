package main

import (
	"context"
	"fmt"
	"github.com/joschahenningsen/TUM-Live/worker/api"
	"github.com/joschahenningsen/TUM-Live/worker/cfg"
	"github.com/joschahenningsen/TUM-Live/worker/pb"
	"github.com/joschahenningsen/TUM-Live/worker/rest"
	"github.com/joschahenningsen/TUM-Live/worker/worker"
	"github.com/pkg/profile"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
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
var OsSignal = make(chan os.Signal, 1)
var VersionTag = "dev"

//prepare checks if the required dependencies are installed
func prepare() {
	//check if ffmpeg is installed
	_, err := exec.LookPath("ffmpeg")
	if err != nil {
		log.Fatal("ffmpeg is not installed")
	}
	_, err = exec.LookPath("tesseract")
	if err != nil {
		log.Fatal("tesseract is not installed")
	}
}

func main() {
	prepare()

	// join main tumlive:
	var conn *grpc.ClientConn
	var err error
	// retry connecting to tumlive every 5 seconds until successful
	for {
		conn, err = grpc.Dial(fmt.Sprintf("%s:50052", cfg.MainBase), grpc.WithTransportCredentials(insecure.NewCredentials()))
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
		Token:    cfg.Token,
		Hostname: cfg.Hostname,
	})
	if err != nil {
		log.Warnf("Could not join main tumlive: %v\n", err)
		return
	}
	cfg.WorkerID = resp.WorkerId
	log.Infof("Joined main tumlive with worker id: %s\n", cfg.WorkerID)
	worker.VersionTag = VersionTag
	defer profile.Start(profile.MemProfile).Stop()
	go func() {
		_ = http.ListenAndServe(":8082", nil) // debug endpoint
	}()

	// log with time, fmt "23.09.2021 10:00:00"
	log.SetFormatter(&log.TextFormatter{TimestampFormat: "02.01.2006 15:04:05", FullTimestamp: true})
	log.SetLevel(log.DebugLevel)

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
