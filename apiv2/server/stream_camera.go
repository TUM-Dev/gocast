package apiv2

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	e "github.com/TUM-Dev/gocast/apiv2/errors"
	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
)

var (
	errStreamNotLive          = errors.New("the stream is not live")
	errStreamHasNoLectureHall = errors.New("the stream has no lecture hall")
)

// SwitchCameraPreset points the camera of a live stream's lecture hall at a preset,
// replacing v1's switchPreset. The course-admin policy has already checked the course;
// this checks that the stream is that course's, and takes the hall from the stream
// rather than the request. v1 took both from the URL and checked neither, so any
// course's administrators could move any room's camera.
func (a *API) SwitchCameraPreset(ctx context.Context, req *protobuf.SwitchCameraPresetRequest) (*emptypb.Empty, error) {
	stream, err := a.dao.GetStreamByID(ctx, fmt.Sprintf("%d", req.GetStreamId()))
	if err != nil {
		return nil, e.FromGorm(err, "can't find stream")
	}

	// Same answer as a missing stream, as authorizeCourseAdmin does for courses.
	if stream.CourseID != uint(req.GetCourseId()) {
		return nil, e.WithStatus(http.StatusNotFound, errors.New("no such stream"))
	}

	// Moving the camera of a hall that is not streaming would only disturb whoever
	// is using the room.
	if !stream.LiveNow {
		return nil, e.WithStatus(http.StatusBadRequest, errStreamNotLive)
	}

	if stream.LectureHallID == 0 {
		return nil, e.WithStatus(http.StatusBadRequest, errStreamHasNoLectureHall)
	}

	preset, err := a.dao.LectureHallsDao.FindPreset(
		fmt.Sprintf("%d", stream.LectureHallID), fmt.Sprintf("%d", req.GetPresetId()),
	)
	if err != nil {
		return nil, e.FromGorm(err, "can not find preset")
	}

	lectureHall, err := a.dao.LectureHallsDao.GetLectureHallByID(stream.LectureHallID)
	if err != nil {
		return nil, e.FromGorm(err, "can not find lecture hall")
	}

	cam, err := a.cameraFor(lectureHall)
	if err != nil {
		return nil, err
	}

	if err := cam.SetPreset(preset.PresetID); err != nil {
		return nil, e.WithStatus(http.StatusServiceUnavailable, err)
	}

	// Answering only once the camera has arrived is what lets the page show the
	// switch as in progress until the picture has actually changed.
	select {
	case <-ctx.Done():
		return nil, status.FromContextError(ctx.Err()).Err()
	case <-time.After(camSwitchDelay):
	}

	return &emptypb.Empty{}, nil
}
