package runner

import (
	"context"
	"github.com/tum-dev/gocast/runner/protobuf"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"
	"log/slog"
	"os"
	"time"
)

const registerRetries = 5

func (r *Runner) RegisterWithGocast(retries int) {
	r.log.Debug("connecting with gocast", slog.Group("conn", "host", r.cfg.GocastServer, "retriesLeft", retries))
	if retries == 0 {
		r.log.Error("no more retries left, can't connect to gocast")
		os.Exit(1)
	}
	con, err := r.dialIn()
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

// dialIn connects to manager instance and returns a client
func (r *Runner) dialIn() (protobuf.FromRunnerClient, error) {
	credentials := insecure.NewCredentials()
	conn, err := grpc.Dial(r.cfg.GocastServer, grpc.WithTransportCredentials(credentials))
	if err != nil {
		return nil, err
	}
	return protobuf.NewFromRunnerClient(conn), nil
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
	con, err := r.dialIn()
	if err != nil {
		r.log.Warn("couldn't dial into server", "error", err, "sleeping(s)", 5-retries)
		time.Sleep(time.Second * time.Duration(5-retries))
		r.ReadDiagnostics(retries - 1)
		return
	}

	_, err = con.Heartbeat(context.Background(), &protobuf.HeartbeatRequest{
		Hostname: r.cfg.Hostname,
		Port:     int32(r.cfg.Port),
		LastSeen: timestamppb.New(time.Now()),
		Status:   "Alive",
		Workload: uint32(len(r.jobs)),
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

	con, err := r.dialIn()
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

func (r *Runner) NotifyStreamStarted(ctx context.Context, started *protobuf.StreamStarted) protobuf.Status {
	//TODO implement me
	r.log.Warn("Got called with request", "request", started)
	return protobuf.Status{Ok: true}
}

func (r *Runner) Register(ctx context.Context, request *protobuf.RegisterRequest) protobuf.RegisterResponse {
	//TODO implement me
	panic("implement me")
}

func (r *Runner) Heartbeat(ctx context.Context, request *protobuf.HeartbeatRequest) protobuf.HeartbeatResponse {
	//TODO implement me
	panic("implement me")
}

func (r *Runner) NotifyVoDUploadFinished(ctx context.Context, request *protobuf.VoDUploadFinished) protobuf.Status {
	//TODO implement me
	panic("implement me")
}

func (r *Runner) NotifySilenceResults(ctx context.Context, request *protobuf.SilenceResults) protobuf.Status {
	//TODO implement me
	panic("implement me")
}

func (r *Runner) NotifyStreamEnded(ctx context.Context, request *protobuf.StreamEnded) protobuf.Status {
	//TODO implement me
	panic("implement me")
}

func (r *Runner) NotifyThumbnailsFinished(ctx context.Context, request *protobuf.ThumbnailsFinished) protobuf.Status {
	//TODO implement me
	panic("implement me")
}

func (r *Runner) NotifyTranscodingFailure(ctx context.Context, request *protobuf.TranscodingFailureNotification) protobuf.Status {
	//TODO implement me
	panic("implement me")
}

func (r *Runner) GetStreamInfoForUpload(ctx context.Context, request *protobuf.StreamInfoForUploadRequest) protobuf.StreamInfoForUploadResponse {
	//TODO implement me
	panic("implement me")
}
