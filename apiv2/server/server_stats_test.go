package apiv2

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"

	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
)

// expectServerStatsDao sets up a MockStatisticsDao that answers every call
// GetServerStats/ExportServerStats can make, all scoped to courseID 0 — the "every
// course" convention the per-course statistics page in api/ already relies on.
func expectServerStatsDao(t *testing.T) *mock_dao.MockStatisticsDao {
	t.Helper()
	ctrl := gomock.NewController(t)
	m := mock_dao.NewMockStatisticsDao(ctrl)

	m.EXPECT().GetCourseNumStudents(uint(0)).Return(int64(42), nil).AnyTimes()
	m.EXPECT().GetCourseNumVodViews(uint(0)).Return(7, nil).AnyTimes()
	m.EXPECT().GetCourseNumLiveViews(uint(0)).Return(3, nil).AnyTimes()
	m.EXPECT().GetStudentActivityCourseStats(uint(0), true).
		Return([]dao.Stat{{X: "2024 01", Y: 5}}, nil).AnyTimes()
	m.EXPECT().GetStudentActivityCourseStats(uint(0), false).
		Return([]dao.Stat{{X: "2024 01", Y: 9}}, nil).AnyTimes()
	m.EXPECT().GetCourseStatsHourly(uint(0)).
		Return([]dao.Stat{{X: "14", Y: 2}}, nil).AnyTimes()
	m.EXPECT().GetCourseStatsWeekdays(uint(0)).
		Return([]dao.Stat{{X: "Monday", Y: 4}}, nil).AnyTimes()
	m.EXPECT().GetCourseNumVodViewsPerDay(uint(0)).
		Return([]dao.Stat{{X: "18.04.2022", Y: 6}}, nil).AnyTimes()

	return m
}

// expectServerStatsStreams sets up a MockStreamsDao answering GetAllStreams with a
// mix a naive count would get wrong: one recorded, one still only scheduled, one live.
func expectServerStatsStreams(t *testing.T) *mock_dao.MockStreamsDao {
	t.Helper()
	ctrl := gomock.NewController(t)
	m := mock_dao.NewMockStreamsDao(ctrl)
	m.EXPECT().GetAllStreams().Return([]model.Stream{
		{Recording: true},
		{Recording: false, LiveNow: false},
		{LiveNow: true},
	}, nil).AnyTimes()
	return m
}

func TestGetServerStats(t *testing.T) {
	m := expectServerStatsDao(t)
	streams := expectServerStatsStreams(t)
	api := &API{dao: dao.DaoWrapper{StatisticsDao: m, StreamsDao: streams}, log: slog.Default()}

	resp, err := api.GetServerStats(context.Background(), &emptypb.Empty{})
	if err != nil {
		t.Fatalf("GetServerStats: %v", err)
	}

	if resp.NumStudents != 42 {
		t.Errorf("NumStudents = %d, want 42", resp.NumStudents)
	}
	if resp.VodViews != 7 {
		t.Errorf("VodViews = %d, want 7", resp.VodViews)
	}
	if resp.LiveViews != 3 {
		t.Errorf("LiveViews = %d, want 3", resp.LiveViews)
	}
	if resp.NumLectures != 2 {
		t.Errorf("NumLectures = %d, want 2 (recorded or live, not merely scheduled)", resp.NumLectures)
	}
	if len(resp.ActivityLive.GetPoints()) != 1 || resp.ActivityLive.GetPoints()[0].GetY() != 5 {
		t.Errorf("ActivityLive = %+v, want one point with y=5", resp.ActivityLive)
	}
	if len(resp.ActivityVod.GetPoints()) != 1 || resp.ActivityVod.GetPoints()[0].GetY() != 9 {
		t.Errorf("ActivityVod = %+v, want one point with y=9", resp.ActivityVod)
	}
	if len(resp.Hourly.GetPoints()) != 1 || resp.Hourly.GetPoints()[0].GetX() != "14" {
		t.Errorf("Hourly = %+v, want one point with x=14", resp.Hourly)
	}
	if len(resp.Weekdays.GetPoints()) != 1 || resp.Weekdays.GetPoints()[0].GetX() != "Monday" {
		t.Errorf("Weekdays = %+v, want one point with x=Monday", resp.Weekdays)
	}
	if len(resp.AllDays.GetPoints()) != 1 || resp.AllDays.GetPoints()[0].GetY() != 6 {
		t.Errorf("AllDays = %+v, want one point with y=6", resp.AllDays)
	}
}

func TestGetServerStatsPropagatesADaoFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := mock_dao.NewMockStatisticsDao(ctrl)
	m.EXPECT().GetCourseNumStudents(uint(0)).Return(int64(0), errors.New("database is on fire"))

	api := &API{dao: dao.DaoWrapper{StatisticsDao: m}, log: slog.Default()}

	_, err := api.GetServerStats(context.Background(), &emptypb.Empty{})
	if err == nil {
		t.Fatal("a failed query was reported as success")
	}
	if got := status.Code(err); got != codes.Unknown {
		t.Errorf("code = %v, want %v", got, codes.Unknown)
	}
}

func TestExportServerStats(t *testing.T) {
	t.Run("refuses a format it does not know", func(t *testing.T) {
		api := &API{log: slog.Default()}

		_, err := api.ExportServerStats(context.Background(), &protobuf.ExportServerStatsRequest{Format: "xml"})
		if got := status.Code(err); got != codes.InvalidArgument {
			t.Errorf("code = %v, want %v", got, codes.InvalidArgument)
		}
	})

	t.Run("exports json with the quick stats and every series", func(t *testing.T) {
		m := expectServerStatsDao(t)
		api := &API{dao: dao.DaoWrapper{StatisticsDao: m}, log: slog.Default()}

		body, err := api.ExportServerStats(context.Background(), &protobuf.ExportServerStatsRequest{Format: "json"})
		if err != nil {
			t.Fatalf("ExportServerStats: %v", err)
		}
		if body.GetContentType() != "application/octet-stream" {
			t.Errorf("ContentType = %q, want application/octet-stream", body.GetContentType())
		}

		var entries []map[string]any
		if err := json.Unmarshal(body.GetData(), &entries); err != nil {
			t.Fatalf("exported body is not valid JSON: %v", err)
		}

		names := map[string]bool{}
		for _, entry := range entries {
			names[entry["name"].(string)] = true
		}
		for _, want := range []string{"day", "hour", "activity-live", "activity-vod", "allDays", "quickStats"} {
			if !names[want] {
				t.Errorf("exported JSON is missing the %q entry: %v", want, names)
			}
		}
	})

	t.Run("exports csv", func(t *testing.T) {
		m := expectServerStatsDao(t)
		api := &API{dao: dao.DaoWrapper{StatisticsDao: m}, log: slog.Default()}

		body, err := api.ExportServerStats(context.Background(), &protobuf.ExportServerStatsRequest{Format: "csv"})
		if err != nil {
			t.Fatalf("ExportServerStats: %v", err)
		}
		if len(body.GetData()) == 0 {
			t.Fatal("exported csv is empty")
		}
	})
}
