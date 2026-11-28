package runner_manager

import (
	"context"
	"fmt"
	"strconv"

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
func (m *Manager) RequestUpload(ctx context.Context, streamID uint, version model.StreamVersion) (slot UploadSlot, err error) {
	ctx, span := startSpan(withStreamTrace(ctx, uint64(streamID), protobufStreamVersion(version)), "manager.request_upload",
		streamAttrs(uint64(streamID), protobufStreamVersion(version))...)
	defer endSpan(span, &err)
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
		StreamId:      ptr.Take(uint64(streamID)),
		Version:       ptr.Take(protobufStreamVersion(version)),
		DetectSilence: ptr.Take(m.detectSilenceOnUpload(ctx, streamID, version)),
	})
	if err != nil {
		return UploadSlot{}, fmt.Errorf("PrepareUpload: %w", err)
	}
	m.logger.With("stream", streamID, "version", version, "runner", runner.Hostname).Info("reserved upload")
	return UploadSlot{Host: runner.Hostname, Port: int(resp.GetUploadPort()), Token: resp.GetToken()}, nil
}

// detectSilenceOnUpload tells whether the runner should scan this upload for silences.
// Versions of a stream share their audio, so one scan is enough: this upload is scanned
// unless a version that silenceDetectionVersion would prefer already has a VoD. The
// reverse order is fine, a better version uploaded later simply replaces the silences.
// A failed lookup means no scan; silences are not worth failing or guessing for.
func (m *Manager) detectSilenceOnUpload(ctx context.Context, streamID uint, version model.StreamVersion) bool {
	stream, err := m.dao.StreamsDao.GetStreamByID(ctx, strconv.FormatUint(uint64(streamID), 10))
	if err != nil {
		m.logger.Warn("not scanning upload for silences, stream lookup failed", "stream", streamID, "err", err)
		return false
	}
	switch version {
	case model.COMB:
		return true
	case model.CAM:
		return stream.PlaylistUrl == ""
	default:
		return stream.PlaylistUrl == "" && stream.PlaylistUrlCAM == ""
	}
}
