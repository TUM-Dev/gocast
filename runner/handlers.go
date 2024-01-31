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

func (r *Runner) RequestStream(ctx context.Context, req *protobuf.StreamRequest) (*protobuf.StreamResponse, error) {
	// don't reuse context from grpc, it will be canceled when the request is done.
	ctx = context.Background()
	ctx = contextFromStreamReq(req, ctx)
	a := []*actions.Action{
		r.actions.PrepareAction(),
		r.actions.StreamAction(),
		r.actions.TranscodeAction(),
		r.actions.UploadAction(),
	}
	jobID := r.AddJob(ctx, a)

	return &protobuf.StreamResponse{Job: jobID}, nil
}

func (r *Runner) RequestStreamEnd(ctx context.Context, request *protobuf.StreamEndRequest) (*protobuf.StreamEndResponse, error) {
	if job, ok := r.jobs[request.GetJobID()]; ok {
		job.Cancel(errors.New("canceled by user request"), actions.StreamAction, actions.UploadAction)
		return &protobuf.StreamEndResponse{}, nil
	}
	return nil, errors.New("job not found")
}
