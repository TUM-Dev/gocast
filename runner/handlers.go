package runner

import (
	"context"
	"os"

	"github.com/tum-dev/gocast/runner/config"
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
		"selfStream":    req.GetSelfStream(),
	}
	r.log.Info("RequestStream data constructed", "data", data)
	a := []actions.Action{
		actions.Stream,
		actions.StreamEnd,
	}
	vod := []actions.Action{
		actions.MkVOD,
		actions.CheckVoD,
		actions.MkThumb,
	}
	if req.GetDetectSilence() {
		// Last, and non-fatal on failure: it is a long scan nobody should wait for or retry.
		vod = append(vod, actions.MkSilence)
	}
	jID := r.RunAction(a, vod, data, r.log.With("stream_id", req.GetStreamId(), "stream_version", req.GetVersion(), "input", req.GetInput()))
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
	if req.GetPlaylistUrl() == "" {
		return nil, status.Errorf(codes.InvalidArgument, "no playlist url given")
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

// regenerateActions is what RegenerateThumbnail runs: the thumbnail, then the silence scan
// if asked for. The scan is last because it is the long one.
func regenerateActions(detectSilence bool) []actions.Action {
	if detectSilence {
		return []actions.Action{actions.RegenerateThumb, actions.MkSilence}
	}
	return []actions.Action{actions.RegenerateThumb}
}

// RegenerateThumbnail regenerates the thumbnail for an already-recorded VOD. The
// result is delivered asynchronously in a ThumbnailReadyNotification, so this only
// acknowledges the job.
func (r *Runner) RegenerateThumbnail(_ context.Context, req *protobuf.RegenerateThumbnailRequest) (*protobuf.RegenerateThumbnailResponse, error) {
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
	jID := r.RunAction(regenerateActions(req.GetDetectSilence()), nil, data,
		r.log.With("stream_id", req.GetStreamId(), "stream_version", req.GetVersion()))
	r.log.Info("thumbnail regeneration job added", "ID", jID)

	return &protobuf.RegenerateThumbnailResponse{JobId: ptr.Take(jID)}, nil
}

// RequestOfflinePackage packages a VOD for offline viewing. The result is delivered
// asynchronously in an OfflinePackageReady or OfflinePackageFailed notification. The
// request holds the package's key: it is passed on to the action and never logged.
func (r *Runner) RequestOfflinePackage(_ context.Context, req *protobuf.OfflinePackageRequest) (*protobuf.OfflinePackageResponse, error) {
	if req.GetPackageId() == 0 || req.GetStreamId() == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "no package or stream id given")
	}
	version, ok := protobuf.StreamVersion_name[int32(req.GetVersion())]
	if !ok || req.GetVersion() == protobuf.StreamVersion_STREAM_VERSION_UNSPECIFIED {
		return nil, status.Errorf(codes.InvalidArgument, "no valid stream version given")
	}
	if len(req.GetKey()) != 16 {
		return nil, status.Errorf(codes.InvalidArgument, "key must be 16 bytes")
	}

	data := map[string]any{
		"packageID":     req.GetPackageId(),
		"streamID":      req.GetStreamId(),
		"streamVersion": version,
		"key":           req.GetKey(),
		"keyURI":        req.GetKeyUri(),
	}
	jID := r.RunAction([]actions.Action{actions.MkOfflinePackage}, nil, data,
		r.log.With("package_id", req.GetPackageId(), "stream_id", req.GetStreamId(), "stream_version", version))
	r.log.Info("offline package job added", "ID", jID)

	return &protobuf.OfflinePackageResponse{JobId: ptr.Take(jID)}, nil
}

// DeleteOfflinePackage removes an offline package. A package that is gone already is
// not an error: the device that asked may have retried after a lost answer.
func (r *Runner) DeleteOfflinePackage(_ context.Context, req *protobuf.DeleteOfflinePackageRequest) (*protobuf.DeleteOfflinePackageResponse, error) {
	if req.GetPackageId() == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "no package id given")
	}
	if err := os.RemoveAll(actions.OfflinePackageDir(req.GetPackageId())); err != nil {
		return nil, status.Errorf(codes.Internal, "can't delete offline package: %v", err)
	}
	return &protobuf.DeleteOfflinePackageResponse{}, nil
}

// PrepareUpload reserves a slot for a video file upload. The file is then POSTed to the
// upload endpoint with the returned token; the result is delivered asynchronously in
// VODReady and ThumbnailReady notifications, so nothing is started here.
func (r *Runner) PrepareUpload(_ context.Context, req *protobuf.PrepareUploadRequest) (*protobuf.PrepareUploadResponse, error) {
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
	return &protobuf.PrepareUploadResponse{
		Token:      ptr.Take(token),
		UploadPort: ptr.Take(int32(config.Config.UploadPort)),
	}, nil
}
