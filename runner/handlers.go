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
	ctx = context.Background()
	ctx = contextFromStreamReq(req, ctx)
	ctx = context.WithValue(ctx, "URL", "")
	ctx = context.WithValue(ctx, "Hostname", r.cfg.Hostname)
	ctx = context.WithValue(ctx, "ActionID", req.ActionID)
	a := []*actions.Action{
		r.actions.PrepareAction(),
		r.actions.StreamAction(),
	}
	aID := r.RunAction(ctx, a)
	r.log.Info("job added", "ActionID", aID)

	return &protobuf.StreamResponse{ActionID: aID}, nil
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
	ctx = context.WithValue(ctx, "ActionID", req.ActionID)
	if req.GetRunnerID() != r.cfg.Hostname {
		r.log.Error("transcoding request for wrong hostname", "hostname", req.GetRunnerID(), "expected", r.cfg.Hostname)
		return nil, errors.New("wrong hostname")
	}
	a := []*actions.Action{
		r.actions.TranscodeAction(),
	}
	_ = r.RunAction(ctx, a)
	r.log.Info("action added", "action", req.ActionID)
	return &protobuf.TranscodingResponse{ActionID: req.ActionID, TranscodingID: req.ActionID}, nil
}

func (r *Runner) RequestStreamEnd(ctx context.Context, request *protobuf.StreamEndRequest) (*protobuf.StreamEndResponse, error) {
	r.ReadDiagnostics(5)
	if activeAction, ok := r.activeActions[request.ActionID]; ok {
		for _, action := range activeAction {
			if action.Cancel != nil {
				// action already running -> cancel context
				action.Cancel(errors.New("cancelled by user request"))
			}
			// set canceled flag -> stop action from being started
			action.Canceled = true
		}
	}
	return &protobuf.StreamEndResponse{}, nil
}

func (r *Runner) GenerateLivePreview(ctx context.Context, request *protobuf.LivePreviewRequest) (*protobuf.LivePreviewResponse, error) {
	r.ReadDiagnostics(5)
	panic("implement me")
}

func (r *Runner) GenerateSectionImages(ctx context.Context, request *protobuf.GenerateSectionImageRequest) (*protobuf.Status, error) {
	r.ReadDiagnostics(5)
	panic("implement me")
}
