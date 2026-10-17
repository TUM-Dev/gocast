package runner_manager

import (
	"context"
	"fmt"

	"github.com/tum-dev/gocast/runner/pkg/ptr"
	"github.com/tum-dev/gocast/runner/protobuf"

	"github.com/TUM-Dev/gocast/model"
)

// RegenerateThumbnailRequest describes the VOD whose thumbnail should be regenerated.
// PlaylistURL must be signed, as for SectionImageRequest.
type RegenerateThumbnailRequest struct {
	StreamID    uint
	Version     model.StreamVersion
	PlaylistURL string
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

// RequestRegenerateThumbnail asks a runner to regenerate the thumbnail for an
// already-recorded VOD. The runner answers asynchronously with a
// ThumbnailReadyNotification, which saveThumbnail then stores, so this returns as
// soon as the job is accepted.
func (m *Manager) RequestRegenerateThumbnail(ctx context.Context, req RegenerateThumbnailRequest) error {
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
		StreamId:    ptr.Take(uint64(req.StreamID)),
		Version:     ptr.Take(protobufStreamVersion(req.Version)),
		PlaylistUrl: ptr.Take(req.PlaylistURL),
	})
	if err != nil {
		return fmt.Errorf("RegenerateThumbnail: %w", err)
	}
	m.logger.With("stream", req.StreamID, "job", resp.GetJobId(), "runner", runner.Hostname).
		Info("requested thumbnail regeneration")
	return nil
}
