package runner_manager

import (
	"context"
	"errors"
	"net"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"go.uber.org/mock/gomock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"

	"github.com/tum-dev/gocast/runner/pkg/ptr"
	"github.com/tum-dev/gocast/runner/protobuf"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
)

type fakeUploadServer struct {
	protobuf.UnimplementedRunnerServiceServer
	got *protobuf.PrepareUploadRequest
}

func (f *fakeUploadServer) PrepareUpload(_ context.Context, req *protobuf.PrepareUploadRequest) (*protobuf.PrepareUploadResponse, error) {
	f.got = req
	return &protobuf.PrepareUploadResponse{Token: ptr.Take("tok"), UploadPort: ptr.Take(int32(8188))}, nil
}

func TestRequestUpload(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = lis.Close() }()
	fake := &fakeUploadServer{}
	srv := grpc.NewServer()
	protobuf.RegisterRunnerServiceServer(srv, fake)
	go func() { _ = srv.Serve(lis) }()
	defer srv.Stop()

	host, portStr, _ := net.SplitHostPort(lis.Addr().String())
	port, _ := strconv.Atoi(portStr)

	ctrl := gomock.NewController(t)
	runnerDao := mock_dao.NewMockRunnerDao(ctrl)
	runnerDao.EXPECT().ReserveRunner(gomock.Any()).Return(model.Runner{Hostname: host, Port: uint32(port)}, nil)
	m := New(dao.DaoWrapper{RunnerDao: runnerDao})

	slot, err := m.RequestUpload(context.Background(), 12, model.PRES)
	if err != nil {
		t.Fatalf("RequestUpload: %v", err)
	}
	// The runner is reached by the host it registered with; only the port differs from gRPC's.
	if slot != (UploadSlot{Host: host, Port: 8188, Token: "tok"}) {
		t.Errorf("slot = %+v", slot)
	}
	if fake.got.GetStreamId() != 12 || fake.got.GetVersion() != protobuf.StreamVersion_STREAM_VERSION_PRESENTATION {
		t.Errorf("runner was asked for %v", fake.got)
	}
}

// TestRequestUpload_NoRunner checks that the empty runner ReserveRunner returns when
// none is alive becomes an error instead of a dial to an empty host.
func TestRequestUpload_NoRunner(t *testing.T) {
	ctrl := gomock.NewController(t)
	runnerDao := mock_dao.NewMockRunnerDao(ctrl)
	runnerDao.EXPECT().ReserveRunner(gomock.Any()).Return(model.Runner{}, nil)
	m := New(dao.DaoWrapper{RunnerDao: runnerDao})

	if _, err := m.RequestUpload(context.Background(), 12, model.COMB); !errors.Is(err, ErrNoRunner) {
		t.Errorf("RequestUpload = %v, want ErrNoRunner", err)
	}
}

