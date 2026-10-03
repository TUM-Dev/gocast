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

// jobCancels ends the two phases of a job separately. Ending the stream is what stops
// a capture early and still lets the VoD be made from what was captured, so the two
// cannot share a context -- but a forced shutdown has to reach both, and a discard has
// to reach whatever is being made after the stream.
type jobCancels struct {
	endStream context.CancelFunc
	endAfter  context.CancelFunc
}

type Runner struct {
	log *slog.Logger

	draining bool
	JobCount chan int
	jobsMu   sync.Mutex
	jobs     map[string]jobCancels
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
		jobs:          make(map[string]jobCancels),
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

func (r *Runner) Drain(ctx context.Context) {
	r.log.Info("Runner set to drain.")
	r.draining = true
	select {
	case r.notifications <- &protobuf.Notification{
		Data: &protobuf.Notification_Heartbeat{
			Heartbeat: &protobuf.HeartbeatNotification{
				Hostname: ptr.Take(config.Config.Hostname),
				Draining: ptr.Take(r.draining),
			},
		},
	}:
	case <-ctx.Done():
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

// RunAction runs the actions of a stream job in the background and returns the id of the
// created job.
//
// The stream actions run under a context that RequestStreamEnd cancels to stop the capture
// early. They keep running after that cancellation, which is what lets StreamEnd report the
// end of a stream that was stopped early.
//
// Afterwards either the vod or the discard actions run, depending on whether the stream was
// ended with discardVod. Both run under a context of their own: turning the segments that
// were captured until the cancellation into a VoD is precisely what still has to happen
// after a stream was ended early, so they must not inherit the cancelled stream context.
func (r *Runner) RunAction(stream, vod, discard []actions.Action, data map[string]any, logger *slog.Logger) string {
	// create new contexts to avoid cancellation on grpc request termination
	streamCtx, endStream := context.WithCancel(context.Background())
	afterCtx, endAfter := context.WithCancel(context.Background())
	job := uuid.New().String()
	r.JobCount <- 1
	r.jobsMu.Lock()
	r.jobs[job] = jobCancels{endStream: endStream, endAfter: endAfter}
	r.jobsMu.Unlock()
	go func() {
		defer func() {
			endStream()
			endAfter()
			r.jobsMu.Lock()
			delete(r.jobs, job)
			delete(r.discard, job)
			r.jobsMu.Unlock()
			r.JobCount <- -1
		}()

		run := func(ctx context.Context, action actions.Action) {
			for {
				log := logger.With("action", getFunctionName(action)).With("job", job)
				log.Info("running action")
				s := time.Now()
				err := action(ctx, log, r.notifications, data, r.Metrics)
				log.Info("action completed", "duration", time.Since(s).String())
				if err != nil {
					log.Error("action error", "error", err) // use action specific logger
					if actions.IsAbortingError(err) {
						log.Info("action can't continue")
						return // escape retry loop on unrecoverable error
					}
				} else {
					return // escape retry loop on no error
				}
			}
		}

		for _, action := range stream {
			run(streamCtx, action)
		}

		log := logger.With("job", job)
		if r.discarded(job) {
			log.Info("discarding recording, skipping VoD creation")
			r.runDiscard(job, discard, run)
			return
		}

		for _, action := range vod {
			// Cancelled by a discard that arrived while the VoD was being made, and by
			// a forced shutdown. Neither is a reason to attempt the actions that were
			// still to come.
			if afterCtx.Err() != nil {
				break
			}
			run(afterCtx, action)
		}
		// The flag is read again because it can be set while the VoD is being made.
		// RequestStreamEnd cancelled afterCtx in that case, so the actions above
		// stopped where they were rather than finishing and announcing a VoD; what is
		// left of the recording still has to be cleaned up.
		if r.discarded(job) {
			log.Info("discard arrived while the VoD was being made, discarding the recording")
			r.runDiscard(job, discard, run)
		}
	}()
	return job
}

// runDiscard runs the discard actions under a context of their own. Whenever a discard
// is what got us here the after context is already cancelled -- that cancellation is
// what keeps a VoD from being made -- so the cleanup cannot inherit it. The new cancel
// takes its place on the job, so a forced shutdown still reaches the cleanup too.
func (r *Runner) runDiscard(job string, discard []actions.Action, run func(context.Context, actions.Action)) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r.jobsMu.Lock()
	if j, ok := r.jobs[job]; ok {
		j.endAfter = cancel
		r.jobs[job] = j
	}
	r.jobsMu.Unlock()

	for _, action := range discard {
		run(ctx, action)
	}
}

func (r *Runner) discarded(job string) bool {
	r.jobsMu.Lock()
	defer r.jobsMu.Unlock()
	return r.discard[job]
}

// notificationBackoff returns a fresh backoff for sending n. Backoffs are stateful
// (retry counter and fibonacci sequence), so every notification needs its own.
func notificationBackoff(n *protobuf.Notification) retry.Backoff {
	b := retry.NewFibonacci(1 * time.Second)
	b = retry.WithJitter(500*time.Millisecond, b)

	switch n.Data.(type) {
	case *protobuf.Notification_StreamEnd,
		*protobuf.Notification_StreamStart,
		*protobuf.Notification_VodReady,
		*protobuf.Notification_ThumbnailReady,
		*protobuf.Notification_SectionImagesReady:
		// Critical notifications retry indefinitely until delivered or runner shuts down
		return retry.WithCappedDuration(30*time.Second, b)
	default:
		return retry.WithMaxRetries(10, b)
	}
}

func (r *Runner) handleNotifications(ctx context.Context) {
	for {
		select {
		case n := <-r.notifications:
			go func() {
				err := retry.Do(ctx, notificationBackoff(n), r.sendNotification(n))
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
		case *protobuf.Notification_SectionImagesReady:
			r.log.Debug("send notification", "type", "SectionImagesReady",
				"stream", notification.GetSectionImagesReady().GetStream().GetId(),
				// strip the images from this notification log to avoid noise
				"images", len(notification.GetSectionImagesReady().GetImages()))
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
	for _, j := range r.jobs {
		j.endStream()
		j.endAfter()
	}
	r.jobsMu.Unlock()
	// sleep 1 second longer than our commands default waitDelay
	time.Sleep(time.Second * 11)
}
