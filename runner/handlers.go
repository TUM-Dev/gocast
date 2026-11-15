package runner

import (
	"context"

	"github.com/tum-dev/gocast/runner/config"
	"github.com/tum-dev/gocast/runner/pkg/ptr"
	"github.com/tum-dev/gocast/runner/pkg/tracing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/tum-dev/gocast/runner/pkg/actions"
	"github.com/tum-dev/gocast/runner/protobuf"
)

func (r *Runner) RequestStream(ctx context.Context, req *protobuf.StreamRequest) (*protobuf.StreamResponse, error) {
	data := map[string]any{
		"streamID":      req.GetStreamId(),
		"streamVersion": protobuf.StreamVersion_name[int32(req.GetVersion())],
		"streamEnd":     req.End.AsTime(),
		"globalOpts":    req.GetFfmpegGlobalOptions(),
		"inputOpts":     req.GetFfmpegInputOptions(),
		"outputOpts":    req.GetFfmpegOutputOptions(),
		"input":         req.GetInput(),
		"selfStream":    req.GetSelfStream(),
	}
	r.log.Info("RequestStream data constructed", "data", data)
	a := []actions.Action{
		actions.Stream,
		actions.StreamEnd,
	}
	vod := []actions.Action{
		actions.MkVOD,
		actions.MkDuration,
		actions.CheckVoD,
		actions.MkThumb,
		actions.MkSprite,
	}
	if req.GetDetectSilence() {
		// Last, and non-fatal on failure: it is a long scan nobody should wait for or retry.
		vod = append(vod, actions.MkSilence)
	}
	jID := r.RunActionContext(ctx, a, vod, data, r.log.With("stream_id", req.GetStreamId(), "stream_version", req.GetVersion(), "input", req.GetInput()))
	r.log.Info("job added", "ID", jID)

	return &protobuf.StreamResponse{JobId: ptr.Take(jID)}, nil
}

func (r *Runner) RequestStreamEnd(_ context.Context, req *protobuf.StreamEndRequest) (*protobuf.StreamEndResponse, error) {
	r.jobsMu.Lock()
	j, ok := r.jobs[req.GetJobId()]
	if ok {
		r.discard[req.GetJobId()] = req.GetDiscardVod()
	}
	r.jobsMu.Unlock()
	if ok {
		// Only the stream: what comes after it is how the VoD gets made from the
		// segments captured until now.
		j.endStream()
		return &protobuf.StreamEndResponse{}, nil
	}
	return nil, status.Errorf(codes.NotFound, "job %s not found", req.GetJobId())
}

// RequestSectionImages generates a thumbnail for each of the given video sections of an
// already recorded stream. The images are delivered asynchronously in a
// SectionImagesReadyNotification, so this only acknowledges the job.
func (r *Runner) RequestSectionImages(ctx context.Context, req *protobuf.SectionImageRequest) (*protobuf.SectionImageResponse, error) {
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
	if req.GetPlaylistUrl() == "" {
		return nil, status.Errorf(codes.InvalidArgument, "no playlist url given")
	}

	data := map[string]any{
		"streamID":    req.GetStreamId(),
		"playlistURL": req.GetPlaylistUrl(),
		"sections":    sections,
	}

	// A section image job has no VoD phase, so there is nothing to skip on discard.
	jID := r.RunActionContext(ctx, []actions.Action{actions.MkSectionImages}, nil, data,
		r.log.With("stream_id", req.GetStreamId(), "sections", len(sections)))
	r.log.Info("section image job added", "ID", jID)

	return &protobuf.SectionImageResponse{JobId: ptr.Take(jID)}, nil
}

// regenerateActions is what RegenerateThumbnail runs: the duration, the thumbnail, then the sprite and the
// silence scan if asked for. The scan is last because it is the long one. The duration is
// redone every time, for the recordings that were made while nothing reported it, and comes
// first because a thumbnail that can't be made aborts the job: those recordings still need it.
func regenerateActions(detectSilence, generateSprite bool) []actions.Action {
	a := []actions.Action{actions.MkDuration, actions.RegenerateThumb}
	if generateSprite {
		a = append(a, actions.MkSprite)
	}
	if detectSilence {
		a = append(a, actions.MkSilence)
	}
	return a
}

// RegenerateThumbnail regenerates the thumbnail for an already-recorded VOD. The
// result is delivered asynchronously in a ThumbnailReadyNotification, so this only
// acknowledges the job.
func (r *Runner) RegenerateThumbnail(ctx context.Context, req *protobuf.RegenerateThumbnailRequest) (*protobuf.RegenerateThumbnailResponse, error) {
	if req.GetStreamId() == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "no stream id given")
	}
	if req.GetPlaylistUrl() == "" {
		return nil, status.Errorf(codes.InvalidArgument, "no playlist url given")
	}

	data := map[string]any{
		"streamID":      req.GetStreamId(),
		"streamVersion": protobuf.StreamVersion_name[int32(req.GetVersion())],
		"playlistURL":   req.GetPlaylistUrl(),
	}

	// A thumbnail regeneration job has no VoD phase, so there is nothing to skip on discard.
	jID := r.RunActionContext(ctx, regenerateActions(req.GetDetectSilence(), req.GetGenerateSprite()), nil, data,
		r.log.With("stream_id", req.GetStreamId(), "stream_version", req.GetVersion()))
	r.log.Info("thumbnail regeneration job added", "ID", jID)

	return &protobuf.RegenerateThumbnailResponse{JobId: ptr.Take(jID)}, nil
}

