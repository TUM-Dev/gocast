package runner_manager

import (
	"context"
	"database/sql"
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

func TestSaveDuration(t *testing.T) {
	const (
		comb = protobuf.StreamVersion_STREAM_VERSION_COMBINED
		cam  = protobuf.StreamVersion_STREAM_VERSION_CAMERA
	)
	req := func(v protobuf.StreamVersion, seconds uint32) *protobuf.DurationProbedNotification {
		return &protobuf.DurationProbedNotification{
			Stream:          &protobuf.StreamInfo{Id: ptr.Take(uint64(5))},
			StreamVersion:   ptr.Take(v),
			DurationSeconds: ptr.Take(seconds),
		}
	}
	withDuration := func(seconds int32) sql.NullInt32 { return sql.NullInt32{Int32: seconds, Valid: true} }

	for name, tc := range map[string]struct {
		stream model.Stream
		req    *protobuf.DurationProbedNotification
		saves  bool
	}{
		"first duration":                  {model.Stream{}, req(comb, 3600), true},
		"combined replaces a stale value": {model.Stream{PlaylistUrl: "c", Duration: withDuration(10)}, req(comb, 3600), true},
		"camera sets a missing duration":  {model.Stream{PlaylistUrl: "c"}, req(cam, 3600), true},
		"camera without combined VOD":     {model.Stream{PlaylistUrlCAM: "x", Duration: withDuration(10)}, req(cam, 3600), true},
		"camera does not replace combined": {model.Stream{PlaylistUrl: "c", PlaylistUrlCAM: "x", Duration: withDuration(3601)},
			req(cam, 3600), false},
	} {
		t.Run(name, func(t *testing.T) {
			tc.stream.ID = 5
			streamsDao := mock_dao.NewMockStreamsDao(gomock.NewController(t))
			streamsDao.EXPECT().GetStreamByID(gomock.Any(), "5").Return(tc.stream, nil)
			if tc.saves {
				streamsDao.EXPECT().SetDuration(uint(5), uint32(3600)).Return(nil)
			}
			m := New(dao.DaoWrapper{StreamsDao: streamsDao})
			if err := m.saveDuration(context.Background(), tc.req); err != nil {
				t.Fatal(err)
			}
		})
	}

	t.Run("no duration is rejected for good", func(t *testing.T) {
		m := New(dao.DaoWrapper{StreamsDao: mock_dao.NewMockStreamsDao(gomock.NewController(t))})
		if got := status.Code(m.saveDuration(context.Background(), req(comb, 0))); got != codes.InvalidArgument {
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
			if got := status.Code(m.saveDuration(context.Background(), req(comb, 60))); got != tc.want {
				t.Errorf("code = %v, want %v", got, tc.want)
			}
		})
	}
}
