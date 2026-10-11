package apiv2

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"testing"

	"go.uber.org/mock/gomock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"gorm.io/gorm"

	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/model"
)

func TestGetLectureCut(t *testing.T) {
	req := &protobuf.GetLectureCutRequest{CourseId: 1, StreamId: 7}

	t.Run("answers the signed recording and the cut the lecture has", func(t *testing.T) {
		noSigning(t)
		api, m := newLectureAPI(t)
		lecture := courseOneLecture()
		lecture.PlaylistUrlPRES = "https://pres"
		lecture.StartOffset, lecture.EndOffset = 5000, 65000
		lecture.Duration = sql.NullInt32{Int32: 120, Valid: true}
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(lecture, nil)
		m.courses.EXPECT().GetCourseById(gomock.Any(), uint(1)).
			Return(model.Course{Model: gorm.Model{ID: 1}, Name: "Brauereiwesen"}, nil)

		resp, err := api.GetLectureCut(asCaller(lectureAdmin), req)
		if err != nil {
			t.Fatalf("GetLectureCut: %v", err)
		}
		if resp.CourseName != "Brauereiwesen" || resp.LectureName != "Old" {
			t.Errorf("names = %q / %q", resp.CourseName, resp.LectureName)
		}
		// No combined version, so the presentation is the reference, signed.
		if resp.PlaylistUrl != "https://pres?jwt=signed" {
			t.Errorf("playlist = %q", resp.PlaylistUrl)
		}
		if resp.StartOffsetMs != 5000 || resp.EndOffsetMs != 65000 || resp.DurationSeconds != 120 {
			t.Errorf("cut = %d-%d, duration %d", resp.StartOffsetMs, resp.EndOffsetMs, resp.DurationSeconds)
		}
	})

	t.Run("answers an empty playlist for a lecture without a recording", func(t *testing.T) {
		noSigning(t)
		api, m := newLectureAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(courseOneLecture(), nil)
		m.courses.EXPECT().GetCourseById(gomock.Any(), uint(1)).Return(model.Course{Model: gorm.Model{ID: 1}}, nil)

		resp, err := api.GetLectureCut(asCaller(lectureAdmin), req)
		if err != nil {
			t.Fatalf("GetLectureCut: %v", err)
		}
		if resp.PlaylistUrl != "" || resp.DurationSeconds != 0 {
			t.Errorf("response = %v", resp)
		}
	})

	t.Run("does not reveal another course's lecture", func(t *testing.T) {
		api, m := newLectureAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(foreignLecture(), nil)

		_, err := api.GetLectureCut(asCaller(lectureAdmin), req)
		wantCode(t, err, codes.NotFound)
	})

	t.Run("404s a missing lecture", func(t *testing.T) {
		api, m := newLectureAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(model.Stream{}, gorm.ErrRecordNotFound)

		_, err := api.GetLectureCut(asCaller(lectureAdmin), req)
		wantCode(t, err, codes.NotFound)
	})
}

func TestSetLectureCut(t *testing.T) {
	set := func(start, end uint32) *protobuf.SetLectureCutRequest {
		return &protobuf.SetLectureCutRequest{CourseId: 1, StreamId: 7, StartOffsetMs: start, EndOffsetMs: end}
	}

	t.Run("saves the cut and audits it", func(t *testing.T) {
		api, m := newLectureAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(courseOneLecture(), nil)
		m.streams.EXPECT().SetCut(uint(7), uint(5000), uint(65000)).Return(nil)
		m.audits.EXPECT().Create(gomock.Any()).Return(nil)

		if _, err := api.SetLectureCut(asCaller(lectureAdmin), set(5000, 65000)); err != nil {
			t.Fatalf("SetLectureCut: %v", err)
		}
	})

	t.Run("removes the cut when both offsets are zero", func(t *testing.T) {
		api, m := newLectureAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(courseOneLecture(), nil)
		m.streams.EXPECT().SetCut(uint(7), uint(0), uint(0)).Return(nil)
		m.audits.EXPECT().Create(gomock.Any()).Return(nil)

		if _, err := api.SetLectureCut(asCaller(lectureAdmin), set(0, 0)); err != nil {
			t.Fatalf("SetLectureCut: %v", err)
		}
	})

	t.Run("refuses a cut that ends before it starts, writing nothing", func(t *testing.T) {
		for _, c := range []struct{ start, end uint32 }{{9000, 1000}, {4000, 4000}, {4000, 0}} {
			api, m := newLectureAPI(t)
			m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(courseOneLecture(), nil)

			_, err := api.SetLectureCut(asCaller(lectureAdmin), set(c.start, c.end))
			wantCode(t, err, codes.InvalidArgument)
		}
	})

	t.Run("will not cut another course's lecture, writing nothing", func(t *testing.T) {
		api, m := newLectureAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(foreignLecture(), nil)

		_, err := api.SetLectureCut(asCaller(lectureAdmin), set(1000, 2000))
		wantCode(t, err, codes.NotFound)
	})

	t.Run("reports a failed save and does not audit it", func(t *testing.T) {
		api, m := newLectureAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(courseOneLecture(), nil)
		m.streams.EXPECT().SetCut(uint(7), uint(1000), uint(2000)).Return(errors.New("db down"))

		_, err := api.SetLectureCut(asCaller(lectureAdmin), set(1000, 2000))
		// e.WithStatus maps a 500 to Unknown.
		wantCode(t, err, codes.Unknown)
	})
}

// The handlers check the lecture belongs to the course, not that the caller may
// touch the course: that is the policy, and a lecturer of another course or a
// student must not reach the handler at all.
func TestLectureCutIsCourseAdminOnly(t *testing.T) {
	course := model.Course{Model: gorm.Model{ID: 1}, UserID: 99}
	student := &model.User{Model: gorm.Model{ID: 3}, Role: model.StudentType}
	otherLecturer := &model.User{Model: gorm.Model{ID: 4}, Role: model.LecturerType}

	for _, method := range []string{"getLectureCut", "setLectureCut"} {
		for name, user := range map[string]*model.User{"a student": student, "a lecturer of other courses": otherLecturer} {
			t.Run(method+" refuses "+name, func(t *testing.T) {
				api, m := newLectureAPI(t)
				m.courses.EXPECT().GetCourseById(gomock.Any(), uint(1)).Return(course, nil).AnyTimes()
				api.log = slog.Default()

				called := false
				handler := func(context.Context, any) (any, error) { called = true; return nil, nil }
				info := &grpc.UnaryServerInfo{FullMethod: "/protobuf.CourseService/" + method}

				_, err := api.authorize(asCaller(user), &protobuf.GetLectureCutRequest{CourseId: 1, StreamId: 7}, info, handler)
				wantCode(t, err, codes.NotFound)
				if called {
					t.Error("the handler ran for a caller who does not administer the course")
				}
			})
		}
	}
}
