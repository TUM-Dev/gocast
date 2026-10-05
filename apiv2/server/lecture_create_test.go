package apiv2

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.uber.org/mock/gomock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"

	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/model"
)

var createCourse = model.Course{Model: gorm.Model{ID: 1}, Name: "Course", Slug: "crs", Year: 2026, TeachingTerm: "W"}

// expectCreated fills in IDs from 100 on, as the insert does, and hands the lectures
// to check.
func expectCreated(m lectureMocks, check func([]model.Stream)) {
	m.streams.EXPECT().CreateStreams(gomock.Any()).DoAndReturn(func(streams []model.Stream) error {
		for i := range streams {
			streams[i].ID = uint(100 + i)
		}
		check(streams)
		return nil
	})
}

func TestCreateLectures(t *testing.T) {
	livestream := func() *protobuf.CreateLecturesRequest {
		return &protobuf.CreateLecturesRequest{
			CourseId: 1, Title: "  Lecture 1 ", Start: timestamppb.New(lectureStart),
			DurationMinutes: 90, ChatEnabled: true,
		}
	}

	t.Run("creates one self-streamed lecture", func(t *testing.T) {
		api, m := newLectureAPI(t)
		m.courses.EXPECT().GetCourseById(gomock.Any(), uint(1)).Return(createCourse, nil)
		expectCreated(m, func(s []model.Stream) {
			if len(s) != 1 {
				t.Fatalf("created %d lectures", len(s))
			}
			l := s[0]
			if l.Name != "Lecture 1" || l.CourseID != 1 || !l.Start.Equal(lectureStart) || !l.End.Equal(lectureEnd) ||
				!l.ChatEnabled || l.LectureHallID != 0 || l.SeriesIdentifier != "" || len(l.StreamKey) != 32 ||
				l.Premiere || l.Recording || l.LiveNow || len(l.Files) != 0 {
				t.Errorf("created %+v", l)
			}
		})
		m.audits.EXPECT().Create(gomock.Any()).Return(nil)

		resp, err := api.CreateLectures(asCaller(lectureAdmin), livestream())
		if err != nil {
			t.Fatalf("CreateLectures: %v", err)
		}
		if len(resp.Lectures) != 1 || resp.Lectures[0].Id != 100 || resp.Lectures[0].StreamKey == "" ||
			!resp.Lectures[0].End.AsTime().Equal(lectureEnd) {
			t.Errorf("answered %v", resp.Lectures)
		}
	})

	t.Run("creates a series in a hall, one series identifier for all", func(t *testing.T) {
		api, m := newLectureAPI(t)
		req := livestream()
		req.LectureHallId = 3
		req.DateSeries = []*timestamppb.Timestamp{
			timestamppb.New(lectureStart.AddDate(0, 0, 7)), timestamppb.New(lectureStart.AddDate(0, 0, 14)),
		}
		m.courses.EXPECT().GetCourseById(gomock.Any(), uint(1)).Return(createCourse, nil)
		m.halls.EXPECT().GetLectureHallByID(uint(3)).Return(model.LectureHall{Model: gorm.Model{ID: 3}, Name: "HS1"}, nil)
		expectCreated(m, func(s []model.Stream) {
			if len(s) != 3 {
				t.Fatalf("created %d lectures", len(s))
			}
			for i, l := range s {
				want := lectureStart.AddDate(0, 0, 7*i)
				if !l.Start.Equal(want) || !l.End.Equal(want.Add(90*time.Minute)) || l.LectureHallID != 3 ||
					l.SeriesIdentifier == "" || l.SeriesIdentifier != s[0].SeriesIdentifier {
					t.Errorf("lecture %d: %+v", i, l)
				}
			}
			if s[0].StreamKey == s[1].StreamKey {
				t.Error("the lectures share a stream key")
			}
		})
		m.audits.EXPECT().Create(gomock.Any()).Return(nil).Times(3)

		resp, err := api.CreateLectures(asCaller(lectureAdmin), req)
		if err != nil {
			t.Fatalf("CreateLectures: %v", err)
		}
		if len(resp.Lectures) != 3 || resp.Lectures[2].Id != 102 || resp.Lectures[1].LectureHallName != "HS1" {
			t.Errorf("answered %v", resp.Lectures)
		}
	})

	t.Run("a VOD upload gets an hour, no key and no file", func(t *testing.T) {
		api, m := newLectureAPI(t)
		req := livestream()
		req.Kind = protobuf.LectureCreationKind_LECTURE_CREATION_KIND_VOD_UPLOAD
		req.DurationMinutes = 0
		m.courses.EXPECT().GetCourseById(gomock.Any(), uint(1)).Return(createCourse, nil)
		expectCreated(m, func(s []model.Stream) {
			l := s[0]
			if !l.End.Equal(lectureStart.Add(time.Hour)) || l.StreamKey != "" || l.Premiere || len(l.Files) != 0 {
				t.Errorf("created %+v", l)
			}
		})
		m.audits.EXPECT().Create(gomock.Any()).Return(nil)

		if _, err := api.CreateLectures(asCaller(lectureAdmin), req); err != nil {
			t.Fatalf("CreateLectures: %v", err)
		}
	})

	t.Run("each premiere of a series gets its own file", func(t *testing.T) {
		api, m := newLectureAPI(t)
		api.massStorage = t.TempDir()
		req := livestream()
		req.Kind = protobuf.LectureCreationKind_LECTURE_CREATION_KIND_PREMIERE
		req.DateSeries = []*timestamppb.Timestamp{timestamppb.New(lectureStart.AddDate(0, 0, 7))}
		dir := filepath.Join(api.massStorage, "2026", "W", "crs")
		m.courses.EXPECT().GetCourseById(gomock.Any(), uint(1)).Return(createCourse, nil)
		expectCreated(m, func(s []model.Stream) {
			want := []string{dir + "/crs_2026-10-12_08-15.mp4", dir + "/crs_2026-10-19_08-15.mp4"}
			for i, l := range s {
				if !l.Premiere || len(l.Files) != 1 || l.Files[0].Path != want[i] || l.StreamKey != "" {
					t.Errorf("lecture %d: %+v, want file %s", i, l, want[i])
				}
			}
		})
		m.audits.EXPECT().Create(gomock.Any()).Return(nil).Times(2)

		if _, err := api.CreateLectures(asCaller(lectureAdmin), req); err != nil {
			t.Fatalf("CreateLectures: %v", err)
		}
		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			t.Errorf("premiere directory: %v", err)
		}
	})

	t.Run("an ad-hoc lecture starts two minutes from now", func(t *testing.T) {
		api, m := newLectureAPI(t)
		req := livestream()
		req.Start, req.AdHoc = nil, true
		before := time.Now()
		m.courses.EXPECT().GetCourseById(gomock.Any(), uint(1)).Return(createCourse, nil)
		expectCreated(m, func(s []model.Stream) {
			l := s[0]
			if l.Start.Before(before.Add(adHocDelay)) || l.Start.After(time.Now().Add(adHocDelay)) ||
				l.End.Sub(l.Start) != 90*time.Minute {
				t.Errorf("created %+v", l)
			}
		})
		m.audits.EXPECT().Create(gomock.Any()).Return(nil)

		if _, err := api.CreateLectures(asCaller(lectureAdmin), req); err != nil {
			t.Fatalf("CreateLectures: %v", err)
		}
	})

	// Every refusal below happens before anything is written: the mocks have no
	// expectation that would let a write through.
	for name, mutate := range map[string]func(*protobuf.CreateLecturesRequest){
		"a VOD upload in a hall": func(r *protobuf.CreateLecturesRequest) {
			r.Kind = protobuf.LectureCreationKind_LECTURE_CREATION_KIND_VOD_UPLOAD
			r.LectureHallId = 3
		},
		"a premiere in a hall": func(r *protobuf.CreateLecturesRequest) {
			r.Kind = protobuf.LectureCreationKind_LECTURE_CREATION_KIND_PREMIERE
			r.LectureHallId = 3
		},
		"a blank title":                func(r *protobuf.CreateLecturesRequest) { r.Title = "  " },
		"a livestream with no length":  func(r *protobuf.CreateLecturesRequest) { r.DurationMinutes = 0 },
		"no start":                     func(r *protobuf.CreateLecturesRequest) { r.Start = nil },
		"an unknown kind":              func(r *protobuf.CreateLecturesRequest) { r.Kind = 7 },
		"start repeated in the series": func(r *protobuf.CreateLecturesRequest) { r.DateSeries = []*timestamppb.Timestamp{r.Start} },
		"an ad-hoc series": func(r *protobuf.CreateLecturesRequest) {
			r.AdHoc = true
			r.DateSeries = []*timestamppb.Timestamp{timestamppb.New(lectureStart.AddDate(0, 0, 7))}
		},
		"an ad-hoc VOD upload": func(r *protobuf.CreateLecturesRequest) {
			r.AdHoc = true
			r.Kind = protobuf.LectureCreationKind_LECTURE_CREATION_KIND_VOD_UPLOAD
		},
		"too long a series": func(r *protobuf.CreateLecturesRequest) {
			for i := 1; i < maxLecturesPerRequest+1; i++ {
				r.DateSeries = append(r.DateSeries, timestamppb.New(lectureStart.AddDate(0, 0, i)))
			}
		},
	} {
		t.Run("400s "+name, func(t *testing.T) {
			api, _ := newLectureAPI(t)
			req := livestream()
			mutate(req)
			_, err := api.CreateLectures(asCaller(lectureAdmin), req)
			wantCode(t, err, codes.InvalidArgument)
		})
	}

	t.Run("400s an unknown hall", func(t *testing.T) {
		api, m := newLectureAPI(t)
		req := livestream()
		req.LectureHallId = 99
		m.courses.EXPECT().GetCourseById(gomock.Any(), uint(1)).Return(createCourse, nil)
		m.halls.EXPECT().GetLectureHallByID(uint(99)).Return(model.LectureHall{}, gorm.ErrRecordNotFound)

		_, err := api.CreateLectures(asCaller(serverAdmin), req)
		wantCode(t, err, codes.InvalidArgument)
	})

	t.Run("404s course 0", func(t *testing.T) {
		api, _ := newLectureAPI(t)
		req := livestream()
		req.CourseId = 0
		_, err := api.CreateLectures(asCaller(serverAdmin), req)
		wantCode(t, err, codes.NotFound)
	})
}

