package apiv2

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"

	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
)

// expectCourseStatsDao answers the statistics queries for course 5 only. A query for
// any other ID -- 0, the server-wide one, above all -- is an unexpected call and fails
// the test.
func expectCourseStatsDao(t *testing.T) *mock_dao.MockStatisticsDao {
	t.Helper()
	m := mock_dao.NewMockStatisticsDao(gomock.NewController(t))

	m.EXPECT().GetCourseNumStudents(uint(5)).Return(int64(12), nil).AnyTimes()
	m.EXPECT().GetCourseNumVodViews(uint(5)).Return(30, nil).AnyTimes()
	m.EXPECT().GetCourseNumLiveViews(uint(5)).Return(8, nil).AnyTimes()
	m.EXPECT().GetStudentActivityCourseStats(uint(5), true).
		Return([]dao.Stat{{X: "2024 01", Y: 5}}, nil).AnyTimes()
	m.EXPECT().GetStudentActivityCourseStats(uint(5), false).
		Return([]dao.Stat{{X: "2024 01", Y: 9}}, nil).AnyTimes()
	m.EXPECT().GetCourseStatsHourly(uint(5)).Return([]dao.Stat{{X: "14", Y: 2}}, nil).AnyTimes()
	m.EXPECT().GetCourseStatsWeekdays(uint(5)).Return([]dao.Stat{{X: "Monday", Y: 4}}, nil).AnyTimes()
	m.EXPECT().GetCourseNumVodViewsPerDay(uint(5)).
		Return([]dao.Stat{{X: "18.04.2022", Y: 6}}, nil).AnyTimes()

	return m
}

func courseStatsAPI(t *testing.T, course model.Course, courseErr error) *API {
	t.Helper()
	courses := mock_dao.NewMockCoursesDao(gomock.NewController(t))
	courses.EXPECT().GetCourseById(gomock.Any(), gomock.Any()).Return(course, courseErr).AnyTimes()

	return &API{
		dao: dao.DaoWrapper{CoursesDao: courses, StatisticsDao: expectCourseStatsDao(t)},
		log: slog.Default(),
	}
}

func TestGetCourseStats(t *testing.T) {
	course := model.Course{
		Model: gorm.Model{ID: 5},
		Name:  "Einführung Brauereiwesen",
		Year:  2024,
		Streams: []model.Stream{
			{Recording: true},
			{Recording: false, LiveNow: false},
			{LiveNow: true},
		},
	}

	t.Run("answers with the course's own numbers", func(t *testing.T) {
		api := courseStatsAPI(t, course, nil)

		resp, err := api.GetCourseStats(context.Background(), &protobuf.GetCourseStatsRequest{CourseId: 5})
		if err != nil {
			t.Fatalf("GetCourseStats: %v", err)
		}

		if resp.CourseName != course.Name {
			t.Errorf("CourseName = %q, want %q", resp.CourseName, course.Name)
		}
		if resp.NumStudents != 12 || resp.VodViews != 30 || resp.LiveViews != 8 {
			t.Errorf("counters = %d/%d/%d, want 12/30/8", resp.NumStudents, resp.VodViews, resp.LiveViews)
		}
		if resp.NumLectures != 2 {
			t.Errorf("NumLectures = %d, want 2 (recorded or live, not merely scheduled)", resp.NumLectures)
		}
		if got := resp.AllDays.GetPoints(); len(got) != 1 || got[0].X != "18.04.2022" || got[0].Y != 6 {
			t.Errorf("AllDays = %v", got)
		}
		if resp.PartialHistory {
			t.Error("a 2024 course was flagged as predating the viewing data")
		}
	})

	t.Run("flags a course old enough to predate the viewing data", func(t *testing.T) {
		old := course
		old.Year = 2022
		api := courseStatsAPI(t, old, nil)

		resp, err := api.GetCourseStats(context.Background(), &protobuf.GetCourseStatsRequest{CourseId: 5})
		if err != nil {
			t.Fatalf("GetCourseStats: %v", err)
		}
		if !resp.PartialHistory {
			t.Error("a 2022 course was not flagged")
		}
	})

	t.Run("never falls through to the server-wide numbers", func(t *testing.T) {
		// What GetCourseById's Find answers for a row that is not there.
		api := courseStatsAPI(t, model.Course{}, nil)

		_, err := api.GetCourseStats(context.Background(), &protobuf.GetCourseStatsRequest{CourseId: 0})
		if status.Code(err) != codes.NotFound {
			t.Fatalf("code = %v, want NotFound", status.Code(err))
		}
	})
}

func TestExportCourseStats(t *testing.T) {
	api := courseStatsAPI(t, model.Course{Model: gorm.Model{ID: 5}}, nil)

	t.Run("exports the course's series under v1's entry names", func(t *testing.T) {
		body, err := api.ExportCourseStats(context.Background(), &protobuf.ExportCourseStatsRequest{CourseId: 5, Format: "json"})
		if err != nil {
			t.Fatalf("ExportCourseStats: %v", err)
		}

		var entries []struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(body.Data, &entries); err != nil {
			t.Fatalf("export is not the JSON list v1 produced: %v", err)
		}
		names := map[string]bool{}
		for _, entry := range entries {
			names[entry.Name] = true
		}
		for _, want := range []string{"day", "hour", "activity-live", "activity-vod", "allDays", "quickStats"} {
			if !names[want] {
				t.Errorf("export has no %q entry", want)
			}
		}
	})

	t.Run("rejects an unknown format", func(t *testing.T) {
		_, err := api.ExportCourseStats(context.Background(), &protobuf.ExportCourseStatsRequest{CourseId: 5, Format: "xml"})
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("code = %v, want InvalidArgument", status.Code(err))
		}
	})

	t.Run("never exports the server-wide numbers", func(t *testing.T) {
		_, err := api.ExportCourseStats(context.Background(), &protobuf.ExportCourseStatsRequest{CourseId: 0, Format: "csv"})
		if status.Code(err) != codes.NotFound {
			t.Fatalf("code = %v, want NotFound", status.Code(err))
		}
	})
}
