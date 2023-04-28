package worker

import (
	"net"
	"net/http"
	"time"

	"github.com/joschahenningsen/TUM-Live/worker-v2/pb"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
)

type Worker struct {
	JobCount   chan int // This channel notifies whenever a job is done. It can be used to gracefully shut down the worker when no jobs are left.
	isDraining bool     // indicates to tum-live that the worker wants to shut down and should not be assigned new jobs.

	restRouter restRouter

	pb.UnimplementedToWorkerServer
}

func (w *Worker) Run() {
	go w.InitApiGrpc(":50051")
	go w.InitApiHTTP(":50051")
}

func (w *Worker) Drain() {
	w.isDraining = true
}

func NewWorker() *Worker {
	return &Worker{
		JobCount:   make(chan int, 1),
		isDraining: false,
	}
}

// InitApiGrpc Initializes api endpoints
// addr: port to run on, e.g. ":8080"
func (w *Worker) InitApiGrpc(addr string) {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.WithError(err).Fatal("failed to listen")
	}
	grpcServer := grpc.NewServer(grpc.KeepaliveParams(keepalive.ServerParameters{
		MaxConnectionIdle:     time.Minute,
		MaxConnectionAge:      time.Minute,
		MaxConnectionAgeGrace: time.Second * 5,
		Time:                  time.Minute * 10,
		Timeout:               time.Second * 20,
	}))
	pb.RegisterToWorkerServer(grpcServer, w)

	reflection.Register(grpcServer)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

func (w *Worker) InitApiHTTP(addr string) {
	http.HandleFunc("/", w.restRouter.defaultHandler)
	//http.HandleFunc("/on_publish", streams.onPublish)
	// this endpoint should **not** be exposed to the public!
	//http.HandleFunc("/upload", handleUpload)
	log.Fatal(http.ListenAndServe(addr, nil))
}
