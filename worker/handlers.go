package worker

import (
	"context"
	"github.com/joschahenningsen/TUM-Live/worker/protobuf"
)

func contextFromStreamReq(r *protobuf.StreamRequest, ctx context.Context) context.Context {
	ctx = context.WithValue(ctx, "stream", r.GetStream())
	ctx = context.WithValue(ctx, "course", r.GetCourse())
	ctx = context.WithValue(ctx, "version", r.GetVersion())
	return context.WithValue(ctx, "end", r.GetEnd().AsTime())
}

func (w *Worker) RequestStream(ctx context.Context, r *protobuf.StreamRequest) (*protobuf.StreamResponse, error) {
	ctx, _ = context.WithCancel(ctx)
	ctx = contextFromStreamReq(r, ctx)
	jobID := w.AddJob(ctx, "stream-default", nil)
	return &protobuf.StreamResponse{Job: jobID}, nil
}
