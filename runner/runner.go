package runner

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"

	"github.com/tum-dev/gocast/runner/config"
	"github.com/tum-dev/gocast/runner/pkg/actions"
	"github.com/tum-dev/gocast/runner/pkg/netutil"
	"github.com/tum-dev/gocast/runner/pkg/vmstat"
	"github.com/tum-dev/gocast/runner/protobuf"
)

type envConfig struct {
	LogFmt       string `env:"LOG_FMT" envDefault:"txt"`
	LogLevel     string `env:"LOG_LEVEL" envDefault:"debug"`
	Port         int    `env:"PORT" envDefault:"0"`
	StoragePath  string `env:"STORAGE_PATH" envDefault:"storage/mass"`
	SegmentPath  string `env:"SEGMENT_PATH" envDefault:"storage/live"`
	RecPath      string `env:"REC_PATH" envDefault:"storage/rec"`
	GocastServer string `env:"GOCAST_SERVER" envDefault:"localhost:50056"`
	Hostname     string `env:"REALHOST" envDefault:"localhost"`
	Version      string `env:"VERSION" envDefault:"dev"`
}

type Runner struct {
	log *slog.Logger

	draining bool
	JobCount chan int
	jobs     map[string]context.CancelFunc

	hlsServer *HLSServer

	stats *vmstat.VmStat

	StartTime time.Time
	protobuf.UnimplementedRunnerServiceServer

	notifications chan *protobuf.Notification
}

func NewRunner(v string) *Runner {
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})).With("version", v)

	vmstats := vmstat.New()

	start := time.Now()
	return &Runner{
		log:           log,
		JobCount:      make(chan int),
		jobs:          make(map[string]context.CancelFunc),
		draining:      false,
		hlsServer:     NewHLSServer(config.Config.SegmentPath, log.WithGroup("HLSServer")),
		stats:         vmstats,
		StartTime:     start,
		notifications: make(chan *protobuf.Notification),
	}
}

func (r *Runner) Run() {
	r.log.Info("Running!")
	if config.Config.Port == 0 {
		r.log.Info("Getting free port")
		p, err := netutil.GetFreePort()
		if err != nil {
			r.log.Error("Failed to get free port", "error", err)
			os.Exit(1)
		}
		config.Config.Port = p
	}
	r.log.Info("using port", "port", config.Config.Port)

	go r.handleNotifications()
	go r.InitApiGrpc()
	go func() {
		err := r.hlsServer.Start()
		if err != nil {

		}
	}()

	r.RegisterWithGocast(5)
	r.log.Info("successfully connected to gocast")
}

func (r *Runner) Drain() {
	r.log.Info("Runner set to drain.")
	r.draining = true
}

func (r *Runner) InitApiGrpc() {
	r.log.Info("Starting gRPC server", "port", config.Config.Port)
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", config.Config.Port))
	if err != nil {
		r.log.Error("failed to listen", "error", err)
		os.Exit(1)
	}
	grpcServer := grpc.NewServer(grpc.KeepaliveParams(keepalive.ServerParameters{
		MaxConnectionIdle:     time.Minute,
		MaxConnectionAge:      time.Minute,
		MaxConnectionAgeGrace: time.Second * 5,
		Time:                  time.Minute * 10,
		Timeout:               time.Second * 20,
	}))
	protobuf.RegisterRunnerServiceServer(grpcServer, r)

	reflection.Register(grpcServer)
	if err := grpcServer.Serve(lis); err != nil {
		r.log.Error("failed to serve", "error", err)
		os.Exit(1)
	}

}

func (r *Runner) RunAction(a []actions.Action, data map[string]any) string {
	// create new context to avoid cancellation on grpc request termination
	c, cancel := context.WithCancel(context.Background())
	job := uuid.New().String()
	r.JobCount <- 1
	r.jobs[job] = cancel
	defer func() {
		cancel()
		delete(r.jobs, job)
		r.JobCount <- -1
	}()
	go func() {
		for _, action := range a {
			err := action(c, r.log, r.notifications, data)
			if err != nil {
				r.log.Error("action error", "error", err)
			}
			if errors.Is(err, actions.ErrAborted) {
				r.log.Info("action can't continue")
				return
			}
		}
	}()
	return job
}

func (r *Runner) handleNotifications() {
	for n := range r.notifications {
		r.log.Info("got notification", "notification", n)
	}
}
