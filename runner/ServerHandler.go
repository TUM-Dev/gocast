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
	go func() {
		for {
			r.ReadDiagnostics(5)
			time.Sleep(time.Minute)
		}
	}()
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

func (r *Runner) handleSelfStream(ctx context.Context, retries int) {
	r.log.Info("Started Requesting Self Stream", "retriesLeft", retries)

	streamKey := ctx.Value("streamKey").(string)

	if retries == 0 {
		r.log.Error("no more retries left, can't start Self Stream")
		return
	}

	con, err := r.dialIn()
	if err != nil {
		r.log.Warn("error connecting to gocast", "error", err, "sleeping(s)", 5-retries)
		time.Sleep(time.Second * time.Duration(5-retries))
		r.handleSelfStream(ctx, retries-1)
		return
	}

	_, err = con.RequestSelfStream(context.Background(), &protobuf.SelfStreamRequest{
		StreamKey: streamKey,
	})
}

func (r *Runner) RequestSelfStream(ctx context.Context, req *protobuf.SelfStreamRequest) *protobuf.SelfStreamResponse {
	panic("implement me")
}

func (r *Runner) NotifyStreamStarted(ctx context.Context, started *protobuf.StreamStarted) *protobuf.Status {

	r.log.Info("Got called with stream start notify", "request", started)

	con, err := r.dialIn()
	if err != nil {
		r.log.Warn("error connecting to gocast", "error", err)
		time.Sleep(time.Second * 5)
		return r.NotifyStreamStarted(ctx, started)

	}

	resp, err := con.NotifyStreamStarted(context.Background(), started)
	if err != nil {
		r.log.Warn("error sending stream started", "error", err)
		time.Sleep(time.Second * 5)
		return r.NotifyStreamStarted(ctx, started)
	}
	return resp
}

func (r *Runner) NotifyVoDUploadFinished(ctx context.Context, request *protobuf.VoDUploadFinished) *protobuf.Status {
	//TODO: Test me

	r.log.Info("Got called with VoD upload finished notify", "request", request)

	con, err := r.dialIn()
	if err != nil {
		r.log.Warn("error connecting to gocast", "error", err)
		time.Sleep(time.Second * 5)
		return r.NotifyVoDUploadFinished(ctx, request)
	}

	resp, err := con.NotifyVoDUploadFinished(ctx, request)
	if err != nil {
		r.log.Warn("error sending VoD upload finished", "error", err)
		time.Sleep(time.Second * 5)
		return r.NotifyVoDUploadFinished(ctx, request)

	}
	return resp
}

func (r *Runner) NotifySilenceResults(ctx context.Context, request *protobuf.SilenceResults) *protobuf.Status {
	//TODO: Test me

	r.log.Info("Got called with Silence Results notify", "request", request)

	con, err := r.dialIn()
	if err != nil {
		r.log.Warn("error connecting to gocast", "error", err)
		time.Sleep(time.Second * 5)
		return r.NotifySilenceResults(ctx, request)
	}

	resp, err := con.NotifySilenceResults(ctx, request)
	if err != nil {
		r.log.Warn("error sending silence results", "error", err)
		time.Sleep(time.Second * 5)
		return r.NotifySilenceResults(ctx, request)
	}
	return resp
}

func (r *Runner) NotifyStreamEnded(ctx context.Context, request *protobuf.StreamEnded) *protobuf.Status {

	//TODO: Test me

	r.log.Info("Got called with Stream end notify", "request", request)

	con, err := r.dialIn()
	if err != nil {
		r.log.Warn("error connecting to gocast", "error", err)
		time.Sleep(time.Second * 5)
		return r.NotifyStreamEnded(ctx, request)
	}

	resp, err := con.NotifyStreamEnded(ctx, request)
	if err != nil {
		r.log.Warn("error sending stream end", "error", err)
		time.Sleep(time.Second * 5)
		return r.NotifyStreamEnded(ctx, request)
	}
	return resp
}

func (r *Runner) NotifyThumbnailsFinished(ctx context.Context, request *protobuf.ThumbnailsFinished) *protobuf.Status {
	//TODO: Test me
	r.log.Info("Got called with Thumbnails Finished notify", "request", request)

	con, err := r.dialIn()
	if err != nil {
		r.log.Warn("error connecting to gocast", "error", err)
		time.Sleep(time.Second * 5)
		return r.NotifyThumbnailsFinished(ctx, request)
	}

	resp, err := con.NotifyThumbnailsFinished(ctx, request)
	if err != nil {
		r.log.Warn("error sending thumbnails finished", "error", err)
		time.Sleep(time.Second * 5)
		return r.NotifyThumbnailsFinished(ctx, request)
	}
	return resp
}

func (r *Runner) NotifyTranscodingFailure(ctx context.Context, request *protobuf.TranscodingFailureNotification) *protobuf.Status {
	//TODO: Test me
	r.log.Info("Got called with Transcoding Failure notify", "request", request)

	con, err := r.dialIn()
	if err != nil {
		r.log.Warn("error connecting to gocast", "error", err)
		time.Sleep(time.Second * 5)
		return r.NotifyTranscodingFailure(ctx, request)
	}

	resp, err := con.NotifyTranscodingFailure(ctx, request)
	if err != nil {
		r.log.Warn("error sending transcoding failure", "error", err)
		time.Sleep(time.Second * 5)
		return r.NotifyTranscodingFailure(ctx, request)
	}
	return resp
}

func (r *Runner) GetStreamInfoForUpload(ctx context.Context, request *protobuf.StreamInfoForUploadRequest) *protobuf.StreamInfoForUploadResponse {
	//TODO: Test me
	r.log.Info("Got called with Stream Info For Upload", "request", request)

	con, err := r.dialIn()
	if err != nil {
		r.log.Warn("error connecting to gocast", "error", err)
		time.Sleep(time.Second * 5)
		return r.GetStreamInfoForUpload(ctx, request)
	}

	resp, err := con.GetStreamInfoForUpload(ctx, request)
	if err != nil {
		r.log.Warn("error getting stream info for upload", "error", err)
		time.Sleep(time.Second * 5)
		return r.GetStreamInfoForUpload(ctx, request)
	}
	return resp
}
