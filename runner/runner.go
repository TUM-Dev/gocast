package runner

import (
	"context"
	"fmt"
	"github.com/caarlos0/env"
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

	draining      bool
	ActionCount   chan int
	activeActions map[string][]*actions.Action

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
		log:           log,
		ActionCount:   make(chan int),
		activeActions: make(map[string][]*actions.Action),
		draining:      false,
		cfg:           cfg,
		cmd:           *cmd,
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

func (r *Runner) RunAction(ctx context.Context, a []*actions.Action) string {
	r.ActionCount <- len(r.activeActions)
	ActionID := ctx.Value("actionID").(string)
	r.activeActions[ActionID] = a
	go func() {
		for _, action := range a {
			if action.Canceled {
				r.log.Info("skipping action because it was canceled", "action", action.Type)
				continue
			}
			// create new context to make each action cancelable individually
			actionContext, cancel := context.WithCancelCause(ctx)
			action.Cancel = cancel
			r.log.Info("running action", "action", action.Type)
			c, err := action.ActionFn(actionContext, r.log.With("action", action.Type))
			if err != nil {
				// ensure context is canceled even on error
				r.log.Error("action failed", "error", err, "action", action.Type)
				action.Cancel(err)
				return
			}
			// pass context to next action without cancel
			ctx = context.WithoutCancel(c)

			action.Cancel(nil)
		}
	}()

	return ActionID
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
