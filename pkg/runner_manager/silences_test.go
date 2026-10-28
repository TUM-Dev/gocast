package runner_manager

import (
	"context"
	"errors"
	"testing"

	"github.com/tum-dev/gocast/runner/pkg/ptr"
	"github.com/tum-dev/gocast/runner/protobuf"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
)

func TestSilenceDetectionVersion(t *testing.T) {
	const (
		comb = protobuf.StreamVersion_STREAM_VERSION_COMBINED
		cam  = protobuf.StreamVersion_STREAM_VERSION_CAMERA
		pres = protobuf.StreamVersion_STREAM_VERSION_PRESENTATION
	)
	for name, tc := range map[string]struct {
		lh   model.LectureHall
		want protobuf.StreamVersion
	}{
		"all sources":      {model.LectureHall{CombIP: "a", CamIP: "b", PresIP: "c"}, comb},
		"camera only":      {model.LectureHall{CamIP: "b"}, cam},
		"camera and slide": {model.LectureHall{CamIP: "b", PresIP: "c"}, cam},
		"slides only":      {model.LectureHall{PresIP: "c"}, pres},
		"no source":        {model.LectureHall{}, protobuf.StreamVersion(-1)},
	} {
		t.Run(name, func(t *testing.T) {
			if got := silenceDetectionVersion(tc.lh); got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestSaveSilences(t *testing.T) {
	req := func(starts, ends []uint32) *protobuf.SilenceDetectedNotification {
		return &protobuf.SilenceDetectedNotification{
			Stream: &protobuf.StreamInfo{Id: ptr.Take(uint64(5))},
			Starts: starts,
			Ends:   ends,
		}
	}

	t.Run("replaces the stream's silences", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		streamsDao := mock_dao.NewMockStreamsDao(ctrl)
		streamsDao.EXPECT().GetStreamByID(gomock.Any(), "5").Return(model.Stream{Model: gorm.Model{ID: 5}}, nil)
		streamsDao.EXPECT().UpdateSilences([]model.Silence{{Start: 0, End: 40, StreamID: 5}, {Start: 100, End: 200, StreamID: 5}}, "5").Return(nil)
		m := New(dao.DaoWrapper{StreamsDao: streamsDao})
		if err := m.saveSilences(context.Background(), req([]uint32{0, 100}, []uint32{40, 200})); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("an empty list removes the old silences", func(t *testing.T) {
		streamsDao := mock_dao.NewMockStreamsDao(gomock.NewController(t))
		streamsDao.EXPECT().GetStreamByID(gomock.Any(), "5").Return(model.Stream{Model: gorm.Model{ID: 5}}, nil)
		streamsDao.EXPECT().DeleteSilences("5").Return(nil)
		m := New(dao.DaoWrapper{StreamsDao: streamsDao})
		if err := m.saveSilences(context.Background(), req(nil, nil)); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("mismatched lists are rejected for good", func(t *testing.T) {
		m := New(dao.DaoWrapper{StreamsDao: mock_dao.NewMockStreamsDao(gomock.NewController(t))})
		if got := status.Code(m.saveSilences(context.Background(), req([]uint32{1, 2}, []uint32{3}))); got != codes.InvalidArgument {
			t.Errorf("code = %v, want InvalidArgument", got)
		}
	})

	for name, tc := range map[string]struct {
		err  error
		want codes.Code
	}{
		"the stream does not exist":   {gorm.ErrRecordNotFound, codes.NotFound},
		"the database is unreachable": {errors.New("driver: bad connection"), codes.Internal},
	} {
		t.Run(name, func(t *testing.T) {
			streamsDao := mock_dao.NewMockStreamsDao(gomock.NewController(t))
			streamsDao.EXPECT().GetStreamByID(gomock.Any(), "5").Return(model.Stream{}, tc.err)
			m := New(dao.DaoWrapper{StreamsDao: streamsDao})
			if got := status.Code(m.saveSilences(context.Background(), req([]uint32{0}, []uint32{40}))); got != tc.want {
				t.Errorf("code = %v, want %v", got, tc.want)
			}
		})
	}
}
