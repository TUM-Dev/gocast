package runner

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"reflect"
	"runtime"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sethvargo/go-retry"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"

	"github.com/tum-dev/gocast/runner/pkg/ptr"

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
	jobsMu   sync.Mutex
	jobs     map[string]context.CancelFunc
	discard  map[string]bool

	hlsServer *HLSServer

	stats *vmstat.VmStat

	StartTime time.Time
	protobuf.UnimplementedRunnerServiceServer

	notifications chan *protobuf.Notification
	Metrics       *metrics.Broker
	Version       string

	connMu        sync.Mutex
	managerConn   *grpc.ClientConn
	managerClient protobuf.RunnerManagerServiceClient
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
		discard:       make(map[string]bool),
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
	go r.handleNotifications(ctx)
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
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				r.jobsMu.Lock()
				jobCount := uint64(len(r.jobs))
				r.jobsMu.Unlock()
				r.notifications <- &protobuf.Notification{
					Data: &protobuf.Notification_Heartbeat{
						Heartbeat: &protobuf.HeartbeatNotification{
							Hostname: ptr.Take(config.Config.Hostname),
							Draining: ptr.Take(r.draining),
							JobCount: ptr.Take(jobCount),
						},
					},
				}
			case <-ctx.Done():
				return
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
	r.jobsMu.Lock()
	r.jobs[job] = cancel
	r.jobsMu.Unlock()
	go func() {
		defer func() {
			cancel()
			r.jobsMu.Lock()
			delete(r.jobs, job)
			delete(r.discard, job)
			r.jobsMu.Unlock()
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
			// VoD creation (MkVOD, CheckVoD, MkThumb) is intentionally skipped once the
			// recording is discarded, right after StreamEnd notifies gocast the stream has ended.
			r.jobsMu.Lock()
			shouldDiscard := r.discard[job]
			r.jobsMu.Unlock()
			if shouldDiscard && reflect.ValueOf(action).Pointer() == reflect.ValueOf(actions.StreamEnd).Pointer() {
				logger.With("job", job).Info("discarding recording, skipping VoD creation")
				break
			}
		}
	}()
	return job
}

func (r *Runner) handleNotifications(ctx context.Context) {
	bounded := retry.NewFibonacci(1 * time.Second)
	bounded = retry.WithJitter(500*time.Millisecond, bounded)
	bounded = retry.WithMaxRetries(10, bounded)

	// Critical notifications retry indefinitely until delivered or runner shuts down
	unbounded := retry.NewFibonacci(1 * time.Second)
	unbounded = retry.WithJitter(500*time.Millisecond, unbounded)
	unbounded = retry.WithCappedDuration(30*time.Second, unbounded)

	for {
		select {
		case n := <-r.notifications:
			go func() {
				b := bounded
				switch n.Data.(type) {
				case *protobuf.Notification_StreamEnd,
					*protobuf.Notification_StreamStart,
					*protobuf.Notification_VodReady:
					b = unbounded
				}
				err := retry.Do(ctx, b, r.sendNotification(n))
				if err != nil {
					r.log.Error("failed to send notification", "error", err,
						"type", fmt.Sprintf("%T", n.Data))
				}
			}()
		case <-ctx.Done():
			return
		}
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
		conn, err := r.getManagerClient()
		if err != nil {
			return retry.RetryableError(fmt.Errorf("send notification: %w", err))
		}
		_, err = conn.Notify(ctx, notification)
		if err != nil {
			r.invalidateManagerConn()
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
	r.jobsMu.Lock()
	for _, cancelFunc := range r.jobs {
		cancelFunc()
	}
	r.jobsMu.Unlock()
	// sleep 1 second longer than our commands default waitDelay
	time.Sleep(time.Second * 11)
}
