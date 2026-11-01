package runner_manager

import (
	"context"
	"math"
	"strconv"

	"github.com/tum-dev/gocast/runner/protobuf"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// saveDuration stores the length a runner probed from a VOD as the stream's duration.
//
// A stream has one duration but up to three VODs, which differ by a few seconds at most.
// The combined one is the one viewers watch by default, so it wins: a camera or slide
// VOD only sets the duration while the stream has none or no combined VOD to say better,
// and never replaces the combined one's. Repeating a notification is harmless, it writes
// the same value.
func (m *Manager) saveDuration(ctx context.Context, req *protobuf.DurationProbedNotification) error {
	seconds := req.GetDurationSeconds()
	// The int32 column cannot hold more, and 0 is how a runner says it found nothing.
	if seconds == 0 || seconds > math.MaxInt32 {
		return status.Errorf(codes.InvalidArgument, "duration of %d seconds", seconds)
	}
	streamID := strconv.FormatUint(req.GetStream().GetId(), 10)
	stream, err := m.dao.StreamsDao.GetStreamByID(ctx, streamID)
	if err != nil {
		return lookupError(err, "stream %s", streamID)
	}
	if req.GetStreamVersion() != protobuf.StreamVersion_STREAM_VERSION_COMBINED &&
		stream.PlaylistUrl != "" && stream.Duration.Valid && stream.Duration.Int32 > 0 {
		return nil
	}
	if err := m.dao.StreamsDao.SetDuration(stream.ID, seconds); err != nil {
		return status.Errorf(codes.Internal, "can't save duration: %v", err)
	}
	return nil
}
