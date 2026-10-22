package runner_manager

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/mock/gomock"
	"gorm.io/gorm"

	"github.com/tum-dev/gocast/runner/pkg/ptr"
	"github.com/tum-dev/gocast/runner/protobuf"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/pkg/camera"
)

type fakeCam struct {
	camera.Cam
	setPreset *atomic.Int32
	err       error
}

func (f fakeCam) SetPreset(int) error {
	f.setPreset.Add(1)
	return f.err
}

type fakeCamService struct{ cam fakeCam }

func (f fakeCamService) For(string, model.CameraType) (camera.Cam, error) { return f.cam, nil }

func newCameraManager(t *testing.T, camErr error) (*Manager, *atomic.Int32) {
	t.Helper()
	ctrl := gomock.NewController(t)
	lectureHallsDao := mock_dao.NewMockLectureHallsDao(ctrl)
	lectureHallsDao.EXPECT().GetLectureHallByID(uint(1)).Return(model.LectureHall{
		CameraPresets: []model.CameraPreset{{PresetID: 4, IsDefault: true}},
	}, nil).AnyTimes()
	coursesDao := mock_dao.NewMockCoursesDao(ctrl)
	coursesDao.EXPECT().GetCourseById(gomock.Any(), gomock.Any()).Return(model.Course{}, nil).AnyTimes()

	calls := &atomic.Int32{}
	m := New(dao.DaoWrapper{LectureHallsDao: lectureHallsDao, CoursesDao: coursesDao},
		WithCamService(fakeCamService{fakeCam{setPreset: calls, err: camErr}}))
	return m, calls
}

// A camera that answers with an error used to have its mark cleared, so every redelivered
// StreamStart moved it again. Several streams of one hall then fought over the preset.
func TestHandleCamera_FailingCameraIsTriedAFewTimesOnly(t *testing.T) {
	m, calls := newCameraManager(t, errors.New("camera 10.0.0.1: unexpected camera response status: 204 No Content"))
	stream := model.Stream{Model: gorm.Model{ID: 9}, LectureHallID: 1}

	for i := 0; i < 10; i++ {
		_ = m.handleCamera(context.Background(), stream)
	}
	if got := calls.Load(); got != maxCameraAttempts {
		t.Errorf("preset was set %d times for a failing camera, want %d", got, maxCameraAttempts)
	}
}

// StreamStart arrives once per stream version; only the first may move the camera.
func TestHandleCamera_SuccessfulPresetIsSetOnce(t *testing.T) {
	m, calls := newCameraManager(t, nil)
	stream := model.Stream{Model: gorm.Model{ID: 9}, LectureHallID: 1}

	for i := 0; i < 3; i++ {
		if err := m.handleCamera(context.Background(), stream); err != nil {
			t.Fatal(err)
		}
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("preset was set %d times, want 1", got)
	}
}

// A StreamStart that is delivered after the scheduled end is a leftover of the runner's
// endless retries. It must neither touch the hall nor wait for it. The mocks have no
// expectation for the lecture hall, so any attempt fails the test.
func TestStreamStarted_AfterScheduledEndLeavesTheHallAlone(t *testing.T) {
	ctrl := gomock.NewController(t)
	streamsDao := mock_dao.NewMockStreamsDao(ctrl)
	streamsDao.EXPECT().GetStreamByID(gomock.Any(), "9").Return(model.Stream{
		Model: gorm.Model{ID: 9}, LectureHallID: 1, End: time.Now().Add(-time.Hour),
	}, nil)
	streamsDao.EXPECT().SavePRESURL(gomock.Any(), "http://edge/live/9/playlist.m3u8")

	m := New(dao.DaoWrapper{StreamsDao: streamsDao, LectureHallsDao: mock_dao.NewMockLectureHallsDao(ctrl)})

	err := m.streamStarted(context.Background(), &protobuf.StreamStartNotification{
		Stream:        &protobuf.StreamInfo{Id: ptr.Take(uint64(9))},
		StreamVersion: ptr.Take(protobuf.StreamVersion_STREAM_VERSION_PRESENTATION),
		Url:           ptr.Take("http://edge/live/9/playlist.m3u8"),
	})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond) // a hall setup in the background would have started by now
}

// The acknowledgement must not wait for the hall: the camera and lights can take many
// seconds, and a runner whose call is cut off sends the notification again.
func TestStreamStarted_DoesNotWaitForTheHall(t *testing.T) {
	ctrl := gomock.NewController(t)
	streamsDao := mock_dao.NewMockStreamsDao(ctrl)
	streamsDao.EXPECT().GetStreamByID(gomock.Any(), "9").Return(model.Stream{
		Model: gorm.Model{ID: 9}, LectureHallID: 1, End: time.Now().Add(time.Hour),
	}, nil)
	streamsDao.EXPECT().SaveCOMBURL(gomock.Any(), gomock.Any())

	release := make(chan struct{})
	reached := make(chan struct{})
	var once sync.Once
	lectureHallsDao := mock_dao.NewMockLectureHallsDao(ctrl)
	lectureHallsDao.EXPECT().GetLectureHallByID(uint(1)).DoAndReturn(func(uint) (model.LectureHall, error) {
		once.Do(func() { close(reached) })
		<-release
		return model.LectureHall{}, errors.New("hall is gone")
	}).AnyTimes()

	m := New(dao.DaoWrapper{StreamsDao: streamsDao, LectureHallsDao: lectureHallsDao})

	done := make(chan error, 1)
	go func() {
		done <- m.streamStarted(context.Background(), &protobuf.StreamStartNotification{
			Stream:        &protobuf.StreamInfo{Id: ptr.Take(uint64(9))},
			StreamVersion: ptr.Take(protobuf.StreamVersion_STREAM_VERSION_COMBINED),
			Url:           ptr.Take("http://edge/live/9/playlist.m3u8"),
		})
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("streamStarted waited for the lecture hall")
	}
	select {
	case <-reached:
	case <-time.After(2 * time.Second):
		t.Error("the hall was never set up")
	}
	close(release)
}