// RequestWaveform builds the audio level timeline for the cut editor. The result is delivered
// asynchronously in a WaveformReadyNotification, so this only acknowledges the job.
func (r *Runner) RequestWaveform(ctx context.Context, req *protobuf.WaveformRequest) (*protobuf.WaveformResponse, error) {
	if req.GetStreamId() == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "no stream id given")
	}
	sources := map[string]string{}
	for _, s := range req.GetSources() {
		if s.GetVersion() == protobuf.StreamVersion_STREAM_VERSION_UNSPECIFIED || s.GetPlaylistUrl() == "" {
			continue
		}
		sources[protobuf.StreamVersion_name[int32(s.GetVersion())]] = s.GetPlaylistUrl()
	}
	if len(sources) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "no stream versions with a playlist given")
	}

	data := map[string]any{
		"streamID": req.GetStreamId(),
		"sources":  sources,
	}
	jID := r.RunActionContext(ctx, []actions.Action{actions.MkWaveform}, nil, data,
		r.log.With("stream_id", req.GetStreamId()))
	r.log.Info("waveform job added", "ID", jID)

	return &protobuf.WaveformResponse{JobId: ptr.Take(jID)}, nil
}

// cutActions is what a cut runs. The duration and the thumbnail are made from the VOD that
// CutVOD leaves, see its playlistURL. The silences are not: the server asks for them once
// all versions are cut, as the old ones are corrected only then.
var cutActions = []actions.Action{actions.CutVOD, actions.MkDuration, actions.RegenerateThumb}

// CutVOD cuts parts out of one version of a recorded stream. Progress and the result are
// delivered asynchronously, like those of an upload, so this only acknowledges the job.
func (r *Runner) CutVOD(ctx context.Context, req *protobuf.CutVODRequest) (*protobuf.CutVODResponse, error) {
	if req.GetStreamId() == 0 || req.GetTargetStreamId() == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "no stream id given")
	}
	version, ok := protobuf.StreamVersion_name[int32(req.GetVersion())]
	if !ok || req.GetVersion() == protobuf.StreamVersion_STREAM_VERSION_UNSPECIFIED {
		return nil, status.Errorf(codes.InvalidArgument, "no valid stream version given")
	}
	if len(req.GetDiscards()) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "nothing to cut")
	}
	if req.GetPlaylistUrl() == "" {
		return nil, status.Errorf(codes.InvalidArgument, "no playlist url given")
	}
	discard := make([]actions.TimeRange, 0, len(req.GetDiscards()))
	for _, d := range req.GetDiscards() {
		if d.GetStart() < 0 || d.GetEnd() <= d.GetStart() {
			return nil, status.Errorf(codes.InvalidArgument, "invalid range %v to %v", d.GetStart(), d.GetEnd())
		}
		discard = append(discard, actions.TimeRange{Start: d.GetStart(), End: d.GetEnd()})
	}

	// streamID is the stream the result belongs to, as every action after the cut reports on it.
	data := map[string]any{
		"streamID":      req.GetTargetStreamId(),
		"streamVersion": version,
		"sourceURL":     req.GetPlaylistUrl(),
		"discard":       discard,
	}
	jID := r.RunActionContext(ctx, cutActions, nil, data,
		r.log.With("stream_id", req.GetTargetStreamId(), "source_stream_id", req.GetStreamId(), "stream_version", version))
	r.log.Info("cut job added", "ID", jID)

	return &protobuf.CutVODResponse{JobId: ptr.Take(jID)}, nil
}

// PrepareUpload reserves a slot for a video file upload. The file is then POSTed to the
// upload endpoint with the returned token; the result is delivered asynchronously in
// VODReady and ThumbnailReady notifications, so nothing is started here.
func (r *Runner) PrepareUpload(ctx context.Context, req *protobuf.PrepareUploadRequest) (*protobuf.PrepareUploadResponse, error) {
	if req.GetStreamId() == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "no stream id given")
	}
	version, ok := protobuf.StreamVersion_name[int32(req.GetVersion())]
	if !ok || req.GetVersion() == protobuf.StreamVersion_STREAM_VERSION_UNSPECIFIED {
		return nil, status.Errorf(codes.InvalidArgument, "no valid stream version given")
	}

	token, err := r.uploads.create(req.GetStreamId(), version, req.GetDetectSilence())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "couldn't create upload token")
	}
	r.uploads.attachTrace(token, tracing.Traceparent(ctx))
	return &protobuf.PrepareUploadResponse{
		Token:      ptr.Take(token),
		UploadPort: ptr.Take(int32(config.Config.UploadPort)),
	}, nil
}
