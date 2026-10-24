package runner_manager

import (
	"context"
	"fmt"

	"github.com/tum-dev/gocast/runner/pkg/ptr"
	"github.com/tum-dev/gocast/runner/protobuf"

	"github.com/TUM-Dev/gocast/model"
)

// RedoPostProcessingRequest describes the VOD whose post-processing, its thumbnail and
// optionally its silences, should be redone.
// PlaylistURL must be signed, as for SectionImageRequest.
type RedoPostProcessingRequest struct {
	StreamID    uint
	Version     model.StreamVersion
	PlaylistURL string
	// DetectSilence also has the runner scan the VOD for silences.
	DetectSilence bool
}

// protobufStreamVersion maps a model.StreamVersion to the protobuf.StreamVersion the
// runner expects. The inverse of modelStreamVersion.
func protobufStreamVersion(version model.StreamVersion) protobuf.StreamVersion {
	switch version {
	case model.PRES:
		return protobuf.StreamVersion_STREAM_VERSION_PRESENTATION
	case model.CAM:
		return protobuf.StreamVersion_STREAM_VERSION_CAMERA
	default:
		return protobuf.StreamVersion_STREAM_VERSION_COMBINED
	}
}

// RequestRedoPostProcessing asks a runner to redo the post-processing of an
// already-recorded VOD. The runner answers asynchronously with a
// ThumbnailReadyNotification, which saveThumbnail then stores, and a
// SilenceDetectedNotification if silences were asked for, so this returns as soon as the
// job is accepted. The runner's RPC still has the name it was introduced with.
func (m *Manager) RequestRedoPostProcessing(ctx context.Context, req RedoPostProcessingRequest) error {
	runner, client, conn, err := m.getClient(ctx)
	if err != nil {
		return fmt.Errorf("getClient: %w", err)
	}
	defer func() {
		_ = conn.Close()
	}()

	ctx, cancel := context.WithTimeout(ctx, runnerDispatchTimeout)
	defer cancel()

	resp, err := client.RegenerateThumbnail(ctx, &protobuf.RegenerateThumbnailRequest{
		StreamId:      ptr.Take(uint64(req.StreamID)),
		Version:       ptr.Take(protobufStreamVersion(req.Version)),
		PlaylistUrl:   ptr.Take(req.PlaylistURL),
		DetectSilence: ptr.Take(req.DetectSilence),
	})
	if err != nil {
		return fmt.Errorf("RegenerateThumbnail: %w", err)
	}
	m.logger.With("stream", req.StreamID, "job", resp.GetJobId(), "runner", runner.Hostname).
		Info("requested post-processing")
	return nil
}
