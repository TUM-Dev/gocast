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
	if err := validateStreamRequest(req); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	data := map[string]any{
		"streamID":      req.GetStreamId(),
		"streamVersion": protobuf.StreamVersion_name[int32(req.GetVersion())],
		"streamEnd":     req.End.AsTime(),
		"globalOpts":    req.GetFfmpegGlobalOptions(),
		"inputOpts":     req.GetFfmpegInputOptions(),
		"outputOpts":    req.GetFfmpegOutputOptions(),
		"input":         req.GetInput(),
	}
	// The input url carries the camera credentials, so it stays out of the log.
	r.log.Info("RequestStream data constructed", "streamID", req.GetStreamId(), "version", req.GetVersion(), "end", req.GetEnd().AsTime())
	a := []actions.Action{
		actions.Stream,
		actions.StreamEnd,
	}
	vod := []actions.Action{
		actions.MkVOD,
		actions.CheckVoD,
		actions.MkThumb,
	}
	jID := r.RunAction(a, vod, data, r.log.With("stream_id", req.GetStreamId(), "stream_version", req.GetVersion()))
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
		return &protobuf.StreamEndResponse{}, nil
	}
	return nil, status.Errorf(codes.NotFound, "job %s not found", req.GetJobId())
}

// RequestSectionImages generates a thumbnail for each of the given video sections of an
// already recorded stream. The images are delivered asynchronously in a
// SectionImagesReadyNotification, so this only acknowledges the job.
func (r *Runner) RequestSectionImages(_ context.Context, req *protobuf.SectionImageRequest) (*protobuf.SectionImageResponse, error) {
	sections := make([]actions.SectionTimestamp, 0, len(req.GetSections()))
	for _, s := range req.GetSections() {
		if err := s.GetStart().CheckValid(); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "section %d has no valid start: %v", s.GetId(), err)
		}
		sections = append(sections, actions.SectionTimestamp{
			SectionID: s.GetId(),
			Start:     s.GetStart().AsDuration(),
		})
	}
	if len(sections) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "no sections given")
	}
	if err := validatePlaylistURL(req.GetPlaylistUrl()); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	data := map[string]any{
		"streamID":    req.GetStreamId(),
		"playlistURL": req.GetPlaylistUrl(),
		"sections":    sections,
	}

	// A section image job has no VoD phase, so there is nothing to skip on discard.
	jID := r.RunAction([]actions.Action{actions.MkSectionImages}, nil, data,
		r.log.With("stream_id", req.GetStreamId(), "sections", len(sections)))
	r.log.Info("section image job added", "ID", jID)

	return &protobuf.SectionImageResponse{JobId: ptr.Take(jID)}, nil
}
