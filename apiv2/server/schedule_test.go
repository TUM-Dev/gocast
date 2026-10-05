package apiv2

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"

	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
)

func TestGetSchedule(t *testing.T) {
	from := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	to := from.Add(7 * 24 * time.Hour)
	week := func() *protobuf.GetScheduleRequest {
		return &protobuf.GetScheduleRequest{
			From: timestamppb.New(from), To: timestamppb.New(to), LectureHallIds: []uint32{0, 3},
		}
	}

	lecturer := &model.User{Model: gorm.Model{ID: 2}, Role: model.LecturerType}
	admin := &model.User{Model: gorm.Model{ID: 1}, Role: model.AdminType}

	t.Run("asks for the lecturer's own courses in the window and halls given", func(t *testing.T) {
		halls := mock_dao.NewMockLectureHallsDao(gomock.NewController(t))
		halls.EXPECT().GetSchedule(uint(2), from, to, []uint{0, 3}, false).Return([]dao.ScheduleEntry{{
			StreamID: 7, CourseID: 1, CourseName: "Brauereiwesen", Name: "Hopfen",
			Start: from.Add(time.Hour), End: from.Add(2 * time.Hour), LectureHallID: 3, LectureHallName: "HS1",
		}}, nil)
		api := &API{dao: dao.DaoWrapper{LectureHallsDao: halls}, log: slog.Default()}

		resp, err := api.GetSchedule(asCaller(lecturer), week())
		if err != nil {
			t.Fatalf("GetSchedule: %v", err)
		}
		if len(resp.Lectures) != 1 {
			t.Fatalf("lectures = %v", resp.Lectures)
		}
		got := resp.Lectures[0]
		if got.StreamId != 7 || got.CourseId != 1 || got.Name != "Hopfen" || got.LectureHallName != "HS1" {
			t.Errorf("lecture = %v", got)
		}
		if !got.Start.AsTime().Equal(from.Add(time.Hour)) {
			t.Errorf("start = %v", got.Start.AsTime())
		}
	})

	t.Run("asks for every course for someone who may view them all", func(t *testing.T) {
		halls := mock_dao.NewMockLectureHallsDao(gomock.NewController(t))
		halls.EXPECT().GetSchedule(uint(0), from, to, gomock.Any(), true).Return(nil, nil)
		api := &API{dao: dao.DaoWrapper{LectureHallsDao: halls}, log: slog.Default()}

		req := week()
		req.AllLectureHalls = true
		if _, err := api.GetSchedule(asCaller(admin), req); err != nil {
			t.Fatalf("GetSchedule: %v", err)
		}
	})

	for name, mutate := range map[string]func(*protobuf.GetScheduleRequest){
		"no window":          func(r *protobuf.GetScheduleRequest) { r.From, r.To = nil, nil },
		"a backwards window": func(r *protobuf.GetScheduleRequest) { r.From, r.To = r.To, r.From },
		"a window of over 100 days": func(r *protobuf.GetScheduleRequest) {
			r.To = timestamppb.New(from.Add(101 * 24 * time.Hour))
		},
	} {
		t.Run("rejects "+name, func(t *testing.T) {
			// No expectations: the query must not run.
			halls := mock_dao.NewMockLectureHallsDao(gomock.NewController(t))
			api := &API{dao: dao.DaoWrapper{LectureHallsDao: halls}, log: slog.Default()}
			req := week()
			mutate(req)

			_, err := api.GetSchedule(asCaller(lecturer), req)
			if status.Code(err) != codes.InvalidArgument {
				t.Fatalf("code = %v, want InvalidArgument", status.Code(err))
			}
		})
	}
}

func TestListScheduleLectureHalls(t *testing.T) {
	halls := mock_dao.NewMockLectureHallsDao(gomock.NewController(t))
	halls.EXPECT().GetAllLectureHalls().Return([]model.LectureHall{
		{Model: gorm.Model{ID: 2}, Name: "MW_HS2", CamIP: "rtsp://10.0.0.1/cam"},
		{Model: gorm.Model{ID: 1}, Name: "HS001"},
	})
	api := &API{dao: dao.DaoWrapper{LectureHallsDao: halls}, log: slog.Default()}

	resp, err := api.ListScheduleLectureHalls(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListScheduleLectureHalls: %v", err)
	}
	if len(resp.LectureHalls) != 2 || resp.LectureHalls[0].Name != "HS001" || resp.LectureHalls[1].Id != 2 {
		t.Errorf("halls = %v, want both by name", resp.LectureHalls)
	}
}

func TestUpdateLecture(t *testing.T) {
	stream := model.Stream{Model: gorm.Model{ID: 7}, CourseID: 1, Name: "Old", Description: "Old text"}

	setup := func(t *testing.T, found model.Stream) (*API, *mock_dao.MockStreamsDao) {
		streams := mock_dao.NewMockStreamsDao(gomock.NewController(t))
		streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(found, nil)
		return &API{dao: dao.DaoWrapper{StreamsDao: streams}, log: slog.Default()}, streams
	}
	ptr := func(s string) *string { return &s }

	t.Run("renames and leaves the description", func(t *testing.T) {
		api, streams := setup(t, stream)
		streams.EXPECT().UpdateStream(gomock.Any()).DoAndReturn(func(s model.Stream) error {
			if s.Name != "Hopfen" || s.Description != "Old text" {
				t.Errorf("saved %q / %q", s.Name, s.Description)
			}
			return nil
		})

		_, err := api.UpdateLecture(context.Background(), &protobuf.UpdateLectureRequest{
			CourseId: 1, StreamId: 7, Name: ptr("  Hopfen "),
		})
		if err != nil {
			t.Fatalf("UpdateLecture: %v", err)
		}
	})

	t.Run("clears the description when sent empty", func(t *testing.T) {
		api, streams := setup(t, stream)
		streams.EXPECT().UpdateStream(gomock.Any()).DoAndReturn(func(s model.Stream) error {
			if s.Name != "Old" || s.Description != "" {
				t.Errorf("saved %q / %q", s.Name, s.Description)
			}
			return nil
		})

		_, err := api.UpdateLecture(context.Background(), &protobuf.UpdateLectureRequest{
			CourseId: 1, StreamId: 7, Description: ptr(""),
		})
		if err != nil {
			t.Fatalf("UpdateLecture: %v", err)
		}
	})

	t.Run("does not touch another course's lecture", func(t *testing.T) {
		other := stream
		other.CourseID = 2
		api, _ := setup(t, other)

		_, err := api.UpdateLecture(context.Background(), &protobuf.UpdateLectureRequest{
			CourseId: 1, StreamId: 7, Name: ptr("Hijacked"),
		})
		if status.Code(err) != codes.NotFound {
			t.Fatalf("code = %v, want NotFound", status.Code(err))
		}
	})
}