func TestHandleVODReady(t *testing.T) {
	notification := func(v protobuf.StreamVersion) *protobuf.VODReadyNotification {
		return &protobuf.VODReadyNotification{
			Stream:        &protobuf.StreamInfo{Id: ptr.Take(uint64(5))},
			StreamVersion: ptr.Take(v),
			Url:           ptr.Take("http://edge/vod/5/x/playlist.m3u8"),
		}
	}

	t.Run("first VOD follows the course setting", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		streamsDao := mock_dao.NewMockStreamsDao(ctrl)
		streamsDao.EXPECT().GetStreamByID(gomock.Any(), "5").Return(model.Stream{Model: gorm.Model{ID: 5}, CourseID: 3}, nil)
		coursesDao := mock_dao.NewMockCoursesDao(ctrl)
		coursesDao.EXPECT().GetCourseById(gomock.Any(), uint(3)).Return(model.Course{VodPrivate: true}, nil)
		var saved model.Stream
		streamsDao.EXPECT().SaveStream(gomock.Any()).DoAndReturn(func(s *model.Stream) error { saved = *s; return nil })
		streamsDao.EXPECT().RemoveTranscodingProgress(gomock.Any(), uint(5)).Return(nil)

		m := New(dao.DaoWrapper{StreamsDao: streamsDao, CoursesDao: coursesDao})
		if _, err := m.handleVODReady(context.Background(), notification(protobuf.StreamVersion_STREAM_VERSION_CAMERA)); err != nil {
			t.Fatal(err)
		}
		if !saved.Recording || !saved.Private || saved.PlaylistUrlCAM == "" {
			t.Errorf("saved = %+v", saved)
		}
	})

	t.Run("a stream made private beforehand stays private", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		streamsDao := mock_dao.NewMockStreamsDao(ctrl)
		streamsDao.EXPECT().GetStreamByID(gomock.Any(), "5").Return(model.Stream{Model: gorm.Model{ID: 5}, CourseID: 3, Private: true}, nil)
		coursesDao := mock_dao.NewMockCoursesDao(ctrl)
		coursesDao.EXPECT().GetCourseById(gomock.Any(), uint(3)).Return(model.Course{VodPrivate: false}, nil)
		var saved model.Stream
		streamsDao.EXPECT().SaveStream(gomock.Any()).DoAndReturn(func(s *model.Stream) error { saved = *s; return nil })
		streamsDao.EXPECT().RemoveTranscodingProgress(gomock.Any(), uint(5)).Return(nil)

		m := New(dao.DaoWrapper{StreamsDao: streamsDao, CoursesDao: coursesDao})
		if _, err := m.handleVODReady(context.Background(), notification(protobuf.StreamVersion_STREAM_VERSION_COMBINED)); err != nil {
			t.Fatal(err)
		}
		if !saved.Private {
			t.Error("the stream became public")
		}
	})

	t.Run("later VODs keep the visibility", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		streamsDao := mock_dao.NewMockStreamsDao(ctrl)
		streamsDao.EXPECT().GetStreamByID(gomock.Any(), "5").Return(model.Stream{Model: gorm.Model{ID: 5}, Recording: true}, nil)
		var saved model.Stream
		streamsDao.EXPECT().SaveStream(gomock.Any()).DoAndReturn(func(s *model.Stream) error { saved = *s; return nil })
		streamsDao.EXPECT().RemoveTranscodingProgress(gomock.Any(), uint(5)).Return(nil)

		// no CoursesDao: asking the course would be a mistake here
		m := New(dao.DaoWrapper{StreamsDao: streamsDao})
		if _, err := m.handleVODReady(context.Background(), notification(protobuf.StreamVersion_STREAM_VERSION_COMBINED)); err != nil {
			t.Fatal(err)
		}
		if saved.Private || saved.PlaylistUrl == "" {
			t.Errorf("saved = %+v", saved)
		}
	})

	t.Run("deleted stream is acknowledged", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		streamsDao := mock_dao.NewMockStreamsDao(ctrl)
		streamsDao.EXPECT().GetStreamByID(gomock.Any(), "5").Return(model.Stream{}, gorm.ErrRecordNotFound)

		m := New(dao.DaoWrapper{StreamsDao: streamsDao})
		if _, err := m.handleVODReady(context.Background(), notification(protobuf.StreamVersion_STREAM_VERSION_COMBINED)); err != nil {
			t.Errorf("handleVODReady = %v, want nil so the runner stops retrying", err)
		}
	})
}

