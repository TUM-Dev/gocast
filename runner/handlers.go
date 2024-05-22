package runner

import (
	"context"
	"errors"
	"github.com/tum-dev/gocast/runner/actions"
	"github.com/tum-dev/gocast/runner/protobuf"
)

func contextFromStreamReq(req *protobuf.StreamRequest, ctx context.Context) context.Context {
	ctx = context.WithValue(ctx, "stream", req.GetStream())
	ctx = context.WithValue(ctx, "course", req.GetCourse())
	ctx = context.WithValue(ctx, "version", req.GetVersion())
	ctx = context.WithValue(ctx, "source", req.GetSource())
	return context.WithValue(ctx, "end", req.GetEnd().AsTime())
}

func contextFromTranscodingReq(req *protobuf.TranscodingRequest, ctx context.Context) context.Context {
	ctx = context.WithValue(ctx, "stream", req.StreamName)
	ctx = context.WithValue(ctx, "course", req.CourseName)
	ctx = context.WithValue(ctx, "version", req.SourceType)
	ctx = context.WithValue(ctx, "source", req.DataURL)
	return context.WithValue(ctx, "Runner", req.RunnerID)
}

func (r *Runner) RequestStream(ctx context.Context, req *protobuf.StreamRequest) (*protobuf.StreamResponse, error) {
	r.ReadDiagnostics(5)
	// don't reuse context from grpc, it will be canceled when the request is done.
	ctx = context.Background()
	ctx = contextFromStreamReq(req, ctx)
	ctx = context.WithValue(ctx, "URL", "")
	ctx = context.WithValue(ctx, "Hostname", r.cfg.Hostname)
	a := []*actions.Action{
		r.actions.PrepareAction(),
		r.actions.StreamAction(),
	}
	jobID := r.AddJob(ctx, a)
	r.log.Info("job added", "jobID", jobID)

	return &protobuf.StreamResponse{Job: jobID}, nil
}

func (r *Runner) RequestUpload(ctx context.Context, req *protobuf.UploadRequest) (*protobuf.UploadResponse, error) {
	r.log.Info("upload request", "jobID", req.RunnerID)

	panic("implement me")
}

func (r *Runner) RequestTranscoding(ctx context.Context, req *protobuf.TranscodingRequest) (*protobuf.TranscodingResponse, error) {
	r.log.Info("transcoding request", "jobID", req.RunnerID)
	r.ReadDiagnostics(5)
	ctx = context.Background()
	ctx = contextFromTranscodingReq(req, ctx)
	ctx = context.WithValue(ctx, "URL", "")
	ctx = context.WithValue(ctx, "Hostname", r.cfg.Hostname)
	if req.GetRunnerID() != r.cfg.Hostname {
		r.log.Error("transcoding request for wrong hostname", "hostname", req.GetRunnerID(), "expected", r.cfg.Hostname)
		return nil, errors.New("wrong hostname")
	}
	a := []*actions.Action{
		r.actions.TranscodeAction(),
	}
	jobID := r.AddJob(ctx, a)
	r.log.Info("job added", "jobID", jobID)
	return &protobuf.TranscodingResponse{TranscodingID: jobID}, nil
}

func (r *Runner) RequestStreamEnd(ctx context.Context, request *protobuf.StreamEndRequest) (*protobuf.StreamEndResponse, error) {
	r.ReadDiagnostics(5)
	if job, ok := r.jobs[request.GetJobID()]; ok {
		job.Cancel(errors.New("canceled by user request"), actions.StreamAction, actions.UploadAction)
		return &protobuf.StreamEndResponse{}, nil
	}
	return nil, errors.New("job not found")
}

func (r *Runner) GenerateLivePreview(ctx context.Context, request *protobuf.LivePreviewRequest) (*protobuf.LivePreviewResponse, error) {
	r.ReadDiagnostics(5)
	if job, ok := r.jobs[request.GetRunnerID()]; ok {
		job.Cancel(errors.New("canceled by user request"), actions.StreamAction)
		return &protobuf.LivePreviewResponse{}, nil
	}

	return nil, errors.New("Live Preview not Generated")
}

func (r *Runner) GenerateSectionImages(ctx context.Context, request *protobuf.GenerateSectionImageRequest) (*protobuf.Status, error) {
	r.ReadDiagnostics(5)
	if job, ok := r.jobs[request.PlaylistURL]; ok {
		job.Cancel(errors.New("canceled by user request"), actions.StreamAction)
		return &protobuf.Status{Ok: true}, nil
	}

	return nil, errors.New("Section Images not Generated")
}
