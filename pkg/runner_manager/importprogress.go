package runner_manager

import (
	"context"
	"errors"
	"strconv"
	"unicode/utf8"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"

	"github.com/tum-dev/gocast/runner/protobuf"

	"github.com/TUM-Dev/gocast/model"
)

// maxProgressError is the width of the column the reason is stored in.
const maxProgressError = 255

// handleTranscodingProgress moves the percentage of a conversion the upload started.
// The row is made by the upload itself and removed by VODReady, and a report for a
// version with no row, or one that failed, changes nothing: the runner sends these
// without retrying, but they can still overtake the notification that ended the
// conversion, and must not bring it back.
func (m *Manager) handleTranscodingProgress(_ context.Context, n *protobuf.TranscodingProgressNotification) (*protobuf.NotificationResponse, error) {
	progress := min(max(int(n.GetProgress()), 0), 99)
	err := m.dao.StreamsDao.UpdateTranscodingProgress(modelStreamVersion(n.GetStreamVersion()), uint(n.GetStream().GetId()), progress)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "can't save the transcoding progress: %v", err)
	}
	return &protobuf.NotificationResponse{}, nil
}

// handleVODFailed records that no VOD could be made of an upload, so that the lecture
// says so instead of staying a placeholder. The row is kept until the next upload of
// the version replaces it.
func (m *Manager) handleVODFailed(ctx context.Context, n *protobuf.VODFailedNotification) (*protobuf.NotificationResponse, error) {
	streamID := n.GetStream().GetId()
	version := modelStreamVersion(n.GetStreamVersion())

	_, err := m.dao.StreamsDao.GetStreamByID(ctx, strconv.FormatUint(streamID, 10))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Deleted while its upload was converted. The runner retries every error it gets
		// back and retrying cannot bring the stream back, see handleVODReady.
		m.logger.Warn("dropping the failure of a VOD for a stream that no longer exists", "stream", streamID)
		return &protobuf.NotificationResponse{}, nil
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "can't get stream %d: %v", streamID, err)
	}

	reason := n.GetReason()
	if utf8.RuneCountInString(reason) > maxProgressError {
		reason = string([]rune(reason)[:maxProgressError])
	}
	m.logger.Warn("a VOD could not be made", "stream", streamID, "version", version, "reason", reason)
	err = m.dao.StreamsDao.SaveTranscodingProgress(model.TranscodingProgress{
		StreamID: uint(streamID), Version: version, Failed: true, Error: reason,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "can't save the failure: %v", err)
	}
	return &protobuf.NotificationResponse{}, nil
}