// TestHandleVODReadyEndsTheConversion checks that the card stops showing the version
// that is done, and only that one.
func TestHandleVODReadyEndsTheConversion(t *testing.T) {
	ctrl := gomock.NewController(t)
	streamsDao := mock_dao.NewMockStreamsDao(ctrl)
	streamsDao.EXPECT().GetStreamByID(gomock.Any(), "5").Return(model.Stream{Model: gorm.Model{ID: 5}, Recording: true}, nil)
	streamsDao.EXPECT().SaveStream(gomock.Any()).Return(nil)
	streamsDao.EXPECT().RemoveTranscodingProgress(model.PRES, uint(5)).Return(nil)

	m := New(dao.DaoWrapper{StreamsDao: streamsDao})
	_, err := m.handleVODReady(context.Background(), &protobuf.VODReadyNotification{
		Stream:        &protobuf.StreamInfo{Id: ptr.Take(uint64(5))},
		StreamVersion: ptr.Take(protobuf.StreamVersion_STREAM_VERSION_PRESENTATION),
		Url:           ptr.Take("http://edge/vod/5/x/playlist.m3u8"),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestHandleTranscodingProgress(t *testing.T) {
	report := func(progress int32) *protobuf.TranscodingProgressNotification {
		return &protobuf.TranscodingProgressNotification{
			Stream:        &protobuf.StreamInfo{Id: ptr.Take(uint64(5))},
			StreamVersion: ptr.Take(protobuf.StreamVersion_STREAM_VERSION_CAMERA),
			Progress:      ptr.Take(progress),
		}
	}
	for name, c := range map[string]struct {
		in   int32
		want int
	}{
		"in range": {42, 42},
		"over":     {250, 99}, // 100 is VODReady's to say
		"negative": {-3, 0},
	} {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			streamsDao := mock_dao.NewMockStreamsDao(ctrl)
			streamsDao.EXPECT().UpdateTranscodingProgress(model.CAM, uint(5), c.want).Return(nil)

			m := New(dao.DaoWrapper{StreamsDao: streamsDao})
			if _, err := m.handleTranscodingProgress(context.Background(), report(c.in)); err != nil {
				t.Fatal(err)
			}
		})
	}

	t.Run("a failing database is retried by the runner's next report", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		streamsDao := mock_dao.NewMockStreamsDao(ctrl)
		streamsDao.EXPECT().UpdateTranscodingProgress(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("db down"))

		m := New(dao.DaoWrapper{StreamsDao: streamsDao})
		if _, err := m.handleTranscodingProgress(context.Background(), report(1)); err == nil {
			t.Error("handleTranscodingProgress = nil, want the error")
		}
	})
}

