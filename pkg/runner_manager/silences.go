package runner_manager

import (
	"context"
	"strconv"

	"github.com/tum-dev/gocast/runner/protobuf"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/TUM-Dev/gocast/model"
)

// silenceDetectionVersion is the one version of a stream whose runner scans for silences.
// All versions of a stream carry the same audio, so scanning each would repeat the work.
// The combined version is preferred, then the camera, which has the lecturer's microphone;
// the presentation feed comes last because it may have no audio at all, which would read as
// one silence over the whole lecture. A hall without any source yields no version.
func silenceDetectionVersion(lh model.LectureHall) protobuf.StreamVersion {
	switch {
	case lh.CombIP != "":
		return protobuf.StreamVersion_STREAM_VERSION_COMBINED
	case lh.CamIP != "":
		return protobuf.StreamVersion_STREAM_VERSION_CAMERA
	case lh.PresIP != "":
		return protobuf.StreamVersion_STREAM_VERSION_PRESENTATION
	}
	return protobuf.StreamVersion(-1)
}

// saveSilences replaces the stream's silences with the reported ones, none included: a
// scan that found no silence (again) has to remove what an earlier one found. The runner
// repeats a notification it is unsure was delivered, and replacing makes that harmless.
func (m *Manager) saveSilences(ctx context.Context, req *protobuf.SilenceDetectedNotification) error {
	if len(req.GetStarts()) != len(req.GetEnds()) {
		return status.Errorf(codes.InvalidArgument, "%d silence starts but %d ends", len(req.GetStarts()), len(req.GetEnds()))
	}
	streamID := strconv.FormatUint(req.GetStream().GetId(), 10)
	stream, err := m.dao.StreamsDao.GetStreamByID(ctx, streamID)
	if err != nil {
		return lookupError(err, "stream %s", streamID)
	}
	if len(req.GetStarts()) == 0 {
		// UpdateSilences saves the list it is given, which gorm refuses to do when empty.
		if err := m.dao.StreamsDao.DeleteSilences(streamID); err != nil {
			return status.Errorf(codes.Internal, "can't delete silences: %v", err)
		}
		return nil
	}

	silences := make([]model.Silence, len(req.GetStarts()))
	for i := range silences {
		silences[i] = model.Silence{Start: uint(req.GetStarts()[i]), End: uint(req.GetEnds()[i]), StreamID: stream.ID}
	}
	if err := m.dao.StreamsDao.UpdateSilences(silences, streamID); err != nil {
		return status.Errorf(codes.Internal, "can't save silences: %v", err)
	}
	return nil
}
