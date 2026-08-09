package runner

import (
	"context"

	"github.com/tum-dev/gocast/runner/pkg/ptr"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/tum-dev/gocast/runner/pkg/actions"
	"github.com/tum-dev/gocast/runner/protobuf"
)

func (r *Runner) RequestStream(_ context.Context, req *protobuf.StreamRequest) (*protobuf.StreamResponse, error) {
	data := map[string]any{
		"streamID":      req.GetStreamId(),
		"streamVersion": protobuf.StreamVersion_name[int32(req.GetVersion())],
		"streamEnd":     req.End.AsTime(),
		"globalOpts":    req.GetFfmpegGlobalOptions(),
		"inputOpts":     req.GetFfmpegInputOptions(),
		"outputOpts":    req.GetFfmpegOutputOptions(),
		"input":         req.GetInput(),
	}
	r.log.Info("RequestStream data constructed", "data", data)
	a := []actions.Action{
		actions.Stream,
		actions.StreamEnd,
		actions.MkVOD,
		actions.CheckVoD,
		actions.MkThumb,
	}

	jID := r.RunAction(a, data, r.log.With("stream_id", req.GetStreamId(), "stream_version", req.GetVersion(), "input", req.GetInput()))
	r.log.Info("job added", "ID", jID)

	return &protobuf.StreamResponse{JobId: ptr.Take(jID)}, nil
}

func (r *Runner) RequestStreamEnd(_ context.Context, req *protobuf.StreamEndRequest) (*protobuf.StreamEndResponse, error) {
	r.jobsMu.Lock()
	cancel, ok := r.jobs[req.GetJobId()]
	if ok {
		r.discard[req.GetJobId()] = req.GetDiscardVod()
	}
	r.jobsMu.Unlock()
	if ok {
		cancel()
		return nil, nil
	}
	return nil, status.Errorf(codes.NotFound, "stream not found")
}
