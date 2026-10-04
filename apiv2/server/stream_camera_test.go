package apiv2

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"

	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
)

func TestSwitchCameraPreset(t *testing.T) {
	restore := camSwitchDelay
	camSwitchDelay = time.Millisecond
	defer func() { camSwitchDelay = restore }()

	// Stream 7 is course 1's, live, in hall 3.
	liveStream := model.Stream{Model: gorm.Model{ID: 7}, CourseID: 1, LiveNow: true, LectureHallID: 3}
	hall := model.LectureHall{Model: gorm.Model{ID: 3}, CameraIP: "10.0.0.5"}

	// setup wires a stream lookup that answers with the given stream, and a hall
	// mock the subtest can add expectations to. Unset expectations fail the test,
	// which is how the refusals below prove the camera was never reached.
	setup := func(t *testing.T, stream model.Stream, streamErr error) (*API, *mock_dao.MockLectureHallsDao, *fakeCam) {
		ctrl := gomock.NewController(t)
		streamsMock := mock_dao.NewMockStreamsDao(ctrl)
		streamsMock.EXPECT().GetStreamByID(gomock.Any(), "7").Return(stream, streamErr).Times(1)
		lhMock := mock_dao.NewMockLectureHallsDao(ctrl)

		cam := &fakeCam{}
		api := &API{
			dao:  dao.DaoWrapper{StreamsDao: streamsMock, LectureHallsDao: lhMock},
			log:  slog.Default(),
			cams: &fakeCamService{cam: cam},
		}
		return api, lhMock, cam
	}

	req := &protobuf.SwitchCameraPresetRequest{CourseId: 1, StreamId: 7, PresetId: 2}

	t.Run("moves the camera of the stream's own hall", func(t *testing.T) {
		api, lhMock, cam := setup(t, liveStream, nil)
		lhMock.EXPECT().FindPreset("3", "2").
			Return(model.CameraPreset{PresetID: 2, LectureHallID: 3}, nil).Times(1)
		lhMock.EXPECT().GetLectureHallByID(uint(3)).Return(hall, nil).Times(1)

		if _, err := api.SwitchCameraPreset(context.Background(), req); err != nil {
			t.Fatalf("SwitchCameraPreset: %v", err)
		}
		if len(cam.setPresetIDs) != 1 || cam.setPresetIDs[0] != 2 {
			t.Errorf("SetPreset calls = %v, want [2]", cam.setPresetIDs)
		}
	})

	t.Run("does not reach another course's stream", func(t *testing.T) {
		other := liveStream
		other.CourseID = 2
		api, _, cam := setup(t, other, nil)

		_, err := api.SwitchCameraPreset(context.Background(), req)
		if status.Code(err) != codes.NotFound {
			t.Fatalf("code = %v, want NotFound", status.Code(err))
		}
		if len(cam.setPresetIDs) != 0 {
			t.Error("moved the camera of a stream outside the authorized course")
		}
	})

	t.Run("404s a missing stream", func(t *testing.T) {
		api, _, _ := setup(t, model.Stream{}, gorm.ErrRecordNotFound)

		_, err := api.SwitchCameraPreset(context.Background(), req)
		if status.Code(err) != codes.NotFound {
			t.Fatalf("code = %v, want NotFound", status.Code(err))
		}
	})

	t.Run("refuses a stream that is not live", func(t *testing.T) {
		ended := liveStream
		ended.LiveNow = false
		api, _, _ := setup(t, ended, nil)

		_, err := api.SwitchCameraPreset(context.Background(), req)
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("code = %v, want InvalidArgument", status.Code(err))
		}
	})

	t.Run("refuses a stream with no lecture hall", func(t *testing.T) {
		hallless := liveStream
		hallless.LectureHallID = 0
		api, _, _ := setup(t, hallless, nil)

		_, err := api.SwitchCameraPreset(context.Background(), req)
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("code = %v, want InvalidArgument", status.Code(err))
		}
	})

	t.Run("404s a preset the stream's hall does not have", func(t *testing.T) {
		api, lhMock, _ := setup(t, liveStream, nil)
		lhMock.EXPECT().FindPreset("3", "2").Return(model.CameraPreset{}, gorm.ErrRecordNotFound).Times(1)

		_, err := api.SwitchCameraPreset(context.Background(), req)
		if status.Code(err) != codes.NotFound {
			t.Fatalf("code = %v, want NotFound", status.Code(err))
		}
	})

	t.Run("an unreachable camera is Unavailable", func(t *testing.T) {
		api, lhMock, _ := setup(t, liveStream, nil)
		lhMock.EXPECT().FindPreset("3", "2").
			Return(model.CameraPreset{PresetID: 2, LectureHallID: 3}, nil).Times(1)
		lhMock.EXPECT().GetLectureHallByID(uint(3)).Return(hall, nil).Times(1)
		api.cams = &fakeCamService{err: errors.New("connection refused")}

		_, err := api.SwitchCameraPreset(context.Background(), req)
		if status.Code(err) != codes.Unavailable {
			t.Fatalf("code = %v, want Unavailable", status.Code(err))
		}
	})

	t.Run("cancellation during the switch delay is reported", func(t *testing.T) {
		api, lhMock, _ := setup(t, liveStream, nil)
		lhMock.EXPECT().FindPreset("3", "2").
			Return(model.CameraPreset{PresetID: 2, LectureHallID: 3}, nil).Times(1)
		lhMock.EXPECT().GetLectureHallByID(uint(3)).Return(hall, nil).Times(1)

		camSwitchDelay = time.Hour
		defer func() { camSwitchDelay = time.Millisecond }()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := api.SwitchCameraPreset(ctx, req)
		if status.Code(err) != codes.Canceled {
			t.Fatalf("code = %v, want Canceled", status.Code(err))
		}
	})
}
