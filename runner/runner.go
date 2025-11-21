package runner

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"reflect"
	"runtime"
	"time"

	"github.com/tum-dev/gocast/runner/pkg/ptr"

	"github.com/google/uuid"
	"github.com/sethvargo/go-retry"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"

	"github.com/tum-dev/gocast/runner/config"
	"github.com/tum-dev/gocast/runner/pkg/actions"
	"github.com/tum-dev/gocast/runner/pkg/metrics"
	"github.com/tum-dev/gocast/runner/pkg/netutil"
	"github.com/tum-dev/gocast/runner/pkg/vmstat"
	"github.com/tum-dev/gocast/runner/protobuf"
)

//nolint:all
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
	Metrics       *metrics.Broker
	Version       string
}

func NewRunner(v string) *Runner {
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})).With("version", v)

	vmstats := vmstat.New()

	start := time.Now()
	return &Runner{
		log:           log,
		JobCount:      make(chan int),
		jobs:          make(map[string]context.CancelFunc),
		draining:      false,
		hlsServer:     NewHLSServer(config.Config.SegmentPath, log.WithGroup("HLSServer"), v),
		stats:         vmstats,
		StartTime:     start,
		notifications: make(chan *protobuf.Notification),
		Metrics:       metrics.NewBroker(),
		Version:       v,
	}
}

func (r *Runner) Run(ctx context.Context) {
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

	go r.Metrics.Run()
	go r.handleNotifications()
	go r.InitApiGrpc()
	go r.livestreamCleanup(ctx, r.log.With("job", "livestreamCleanup"))
	go func() {
		err := r.hlsServer.Start()
		if err != nil {
			r.log.Error("error starting hls server", "error", err)
		}
	}()

	r.RegisterWithGocast(5)
	r.log.Info("successfully connected to gocast")
	go func() {
		t := time.NewTicker(5 * time.Second)
		for range t.C {
			r.notifications <- &protobuf.Notification{
				Data: &protobuf.Notification_Heartbeat{
					Heartbeat: &protobuf.HeartbeatNotification{
						Hostname: ptr.Take(config.Config.Hostname),
						Draining: ptr.Take(r.draining),
						JobCount: ptr.Take(uint64(len(r.jobs))),
					},
				},
			}
		}
	}()
}

func (r *Runner) Drain() {
	r.log.Info("Runner set to drain.")
	r.draining = true
	r.notifications <- &protobuf.Notification{
		Data: &protobuf.Notification_Heartbeat{
			Heartbeat: &protobuf.HeartbeatNotification{
				Hostname: ptr.Take(config.Config.Hostname),
				Draining: ptr.Take(r.draining),
			},
		},
	}
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

func (r *Runner) RunAction(a []actions.Action, data map[string]any, logger *slog.Logger) string {
	// create new context to avoid cancellation on grpc request termination
	c, cancel := context.WithCancel(context.Background())
	job := uuid.New().String()
	r.JobCount <- 1
	r.jobs[job] = cancel
	go func() {
		defer func() {
			cancel()
			delete(r.jobs, job)
			r.JobCount <- -1
		}()
		for _, action := range a {
			for {
				log := logger.With("action", getFunctionName(action)).With("job", job)
				log.Info("running action")
				s := time.Now()
				err := action(c, log, r.notifications, data, r.Metrics)
				log.Info("action completed", "duration", time.Since(s).String())
				if err != nil {
					log.Error("action error", "error", err) // use action specific logger
					if actions.IsAbortingError(err) {
						log.Info("action can't continue")
						break // escape retry loop on unrecoverable error
					}
				} else {
					break // escape retry loop on no error
				}
			}
		}
	}()
	return job
}

func (r *Runner) handleNotifications() {
	b := retry.NewFibonacci(1 * time.Second)
	b = retry.WithJitter(500*time.Millisecond, b)
	b = retry.WithMaxRetries(10, b)

	for n := range r.notifications {
		go func() {
			ctx := context.Background()
			err := retry.Do(ctx, b, r.sendNotification(n))
			if err != nil {
				r.log.Error("failed to send notification", "error", err)
			}
		}()
	}
}

func (r *Runner) sendNotification(notification *protobuf.Notification) func(ctx2 context.Context) error {
	return func(ctx context.Context) error {
		switch notification.Data.(type) {
		case *protobuf.Notification_Heartbeat:
		// pass: logging this is too noisy
		case *protobuf.Notification_ThumbnailReady:
			r.log.Debug("send notification", "notification", &protobuf.Notification_ThumbnailReady{
				ThumbnailReady: &protobuf.ThumbnailReadyNotification{
					Stream:        notification.GetThumbnailReady().Stream,
					StreamVersion: notification.GetThumbnailReady().StreamVersion,
					// strip data from this notification log to avoid noise
				},
			})
		default:
			r.log.Debug("send notification", "notification", notification)
		}
		conn, err := r.dialIn()
		if err != nil {
			return retry.RetryableError(fmt.Errorf("send notification: %w", err))
		}
		_, err = conn.Notify(ctx, notification)
		if err != nil {
			return retry.RetryableError(fmt.Errorf("send notification: %w", err))
		}
		return nil
	}
}

func getFunctionName(i interface{}) string {
	return runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name()
}

// Cleanup is called on force shutdown while actions are still running.
// it cancels all running actions
func (r *Runner) Cleanup() {
	for _, cancelFunc := range r.jobs {
		cancelFunc()
	}
	// sleep 1 second longer than our commands default waitDelay
	time.Sleep(time.Second * 11)
}
