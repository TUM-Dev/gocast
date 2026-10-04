package apiv2

import (
	"context"
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

func TestGetLectureStats(t *testing.T) {
	start := time.Date(2024, 4, 18, 10, 15, 0, 0, time.UTC)
	// Stream 7 is course 5's.
	lecture := model.Stream{
		Model: gorm.Model{ID: 7}, CourseID: 5, Name: "Hopfen", Start: start, End: start.Add(90 * time.Minute),
	}

	// setup answers the stream lookup with the given stream. The statistics mock
	// expects nothing unless the subtest says so, which is how the refusals below
	// prove no numbers were read.
	setup := func(t *testing.T, stream model.Stream, streamErr error) (*API, *mock_dao.MockStatisticsDao) {
		ctrl := gomock.NewController(t)
		streams := mock_dao.NewMockStreamsDao(ctrl)
		streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(stream, streamErr).Times(1)
		courses := mock_dao.NewMockCoursesDao(ctrl)
		courses.EXPECT().GetCourseById(gomock.Any(), uint(5)).
			Return(model.Course{Model: gorm.Model{ID: 5}, Name: "Brauereiwesen"}, nil).AnyTimes()
		stats := mock_dao.NewMockStatisticsDao(ctrl)

		return &API{
			dao: dao.DaoWrapper{StreamsDao: streams, CoursesDao: courses, StatisticsDao: stats},
			log: slog.Default(),
		}, stats
	}

	req := &protobuf.GetLectureStatsRequest{CourseId: 5, StreamId: 7}

	t.Run("answers with the lecture's numbers and the course's enrolment", func(t *testing.T) {
		api, stats := setup(t, lecture, nil)
		stats.EXPECT().GetCourseNumStudents(uint(5)).Return(int64(12), nil)
		stats.EXPECT().GetLectureNumVodViews(uint(7)).Return(30, nil)
		stats.EXPECT().GetLectureNumLiveViews(uint(7)).Return(8, nil)
		stats.EXPECT().GetLectureStats(uint(5), uint(7)).Return([]dao.Stat{{X: "10:20", Y: 8}}, nil)
		stats.EXPECT().GetLectureStatsWeekdays(uint(5), uint(7)).Return([]dao.Stat{{X: "Monday", Y: 4}}, nil)
		stats.EXPECT().GetLectureNumVodViewsPerDay(uint(7)).Return([]dao.Stat{{X: "18.04.2024", Y: 6}}, nil)

		resp, err := api.GetLectureStats(context.Background(), req)
		if err != nil {
			t.Fatalf("GetLectureStats: %v", err)
		}

		if resp.CourseName != "Brauereiwesen" || resp.LectureName != "Hopfen" {
			t.Errorf("names = %q / %q", resp.CourseName, resp.LectureName)
		}
		if !resp.Start.AsTime().Equal(start) || !resp.End.AsTime().Equal(start.Add(90*time.Minute)) {
			t.Errorf("times = %v - %v", resp.Start.AsTime(), resp.End.AsTime())
		}
		if resp.NumStudents != 12 || resp.VodViews != 30 || resp.MaxLiveViews != 8 {
			t.Errorf("counters = %d/%d/%d, want 12/30/8", resp.NumStudents, resp.VodViews, resp.MaxLiveViews)
		}
		if got := resp.LiveViewers.GetPoints(); len(got) != 1 || got[0].X != "10:20" {
			t.Errorf("LiveViewers = %v", got)
		}
		if resp.PartialHistory {
			t.Error("a 2024 lecture was flagged as predating the viewing data")
		}
	})

	t.Run("flags a lecture old enough to predate the viewing data", func(t *testing.T) {
		old := lecture
		old.Start = time.Date(2021, 5, 3, 10, 0, 0, 0, time.UTC)
		api, stats := setup(t, old, nil)
		stats.EXPECT().GetCourseNumStudents(gomock.Any()).Return(int64(0), nil)
		stats.EXPECT().GetLectureNumVodViews(gomock.Any()).Return(0, nil)
		stats.EXPECT().GetLectureNumLiveViews(gomock.Any()).Return(0, nil)
		stats.EXPECT().GetLectureStats(gomock.Any(), gomock.Any()).Return(nil, nil)
		stats.EXPECT().GetLectureStatsWeekdays(gomock.Any(), gomock.Any()).Return(nil, nil)
		stats.EXPECT().GetLectureNumVodViewsPerDay(gomock.Any()).Return(nil, nil)

		resp, err := api.GetLectureStats(context.Background(), req)
		if err != nil {
			t.Fatalf("GetLectureStats: %v", err)
		}
		if !resp.PartialHistory {
			t.Error("a 2021 lecture was not flagged")
		}
	})

	t.Run("does not read another course's lecture", func(t *testing.T) {
		other := lecture
		other.CourseID = 6
		api, _ := setup(t, other, nil)

		_, err := api.GetLectureStats(context.Background(), req)
		if status.Code(err) != codes.NotFound {
			t.Fatalf("code = %v, want NotFound", status.Code(err))
		}
	})

	t.Run("404s a missing lecture", func(t *testing.T) {
		api, _ := setup(t, model.Stream{}, gorm.ErrRecordNotFound)

		_, err := api.GetLectureStats(context.Background(), req)
		if status.Code(err) != codes.NotFound {
			t.Fatalf("code = %v, want NotFound", status.Code(err))
		}
	})

	t.Run("never answers for course 0", func(t *testing.T) {
		orphan := lecture
		orphan.CourseID = 0
		api, _ := setup(t, orphan, nil)

		_, err := api.GetLectureStats(context.Background(), &protobuf.GetLectureStatsRequest{CourseId: 0, StreamId: 7})
		if status.Code(err) != codes.NotFound {
			t.Fatalf("code = %v, want NotFound", status.Code(err))
		}
	})
}
