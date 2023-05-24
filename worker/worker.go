package worker

import (
	"github.com/joschahenningsen/TUM-Live/worker/config"
	"github.com/joschahenningsen/TUM-Live/worker/protobuf"
	"github.com/joschahenningsen/TUM-Live/worker/psutil"
	"gopkg.in/yaml.v2"

	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
)

type Worker struct {
	JobCount   chan int // This channel notifies whenever a job is done. It can be used to gracefully shut down the worker when no jobs are left.
	isDraining bool     // indicates to tum-live that the worker wants to shut down and should not be assigned new jobs.

	jobs     map[string]*Pipeline
	jobsLock sync.Mutex

	restRouter restRouter

	protobuf.UnimplementedToWorkerServer

	id          uint // assigned by main instance when registering with it.
	config      config.Config
	p           *psutil.Psutil
	startupTime time.Time
	version     string
}

// Run connects to the manager and starts the worker's api endpoints
func (w *Worker) Run() {
	go w.startup()
	go w.InitApiGrpc(":55024")
	go w.InitApiHTTP(":9988")
	go w.runCron()
}

// Drain sets the worker availability to draining. This signals to the manager instance, that the worker does not want to
// receive any more jobs. If the worker is set to draining and has no active jobs or finishes the last job, it shuts down.
func (w *Worker) Drain() {
	w.isDraining = true
}

// NewWorker creates a default worker
func NewWorker(Version string) (*Worker, error) {
	c, err := getConfig()
	if err != nil {
		return nil, err
	}
	p := psutil.New()
	return &Worker{
		JobCount:    make(chan int, 1),
		isDraining:  false,
		jobs:        make(map[string]*Pipeline),
		jobsLock:    sync.Mutex{},
		config:      *c,
		p:           p,
		startupTime: time.Now(),
		version:     Version,
	}, nil
}

func getConfig() (*config.Config, error) {
	configPaths := []string{
		"/etc/worker/",
		"/",
		".",
	}
	homeDir, err := os.UserHomeDir()
	if err == nil {
		configPaths = append(configPaths, filepath.Join(homeDir, ".worker"))
	}
	for _, path := range configPaths {
		confFile, err := os.Open(filepath.Join(path, "config.yaml"))
		if err == nil {
			conf := config.Config{}
			err := yaml.NewDecoder(confFile).Decode(&conf)
			if err != nil {
				return nil, fmt.Errorf("could not decode config: %w", err)
			}
			_ = confFile.Close()
			return &conf, nil
		}
	}
	return nil, fmt.Errorf("could not find config file in any of %v", configPaths)
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
	protobuf.RegisterToWorkerServer(grpcServer, w)

	reflection.Register(grpcServer)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

// InitApiHTTP initializes the http endpoints for the worker and blocks.
func (w *Worker) InitApiHTTP(addr string) {
	http.HandleFunc("/live/", w.restRouter.liveHandler)
	http.HandleFunc("/", w.restRouter.defaultHandler)

	//http.HandleFunc("/on_publish", streams.onPublish)
	// this endpoint should **not** be exposed to the public!
	//http.HandleFunc("/upload", handleUpload)
	log.Fatal(http.ListenAndServe(addr, nil))
}

// AddJob adds a job to the worker and starts it. It returns the random id of the job.
func (w *Worker) AddJob(ctx context.Context, name string, input interface{}) (id string) {
	uuid := uuid.New().String()
	w.jobsLock.Lock()
	w.jobs[uuid] = Pipelines[name]
	w.jobsLock.Unlock()
	go w.jobs[uuid].Run(ctx)
	return uuid
}

func (w *Worker) startup() error {
	log.Info("Attempting to register with manager")
	attempts := 0
	for w.id == 0 {
		if attempts != 0 {
			time.Sleep(time.Second * 5)
		}
		client, err := w.dialIn()
		if err != nil {
			attempts++
			log.WithError(err).Warn("could not connect to manager, retrying in 5 seconds")
			continue
		}
		host, err := os.Hostname()
		if err != nil {
			return err
		}
		resp, err := client.Register(context.Background(), &protobuf.RegisterRequest{Hostname: host})
		if err != nil {
			attempts++
			log.WithError(err).Warn("could not register with manager, retrying in 5 seconds")
			continue
		}
		w.id = uint(resp.Id)
	}
	log.WithField("ID", w.id).Info("Successfully registered with manager")
	return nil
}

// dialIn connects to manager instance and returns a client
func (w *Worker) dialIn() (protobuf.FromWorkerClient, error) {
	credentials := insecure.NewCredentials()
	conn, err := grpc.Dial(fmt.Sprintf("%s:50052", w.config.Manager), grpc.WithTransportCredentials(credentials))
	if err != nil {
		return nil, err
	}
	return protobuf.NewFromWorkerClient(conn), nil
}

type cronJob struct {
	name     string
	fn       func(*Worker) error
	interval time.Duration
	lastRun  time.Time
}

var cronjobs = []cronJob{
	{
		name: "sendHeartbeat",
		fn: func(w *Worker) error {
			return w.sendHeartbeat()
		},
		interval: time.Second * 10,
		lastRun:  time.Date(1970, 1, 1, 0, 0, 0, 0, time.Local),
	},
}

func (w *Worker) runCron() {
	for {
		time.Sleep(time.Second)
		for i, job := range cronjobs {
			if time.Since(job.lastRun) > job.interval {
				go func() {
					err := job.fn(w)
					if err != nil {
						log.WithError(err).Errorf("error running cronjob %s", job.name)
					} else {
						cronjobs[i].lastRun = time.Now()
					}
				}()
			}
		}
	}
}
