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

// RequestSectionImages generates a thumbnail for each of the given video sections of an
// already recorded stream. The images are delivered asynchronously in a
// SectionImagesReadyNotification, so this only acknowledges the job.
func (r *Runner) RequestSectionImages(_ context.Context, req *protobuf.SectionImageRequest) (*protobuf.SectionImageResponse, error) {
	sections := make([]actions.SectionTimestamp, 0, len(req.GetSections()))
	for _, s := range req.GetSections() {
		sections = append(sections, actions.SectionTimestamp{
			SectionID: s.GetSectionId(),
			Hours:     s.GetHours(),
			Minutes:   s.GetMinutes(),
			Seconds:   s.GetSeconds(),
		})
	}
	if len(sections) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "no sections given")
	}
	if req.GetPlaylistUrl() == "" {
		return nil, status.Errorf(codes.InvalidArgument, "no playlist url given")
	}

	data := map[string]any{
		"streamID":    req.GetStreamId(),
		"playlistURL": req.GetPlaylistUrl(),
		"sections":    sections,
	}

	jID := r.RunAction([]actions.Action{actions.MkSectionImages}, data,
		r.log.With("stream_id", req.GetStreamId(), "sections", len(sections)))
	r.log.Info("section image job added", "ID", jID)

	return &protobuf.SectionImageResponse{JobId: ptr.Take(jID)}, nil
}
