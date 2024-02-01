package runner

import (
	"context"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/tum-dev/gocast/runner/actions"
	"github.com/tum-dev/gocast/runner/config"
	"github.com/tum-dev/gocast/runner/pkg/logging"
	"github.com/tum-dev/gocast/runner/pkg/netutil"
	"github.com/tum-dev/gocast/runner/pkg/server"
	"github.com/tum-dev/gocast/runner/protobuf"
	"github.com/tum-dev/gocast/runner/vmstat"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"
	"log/slog"
	"os"
	"time"
)

type Runner struct {
	cfg *config.EnvConfig
	log *slog.Logger
	cmd config.CmdList

	JobCount chan int
	draining bool
	Jobs     map[string]*Job

	Actions   actions.ActionProvider
	hlsServer *HLSServer

	stats *vmstat.VmStat

	StartTime time.Time
	protobuf.UnimplementedToRunnerServer
}

var Instance *Runner

func InitRunner(v string, grpcServer *grpc.Server) {
	log := logging.GetLogger(v)

	cmd := config.NewCmd(log)
	log.Info("loading cmd.yaml", "cmd", cmd)

	vmstats := vmstat.New()

	start := time.Now()
	Instance = &Runner{
		log:      log,
		JobCount: make(chan int, 1),
		draining: false,
		cfg:      config.Cfg,
		cmd:      *cmd,
		Jobs:     make(map[string]*Job),
		Actions: actions.ActionProvider{
			Log:        log,
			Cmd:        *cmd,
			SegmentDir: config.Cfg.SegmentPath,
			RecDir:     config.Cfg.RecPath,
			MassDir:    config.Cfg.StoragePath,
		},
		hlsServer: NewHLSServer(config.Cfg.SegmentPath, log.WithGroup("HLSServer")),
		stats:     vmstats,
		StartTime: start,
	}

	protobuf.RegisterToRunnerServer(grpcServer, Instance)
}

func (r *Runner) Run() {
	r.log.Info("Running!")
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

	go r.hlsServer.Start()

	r.RegisterWithGocast(5)
	r.log.Info("successfully connected to gocast")
}

func (r *Runner) Drain() {
	r.log.Info("Runner set to drain.")
	r.draining = true
}

const registerRetries = 5

func (r *Runner) RegisterWithGocast(retries int) {
	r.log.Debug("connecting with gocast", slog.Group("conn", "host", r.cfg.GocastServer, "retriesLeft", retries))
	if retries == 0 {
		r.log.Error("no more retries left, can't connect to gocast")
		os.Exit(1)
	}
	con, err := server.Instance.DialIn()
	if err != nil {
		r.log.Warn("error connecting to gocast", "error", err, "sleeping(s)", registerRetries-retries)
		time.Sleep(time.Second * time.Duration(registerRetries-retries))
		r.RegisterWithGocast(retries - 1)
		r.ReadDiagnostics(5)
		return
	}
	_, err = con.Register(context.Background(), &protobuf.RegisterRequest{Hostname: r.cfg.Hostname, Port: int32(r.cfg.Port)})
	if err != nil {
		r.log.Warn("error registering with gocast", "error", err, "sleeping(s)", registerRetries-retries)
		time.Sleep(time.Second * time.Duration(registerRetries-retries))
		r.RegisterWithGocast(retries - 1)
		r.ReadDiagnostics(5)
		return
	}
	r.ReadDiagnostics(5)
}

func (r *Runner) ReadDiagnostics(retries int) {

	r.log.Info("Started Sending Diagnostic Data", "retriesLeft", retries)

	if retries == 0 {
		return
	}
	err := r.stats.Update()
	if err != nil {
		r.ReadDiagnostics(retries - 1)
		return
	}
	cpu := r.stats.GetCpuStr()
	memory := r.stats.GetMemStr()
	disk := r.stats.GetDiskStr()
	uptime := time.Now().Sub(r.StartTime).String()
	con, err := server.Instance.DialIn()
	if err != nil {
		log.Warn("couldn't dial into server", "error", err, "sleeping(s)", 5-retries)
		time.Sleep(time.Second * time.Duration(5-retries))
		r.ReadDiagnostics(retries - 1)
		return
	}

	_, err = con.Heartbeat(context.Background(), &protobuf.HeartbeatRequest{
		Hostname: r.cfg.Hostname,
		Port:     int32(r.cfg.Port),
		LastSeen: timestamppb.New(time.Now()),
		Status:   "Alive",
		Workload: uint32(len(r.Jobs)),
		CPU:      cpu,
		Memory:   memory,
		Disk:     disk,
		Uptime:   uptime,
		Version:  r.cfg.Version,
	})
	if err != nil {
		r.log.Warn("Error sending the heartbeat", "error", err, "sleeping(s)", 5-retries)
		time.Sleep(time.Second * time.Duration(5-retries))
		r.ReadDiagnostics(retries - 1)
		return
	}
	r.log.Info("Successfully sent heartbeat", "retriesLeft", retries)
}

func (r *Runner) RequestSelfStream(ctx context.Context, retries int) {
	r.log.Info("Started Requesting Self Stream", "retriesLeft", retries)

	streamKey := ctx.Value("streamkey").(string)

	if retries == 0 {
		r.log.Error("no more retries left, can't start Self Stream")
		return
	}

	con, err := server.Instance.DialIn()
	if err != nil {
		r.log.Warn("error connecting to gocast", "error", err, "sleeping(s)", 5-retries)
		time.Sleep(time.Second * time.Duration(5-retries))
		r.RequestSelfStream(ctx, retries-1)
		return
	}

	_, err = con.RequestSelfStream(context.Background(), &protobuf.SelfStreamRequest{
		StreamKey: streamKey,
	})
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
	r.Jobs[jobID] = &Job{
		ID:      jobID,
		Actions: a,

		Log: enrichLogger(r.log, ctx).With("jobID", jobID),
	}
	// notify main loop about current job count
	r.JobCount <- len(r.Jobs)
	done := make(chan struct{})

	go func() {
		defer func() { done <- struct{}{} }()
		r.Jobs[jobID].Run(ctx)
	}()
	go func() {
		select {
		case d := <-done:
			// update job count and remove job from map after it's done
			r.log.Info("job cancelled", "jobID", jobID, "reason", ctx.Err(), "cancelReason", d)
			delete(r.Jobs, jobID)
			r.JobCount <- len(r.Jobs)
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