// The policy, not the handler, keeps a caller out of someone else's course; this
// goes through it as a request does.
func TestCreateLecturesPolicy(t *testing.T) {
	someoneElse := &model.User{Model: gorm.Model{ID: 5}, Role: model.LecturerType,
		AdministeredCourses: []model.Course{{Model: gorm.Model{ID: 2}}}}

	for name, tt := range map[string]struct {
		user     *model.User
		courseID uint32
		want     codes.Code
	}{
		"a lecturer of another course":       {someoneElse, 1, codes.NotFound},
		"course 0, even for a server admin":  {serverAdmin, 0, codes.NotFound},
		"the course's administrator gets in": {lectureAdmin, 1, codes.OK},
	} {
		t.Run(name, func(t *testing.T) {
			api, m := newLectureAPI(t)
			m.courses.EXPECT().GetCourseById(gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, id uint) (model.Course, error) {
					if id == 1 {
						return createCourse, nil
					}
					return model.Course{}, nil // gorm's Find: a missing row is a zero value
				})

			called := false
			handler := func(context.Context, any) (any, error) {
				called = true
				return &protobuf.CreateLecturesResponse{}, nil
			}
			_, err := api.authorize(asCaller(tt.user),
				&protobuf.CreateLecturesRequest{CourseId: tt.courseID, Title: "x"},
				&grpc.UnaryServerInfo{FullMethod: protobuf.CourseService_CreateLectures_FullMethodName}, handler)
			wantCode(t, err, tt.want)
			if called != (tt.want == codes.OK) {
				t.Errorf("handler called = %v", called)
			}
		})
	}
}
