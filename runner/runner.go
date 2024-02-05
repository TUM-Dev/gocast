package runner

import (
	"context"
	"fmt"
	"github.com/caarlos0/env"
	"github.com/google/uuid"
	"github.com/tum-dev/gocast/runner/actions"
	"github.com/tum-dev/gocast/runner/config"
	"github.com/tum-dev/gocast/runner/pkg/logging"
	"github.com/tum-dev/gocast/runner/pkg/netutil"
	"github.com/tum-dev/gocast/runner/protobuf"
	"github.com/tum-dev/gocast/runner/vmstat"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
	"log/slog"
	"net"
	"os"
	"time"
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
	cfg envConfig
	log *slog.Logger
	cmd config.CmdList

	JobCount chan int
	draining bool
	jobs     map[string]*Job

	actions   actions.ActionProvider
	hlsServer *HLSServer

	stats *vmstat.VmStat

	StartTime time.Time
	protobuf.UnimplementedToRunnerServer
}

func NewRunner(v string) *Runner {
	log := logging.GetLogger(v)
	var cfg envConfig
	if err := env.Parse(&cfg); err != nil {
		log.Error("error parsing envConfig", "error", err)
	}
	log.Info("envConfig loaded", "envConfig", cfg)

	cmd := config.NewCmd(log)
	log.Info("loading cmd.yaml", "cmd", cmd)

	vmstats := vmstat.New()

	start := time.Now()
	return &Runner{
		log:      log,
		JobCount: make(chan int, 1),
		draining: false,
		cfg:      cfg,
		cmd:      *cmd,
		jobs:     make(map[string]*Job),
		actions: actions.ActionProvider{
			Log:        log,
			Cmd:        *cmd,
			SegmentDir: cfg.SegmentPath,
			RecDir:     cfg.RecPath,
			MassDir:    cfg.StoragePath,
		},
		hlsServer: NewHLSServer(cfg.SegmentPath, log.WithGroup("HLSServer")),
		stats:     vmstats,
		StartTime: start,
	}
}

func (r *Runner) Run() {
	r.log.Info("Running!")
	r.actions.Server = r
	if r.cfg.Port == 0 {
		r.log.Info("Getting free port")
		p, err := netutil.GetFreePort()
		if err != nil {
			r.log.Error("Failed to get free port", "error", err)
			os.Exit(1)
		}
		r.cfg.Port = p
	}
	r.log.Info("using port", "port", r.cfg.Port)

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
	r.log.Info("Starting gRPC server", "port", r.cfg.Port)
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", r.cfg.Port))
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
	}), logging.GetGrpcLogInterceptor(r.log))
	protobuf.RegisterToRunnerServer(grpcServer, r)

	reflection.Register(grpcServer)
	if err := grpcServer.Serve(lis); err != nil {
		r.log.Error("failed to serve", "error", err)
		os.Exit(1)
	}

}

type Job struct {
	ID      string
	Actions []*actions.Action

	Log *slog.Logger
}

// Run triggers all actions in the job sequentially.
func (j *Job) Run(ctx context.Context) {
	for i := range j.Actions {
		if j.Actions[i].Canceled {
			j.Log.Info("skipping action because it was canceled", "action", j.Actions[i].Type)
			continue
		}
		// create new context to make each action cancelable individually
		actionContext, cancel := context.WithCancelCause(ctx)
		j.Actions[i].Cancel = cancel
		j.Log.Info("running action", "action", j.Actions[i].Type)
		c, err := j.Actions[i].ActionFn(actionContext, j.Log.With("action", j.Actions[i].Type))
		if err != nil {
			// ensure context is canceled even on error
			j.Log.Error("action failed", "error", err, "action", j.Actions[i].Type)
			j.Actions[i].Cancel(err)
			return
		}
		// pass context to next action without cancel
		ctx = context.WithoutCancel(c)

		j.Actions[i].Cancel(nil)
	}
}

func (j *Job) Cancel(reason error, actionTypes ...actions.ActionType) {
	for i := len(j.Actions) - 1; i >= 0; i-- { // cancel actions in reverse order to ensure all actions are canceled when they run
		for _, actionType := range actionTypes {
			if j.Actions[i].Type == actionType {
				if j.Actions[i].Cancel != nil {
					// action already running -> cancel context
					j.Actions[i].Cancel(reason)
				}
				// set canceled flag -> stop action from being started
				j.Actions[i].Canceled = true
			}
		}
	}
	j.Log.Info("job canceled", "reason", reason)
}

// AddJob adds a job to the runner and starts it.
func (r *Runner) AddJob(ctx context.Context, a []*actions.Action) string {
	jobID := uuid.New().String()
	r.jobs[jobID] = &Job{
		ID:      jobID,
		Actions: a,

		Log: enrichLogger(r.log, ctx).With("jobID", jobID),
	}
	// notify main loop about current job count
	r.JobCount <- len(r.jobs)
	done := make(chan struct{})

	go func() {
		defer func() { done <- struct{}{} }()
		r.jobs[jobID].Run(ctx)
	}()
	go func() {
		select {
		case d := <-done:
			// update job count and remove job from map after it's done
			r.log.Info("job cancelled", "jobID", jobID, "reason", ctx.Err(), "cancelReason", d)
			delete(r.jobs, jobID)
			r.JobCount <- len(r.jobs)
		}
	}()
	return jobID
}

// enrichLogger adds StreamID, CourseID, Version to the logger if present in the context
func enrichLogger(log *slog.Logger, ctx context.Context) *slog.Logger {
	if streamID, ok := ctx.Value("stream").(uint64); ok {
		log = log.With("streamID", streamID)
	}
	if courseID, ok := ctx.Value("course").(uint64); ok {
		log = log.With("courseID", courseID)
	}
	if version, ok := ctx.Value("version").(string); ok {
		log = log.With("version", version)
	}
	return log
}