func TestHandleVODFailed(t *testing.T) {
	failure := func(reason string) *protobuf.VODFailedNotification {
		return &protobuf.VODFailedNotification{
			Stream:        &protobuf.StreamInfo{Id: ptr.Take(uint64(5))},
			StreamVersion: ptr.Take(protobuf.StreamVersion_STREAM_VERSION_PRESENTATION),
			Reason:        ptr.Take(reason),
		}
	}

	t.Run("is kept for the uploader", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		streamsDao := mock_dao.NewMockStreamsDao(ctrl)
		streamsDao.EXPECT().GetStreamByID(gomock.Any(), "5").Return(model.Stream{Model: gorm.Model{ID: 5}}, nil)
		var saved model.TranscodingProgress
		streamsDao.EXPECT().SaveTranscodingProgress(gomock.Any()).DoAndReturn(func(p model.TranscodingProgress) error { saved = p; return nil })

		m := New(dao.DaoWrapper{StreamsDao: streamsDao})
		if _, err := m.handleVODFailed(context.Background(), failure("The file contains no video.")); err != nil {
			t.Fatal(err)
		}
		want := model.TranscodingProgress{StreamID: 5, Version: model.PRES, Failed: true, Error: "The file contains no video."}
		if saved != want {
			t.Errorf("saved %+v, want %+v", saved, want)
		}
	})

	t.Run("a reason too long for its column is cut", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		streamsDao := mock_dao.NewMockStreamsDao(ctrl)
		streamsDao.EXPECT().GetStreamByID(gomock.Any(), "5").Return(model.Stream{Model: gorm.Model{ID: 5}}, nil)
		var saved model.TranscodingProgress
		streamsDao.EXPECT().SaveTranscodingProgress(gomock.Any()).DoAndReturn(func(p model.TranscodingProgress) error { saved = p; return nil })

		m := New(dao.DaoWrapper{StreamsDao: streamsDao})
		if _, err := m.handleVODFailed(context.Background(), failure(strings.Repeat("ä", 1000))); err != nil {
			t.Fatal(err)
		}
		if n := utf8.RuneCountInString(saved.Error); n != maxProgressError {
			t.Errorf("saved %d characters, want %d", n, maxProgressError)
		}
	})

	t.Run("deleted stream is acknowledged", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		streamsDao := mock_dao.NewMockStreamsDao(ctrl)
		streamsDao.EXPECT().GetStreamByID(gomock.Any(), "5").Return(model.Stream{}, gorm.ErrRecordNotFound)

		// no SaveTranscodingProgress: it would leave a row for a stream that is gone
		m := New(dao.DaoWrapper{StreamsDao: streamsDao})
		if _, err := m.handleVODFailed(context.Background(), failure("x")); err != nil {
			t.Errorf("handleVODFailed = %v, want nil so the runner stops retrying", err)
		}
	})

	t.Run("a failing database is retried", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		streamsDao := mock_dao.NewMockStreamsDao(ctrl)
		streamsDao.EXPECT().GetStreamByID(gomock.Any(), "5").Return(model.Stream{Model: gorm.Model{ID: 5}}, nil)
		streamsDao.EXPECT().SaveTranscodingProgress(gomock.Any()).Return(errors.New("db down"))

		m := New(dao.DaoWrapper{StreamsDao: streamsDao})
		if _, err := m.handleVODFailed(context.Background(), failure("x")); err == nil {
			t.Error("handleVODFailed = nil, want the error so the runner retries")
		}
	})
}

// The runner drops a notification the manager answers NotFound to and retries the rest, so
// a lookup that failed for another reason than the record being gone must not say NotFound.
func TestSaveThumbnailLookupFailures(t *testing.T) {
	req := &protobuf.ThumbnailReadyNotification{
		Stream:        &protobuf.StreamInfo{Id: ptr.Take(uint64(5))},
		StreamVersion: ptr.Take(protobuf.StreamVersion_STREAM_VERSION_COMBINED),
	}
	for name, tc := range map[string]struct {
		err  error
		want codes.Code
	}{
		"the stream does not exist":   {gorm.ErrRecordNotFound, codes.NotFound},
		"the database is unreachable": {errors.New("driver: bad connection"), codes.Internal},
	} {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			streamsDao := mock_dao.NewMockStreamsDao(ctrl)
			streamsDao.EXPECT().GetStreamByID(gomock.Any(), "5").Return(model.Stream{}, tc.err)
			m := New(dao.DaoWrapper{StreamsDao: streamsDao})
			if got := status.Code(m.saveThumbnail(context.Background(), req)); got != tc.want {
				t.Errorf("code = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestHeartbeatLookupFailures(t *testing.T) {
	hb := &protobuf.Notification{Data: &protobuf.Notification_Heartbeat{Heartbeat: &protobuf.HeartbeatNotification{Hostname: ptr.Take("r1")}}}
	for name, tc := range map[string]struct {
		err  error
		want codes.Code
	}{
		"the runner is unknown":       {gorm.ErrRecordNotFound, codes.NotFound},
		"the database is unreachable": {errors.New("driver: bad connection"), codes.Internal},
	} {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			runnerDao := mock_dao.NewMockRunnerDao(ctrl)
			runnerDao.EXPECT().Get(gomock.Any(), "r1").Return(model.Runner{}, tc.err)
			m := New(dao.DaoWrapper{RunnerDao: runnerDao})
			_, err := m.Notify(context.Background(), hb)
			if got := status.Code(err); got != tc.want {
				t.Errorf("code = %v, want %v", got, tc.want)
			}
		})
	}
}
