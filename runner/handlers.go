package runner

import (
	"context"
	"github.com/tum-dev/gocast/runner/pkg/actions"
	"github.com/tum-dev/gocast/runner/pkg/ptr"
	"github.com/tum-dev/gocast/runner/protobuf"
)

func (r *Runner) RequestStream(ctx context.Context, req *protobuf.StreamRequest) (*protobuf.StreamResponse, error) {
	ctx = context.Background()

	data := map[string]any{
		"streamID":   req.StreamId,
		"streamEnd":  req.End.AsTime(),
		"globalOpts": req.FfmpegGlobalOptions,
		"inputOpts":  req.FfmpegInputOptions,
		"outputOpts": req.FfmpegOutputOptions,
		"input":      req.Input,
	}

	a := []actions.Action{
		actions.Stream,
	}

	jID := r.RunAction(a, data)
	r.log.Info("job added", "ID", jID)

	return &protobuf.StreamResponse{JobID: ptr.Take(jID)}, nil
}
