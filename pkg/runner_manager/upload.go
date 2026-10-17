package runner_manager

import (
	"context"
	"fmt"

	"github.com/tum-dev/gocast/runner/pkg/ptr"
	"github.com/tum-dev/gocast/runner/protobuf"

	"github.com/TUM-Dev/gocast/model"
)

// UploadSlot is where and how one video file can be sent to a runner.
type UploadSlot struct {
	// Host and Port are the runner's HTTP upload endpoint.
	Host string
	Port int
	// Token is single use and only valid for a short time.
	Token string
}

// RequestUpload reserves a runner for an upload of one version of a stream's VOD.
// The caller sends the file to the returned slot; the runner then converts it and
// reports the result in a VODReadyNotification and a ThumbnailReadyNotification, which
// the manager stores like those of a recorded stream.
func (m *Manager) RequestUpload(ctx context.Context, streamID uint, version model.StreamVersion) (UploadSlot, error) {
	runner, client, conn, err := m.getClient(ctx)
	if err != nil {
		return UploadSlot{}, fmt.Errorf("getClient: %w", err)
	}
	defer func() {
		_ = conn.Close()
	}()

	ctx, cancel := context.WithTimeout(ctx, runnerDispatchTimeout)
	defer cancel()

	resp, err := client.PrepareUpload(ctx, &protobuf.PrepareUploadRequest{
		StreamId: ptr.Take(uint64(streamID)),
		Version:  ptr.Take(protobufStreamVersion(version)),
	})
	if err != nil {
		return UploadSlot{}, fmt.Errorf("PrepareUpload: %w", err)
	}
	m.logger.With("stream", streamID, "version", version, "runner", runner.Hostname).Info("reserved upload")
	return UploadSlot{Host: runner.Hostname, Port: int(resp.GetUploadPort()), Token: resp.GetToken()}, nil
}
