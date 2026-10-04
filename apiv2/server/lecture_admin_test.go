package apiv2

import (
	"context"
	"errors"
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

// lectureMocks are the DAOs the lecture administration handlers reach. Every
// expectation is explicit, so a refusal that wrote anything fails its test.
type lectureMocks struct {
	streams *mock_dao.MockStreamsDao
	courses *mock_dao.MockCoursesDao
	halls   *mock_dao.MockLectureHallsDao
	audits  *mock_dao.MockAuditDao
}

func newLectureAPI(t *testing.T) (*API, lectureMocks) {
	ctrl := gomock.NewController(t)
	m := lectureMocks{
		streams: mock_dao.NewMockStreamsDao(ctrl),
		courses: mock_dao.NewMockCoursesDao(ctrl),
		halls:   mock_dao.NewMockLectureHallsDao(ctrl),
		audits:  mock_dao.NewMockAuditDao(ctrl),
	}
	return &API{
		dao: dao.DaoWrapper{
			StreamsDao: m.streams, CoursesDao: m.courses, LectureHallsDao: m.halls, AuditDao: m.audits,
		},
		log: slog.Default(),
	}, m
}

var (
	lectureAdmin = &model.User{Model: gorm.Model{ID: 2}, Role: model.LecturerType,
		AdministeredCourses: []model.Course{{Model: gorm.Model{ID: 1}}}}
	lectureStart = time.Date(2026, 10, 12, 8, 15, 0, 0, time.UTC)
	lectureEnd   = lectureStart.Add(90 * time.Minute)
)

// courseOneLecture is stream 7 of course 1, in series "s1".
func courseOneLecture() model.Stream {
	return model.Stream{
		Model: gorm.Model{ID: 7}, CourseID: 1, Name: "Old", Description: "Old text",
		Start: lectureStart, End: lectureEnd, SeriesIdentifier: "s1", StreamKey: "secret",
	}
}

func foreignLecture() model.Stream {
	s := courseOneLecture()
	s.CourseID = 2
	return s
}

func wantCode(t *testing.T, err error, want codes.Code) {
	t.Helper()
	if status.Code(err) != want {
		t.Fatalf("code = %v (%v), want %v", status.Code(err), err, want)
	}
}

func TestListCourseLecturesAdmin(t *testing.T) {
	api, m := newLectureAPI(t)
	recorded := courseOneLecture()
	recorded.Recording = true
	recorded.LectureHallID = 3
	recorded.PlaylistUrl, recorded.PlaylistUrlCAM = "https://comb", "https://cam"
	recorded.Files = []model.File{{Model: gorm.Model{ID: 5}, Type: model.FILETYPE_ATTACHMENT, Path: "/x/slides.pdf"}}
	recorded.TranscodingProgresses = []model.TranscodingProgress{{StreamID: 7, Version: model.PRES, Progress: 40}}
	recorded.VideoSections = []model.VideoSection{{Model: gorm.Model{ID: 9}, Description: "Intro", StartMinutes: 3}}
	selfStreamed := model.Stream{Model: gorm.Model{ID: 8}, CourseID: 1, Name: "Self"}

	m.courses.EXPECT().GetCourseById(gomock.Any(), uint(1)).
		Return(model.Course{Model: gorm.Model{ID: 1}, Streams: []model.Stream{recorded, selfStreamed}}, nil)
	m.halls.EXPECT().GetAllLectureHalls().Return([]model.LectureHall{{Model: gorm.Model{ID: 3}, Name: "HS1"}})

	resp, err := api.ListCourseLecturesAdmin(context.Background(), &protobuf.ListCourseLecturesAdminRequest{CourseId: 1})
	if err != nil {
		t.Fatalf("ListCourseLecturesAdmin: %v", err)
	}
	if len(resp.Lectures) != 2 {
		t.Fatalf("lectures = %v", resp.Lectures)
	}
	got := resp.Lectures[0]
	if got.Id != 7 || got.Name != "Old" || got.LectureHallName != "HS1" || got.StreamKey != "secret" ||
		got.SeriesIdentifier != "s1" || !got.Recording || !got.Start.AsTime().Equal(lectureStart) {
		t.Errorf("lecture = %v", got)
	}
	if len(got.VodVersions) != 2 || got.VodVersions[0] != "COMB" || got.VodVersions[1] != "CAM" {
		t.Errorf("vod versions = %v, want [COMB CAM]", got.VodVersions)
	}
	if len(got.Files) != 1 || got.Files[0].FriendlyName != recorded.Files[0].GetFriendlyFileName() || got.Files[0].Type != uint32(model.FILETYPE_ATTACHMENT) {
		t.Errorf("files = %v", got.Files)
	}
	if len(got.TranscodingProgresses) != 1 || got.TranscodingProgresses[0].Progress != 40 {
		t.Errorf("progresses = %v", got.TranscodingProgresses)
	}
	if len(got.VideoSections) != 1 || got.VideoSections[0].StartMinutes != 3 {
		t.Errorf("sections = %v", got.VideoSections)
	}
	if self := resp.Lectures[1]; self.LectureHallId != 0 || self.LectureHallName != "" {
		t.Errorf("self-streamed lecture = %v, want no hall", self)
	}
}

func TestUpdateLectureAdminFields(t *testing.T) {
	ptrU := func(v uint32) *uint32 { return &v }
	ptrB := func(v bool) *bool { return &v }
	newStart := lectureStart.Add(24 * time.Hour)

	t.Run("moves the lecture, its hall, chat and visibility", func(t *testing.T) {
		api, m := newLectureAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(courseOneLecture(), nil)
		m.halls.EXPECT().GetLectureHallByID(uint(4)).Return(model.LectureHall{Model: gorm.Model{ID: 4}}, nil)
		m.streams.EXPECT().SetLectureHall([]uint{7}, uint(4)).Return(nil)
		m.streams.EXPECT().ToggleVisibility(uint(7), true).Return(nil)
		m.audits.EXPECT().Create(gomock.Any()).Return(nil)
		m.streams.EXPECT().UpdateStream(gomock.Any()).DoAndReturn(func(s model.Stream) error {
			if !s.Start.Equal(newStart) || !s.End.Equal(newStart.Add(time.Hour)) || !s.ChatEnabled || s.Name != "Old" {
				t.Errorf("saved %+v", s)
			}
			return nil
		})

		_, err := api.UpdateLecture(asCaller(lectureAdmin), &protobuf.UpdateLectureRequest{
			CourseId: 1, StreamId: 7,
			Start: timestamppb.New(newStart), End: timestamppb.New(newStart.Add(time.Hour)),
			LectureHallId: ptrU(4), ChatEnabled: ptrB(true), Private: ptrB(true),
		})
		if err != nil {
			t.Fatalf("UpdateLecture: %v", err)
		}
	})

	t.Run("hall 0 takes the lecture out of its hall", func(t *testing.T) {
		api, m := newLectureAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(courseOneLecture(), nil)
		m.streams.EXPECT().UnsetLectureHall([]uint{7}).Return(nil)
		m.streams.EXPECT().UpdateStream(gomock.Any()).Return(nil)

		if _, err := api.UpdateLecture(asCaller(lectureAdmin), &protobuf.UpdateLectureRequest{
			CourseId: 1, StreamId: 7, LectureHallId: ptrU(0),
		}); err != nil {
			t.Fatalf("UpdateLecture: %v", err)
		}
	})

	for name, req := range map[string]*protobuf.UpdateLectureRequest{
		"a start without an end": {CourseId: 1, StreamId: 7, Start: timestamppb.New(newStart)},
		"an end without a start": {CourseId: 1, StreamId: 7, End: timestamppb.New(newStart)},
		"an end before the start": {CourseId: 1, StreamId: 7,
			Start: timestamppb.New(newStart), End: timestamppb.New(newStart.Add(-time.Minute))},
		"an end equal to the start": {CourseId: 1, StreamId: 7,
			Start: timestamppb.New(newStart), End: timestamppb.New(newStart)},
	} {
		t.Run("rejects "+name+" and writes nothing", func(t *testing.T) {
			api, m := newLectureAPI(t)
			m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(courseOneLecture(), nil)
			_, err := api.UpdateLecture(asCaller(lectureAdmin), req)
			wantCode(t, err, codes.InvalidArgument)
		})
	}

	t.Run("404s a missing hall and writes nothing", func(t *testing.T) {
		api, m := newLectureAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(courseOneLecture(), nil)
		m.halls.EXPECT().GetLectureHallByID(uint(99)).Return(model.LectureHall{}, gorm.ErrRecordNotFound)

		_, err := api.UpdateLecture(asCaller(lectureAdmin), &protobuf.UpdateLectureRequest{
			CourseId: 1, StreamId: 7, LectureHallId: ptrU(99), Name: new(string),
		})
		wantCode(t, err, codes.NotFound)
	})

	t.Run("404s another course's lecture and writes nothing", func(t *testing.T) {
		api, m := newLectureAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(foreignLecture(), nil)

		_, err := api.UpdateLecture(asCaller(lectureAdmin), &protobuf.UpdateLectureRequest{
			CourseId: 1, StreamId: 7, Private: ptrB(true), LectureHallId: ptrU(0),
		})
		wantCode(t, err, codes.NotFound)
	})
}

func TestUpdateLectureSeries(t *testing.T) {
	name := "  Hopfen "

	t.Run("applies the fields given to the course's series", func(t *testing.T) {
		api, m := newLectureAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(courseOneLecture(), nil)
		m.halls.EXPECT().GetLectureHallByID(uint(4)).Return(model.LectureHall{Model: gorm.Model{ID: 4}}, nil)
		m.streams.EXPECT().UpdateCourseLectureSeries(uint(1), "s1", gomock.Any()).
			DoAndReturn(func(_ uint, _ string, u dao.LectureSeriesUpdate) error {
				if u.Name == nil || *u.Name != "Hopfen" || u.Description != nil || u.ChatEnabled != nil ||
					u.LectureHallID == nil || *u.LectureHallID != 4 {
					t.Errorf("update = %+v", u)
				}
				return nil
			})

		hall := uint32(4)
		_, err := api.UpdateLectureSeries(asCaller(lectureAdmin), &protobuf.UpdateLectureSeriesRequest{
			CourseId: 1, StreamId: 7, Name: &name, LectureHallId: &hall,
		})
		if err != nil {
			t.Fatalf("UpdateLectureSeries: %v", err)
		}
	})

	t.Run("refuses a lecture in no series", func(t *testing.T) {
		api, m := newLectureAPI(t)
		lone := courseOneLecture()
		lone.SeriesIdentifier = ""
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(lone, nil)

		_, err := api.UpdateLectureSeries(asCaller(lectureAdmin), &protobuf.UpdateLectureSeriesRequest{
			CourseId: 1, StreamId: 7, Name: &name,
		})
		wantCode(t, err, codes.InvalidArgument)
	})

	t.Run("404s another course's lecture and writes nothing", func(t *testing.T) {
		api, m := newLectureAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(foreignLecture(), nil)

		_, err := api.UpdateLectureSeries(asCaller(lectureAdmin), &protobuf.UpdateLectureSeriesRequest{
			CourseId: 1, StreamId: 7, Name: &name,
		})
		wantCode(t, err, codes.NotFound)
	})
}

func TestUpdateLectureSeriesTime(t *testing.T) {
	req := func() *protobuf.UpdateLectureSeriesTimeRequest {
		return &protobuf.UpdateLectureSeriesTimeRequest{
			CourseId: 1, StreamId: 7,
			Start: timestamppb.New(lectureStart.Add(time.Hour)), End: timestamppb.New(lectureEnd.Add(time.Hour)),
		}
	}

	t.Run("moves the lecture and its series in the course", func(t *testing.T) {
		api, m := newLectureAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(courseOneLecture(), nil)
		m.streams.EXPECT().UpdateCourseLectureSeriesTime(uint(1), uint(7), "s1",
			lectureStart.Add(time.Hour), lectureEnd.Add(time.Hour)).Return(nil)

		if _, err := api.UpdateLectureSeriesTime(asCaller(lectureAdmin), req()); err != nil {
			t.Fatalf("UpdateLectureSeriesTime: %v", err)
		}
	})

	for name, mutate := range map[string]func(*protobuf.UpdateLectureSeriesTimeRequest){
		"no times":            func(r *protobuf.UpdateLectureSeriesTimeRequest) { r.Start, r.End = nil, nil },
		"only a start":        func(r *protobuf.UpdateLectureSeriesTimeRequest) { r.End = nil },
		"an end before start": func(r *protobuf.UpdateLectureSeriesTimeRequest) { r.Start, r.End = r.End, r.Start },
	} {
		t.Run("rejects "+name, func(t *testing.T) {
			api, m := newLectureAPI(t)
			m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(courseOneLecture(), nil)
			r := req()
			mutate(r)
			_, err := api.UpdateLectureSeriesTime(asCaller(lectureAdmin), r)
			wantCode(t, err, codes.InvalidArgument)
		})
	}

	t.Run("404s another course's lecture and writes nothing", func(t *testing.T) {
		api, m := newLectureAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(foreignLecture(), nil)
		_, err := api.UpdateLectureSeriesTime(asCaller(lectureAdmin), req())
		wantCode(t, err, codes.NotFound)
	})
}

func TestDeleteLectures(t *testing.T) {
	t.Run("deletes and audits each of the course's lectures", func(t *testing.T) {
		api, m := newLectureAPI(t)
		second := courseOneLecture()
		second.ID = 8
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(courseOneLecture(), nil)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "8").Return(second, nil)
		m.courses.EXPECT().GetCourseById(gomock.Any(), uint(1)).Return(model.Course{Model: gorm.Model{ID: 1}, Name: "Bier"}, nil)
		m.audits.EXPECT().Create(gomock.Any()).Return(nil).Times(2)
		m.streams.EXPECT().DeleteStream("7")
		m.streams.EXPECT().DeleteStream("8")

		if _, err := api.DeleteLectures(asCaller(lectureAdmin), &protobuf.DeleteLecturesRequest{
			CourseId: 1, StreamIds: []uint32{7, 8},
		}); err != nil {
			t.Fatalf("DeleteLectures: %v", err)
		}
	})

	t.Run("deletes nothing when one lecture is another course's", func(t *testing.T) {
		api, m := newLectureAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(courseOneLecture(), nil)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "9").Return(foreignLecture(), nil)

		_, err := api.DeleteLectures(asCaller(lectureAdmin), &protobuf.DeleteLecturesRequest{
			CourseId: 1, StreamIds: []uint32{7, 9},
		})
		wantCode(t, err, codes.NotFound)
	})

	t.Run("deletes nothing when one lecture is missing", func(t *testing.T) {
		api, m := newLectureAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(model.Stream{}, gorm.ErrRecordNotFound)

		_, err := api.DeleteLectures(asCaller(lectureAdmin), &protobuf.DeleteLecturesRequest{
			CourseId: 1, StreamIds: []uint32{7},
		})
		wantCode(t, err, codes.NotFound)
	})

	t.Run("refuses an empty list", func(t *testing.T) {
		api, _ := newLectureAPI(t)
		_, err := api.DeleteLectures(asCaller(lectureAdmin), &protobuf.DeleteLecturesRequest{CourseId: 1})
		wantCode(t, err, codes.InvalidArgument)
	})
}

func TestDeleteLectureSeries(t *testing.T) {
	t.Run("deletes the course's series and audits it", func(t *testing.T) {
		api, m := newLectureAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(courseOneLecture(), nil)
		m.courses.EXPECT().GetCourseById(gomock.Any(), uint(1)).Return(model.Course{Model: gorm.Model{ID: 1}}, nil)
		m.audits.EXPECT().Create(gomock.Any()).Return(nil)
		m.streams.EXPECT().DeleteCourseLectureSeries(uint(1), "s1").Return(nil)

		if _, err := api.DeleteLectureSeries(asCaller(lectureAdmin), &protobuf.DeleteLectureSeriesRequest{
			CourseId: 1, StreamId: 7,
		}); err != nil {
			t.Fatalf("DeleteLectureSeries: %v", err)
		}
	})

	t.Run("refuses a lecture in no series", func(t *testing.T) {
		api, m := newLectureAPI(t)
		lone := courseOneLecture()
		lone.SeriesIdentifier = ""
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(lone, nil)

		_, err := api.DeleteLectureSeries(asCaller(lectureAdmin), &protobuf.DeleteLectureSeriesRequest{CourseId: 1, StreamId: 7})
		wantCode(t, err, codes.InvalidArgument)
	})

	t.Run("404s another course's lecture and deletes nothing", func(t *testing.T) {
		api, m := newLectureAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(foreignLecture(), nil)

		_, err := api.DeleteLectureSeries(asCaller(lectureAdmin), &protobuf.DeleteLectureSeriesRequest{CourseId: 1, StreamId: 7})
		wantCode(t, err, codes.NotFound)
	})
}

func TestCopyLecture(t *testing.T) {
	// The caller administers courses 1 and 3, not 2.
	caller := &model.User{Model: gorm.Model{ID: 2}, Role: model.LecturerType,
		AdministeredCourses: []model.Course{{Model: gorm.Model{ID: 1}}, {Model: gorm.Model{ID: 3}}}}
	courseThree := model.Course{Model: gorm.Model{ID: 3}}

	withChildren := func() model.Stream {
		s := courseOneLecture()
		s.VideoSections = []model.VideoSection{{Model: gorm.Model{ID: 11}, StreamID: 7, Description: "Intro"}}
		s.Files = []model.File{{Model: gorm.Model{ID: 12}, StreamID: 7, Path: "/a.pdf"}}
		return s
	}

	t.Run("move creates the copy, then deletes the original", func(t *testing.T) {
		api, m := newLectureAPI(t)
		original := withChildren()
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(original, nil)
		m.courses.EXPECT().GetCourseById(gomock.Any(), uint(3)).Return(courseThree, nil)
		created := m.streams.EXPECT().CreateStream(gomock.Any()).DoAndReturn(func(s *model.Stream) error {
			if s.ID != 0 || s.CourseID != 3 || s.Name != "Old" || s.StreamKey != "secret" {
				t.Errorf("created %+v", s)
			}
			if s.VideoSections[0].ID != 0 || s.Files[0].ID != 0 {
				t.Error("the copy reuses the original's child rows")
			}
			s.ID = 99
			return nil
		})
		m.audits.EXPECT().Create(gomock.Any()).Return(nil)
		// The original, not the copy v1 deleted.
		m.streams.EXPECT().DeleteStream("7").After(created)

		resp, err := api.CopyLecture(asCaller(caller), &protobuf.CopyLectureRequest{
			CourseId: 1, StreamId: 7, TargetCourseId: 3, Move: true,
		})
		if err != nil {
			t.Fatalf("CopyLecture: %v", err)
		}
		if resp.StreamId != 99 {
			t.Errorf("stream id = %d, want the copy's", resp.StreamId)
		}
		if original.VideoSections[0].ID != 11 || original.Files[0].ID != 12 {
			t.Error("copying changed the original's (cached) children")
		}
	})

	t.Run("a copy keeps the original and gets its own stream key", func(t *testing.T) {
		api, m := newLectureAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(withChildren(), nil)
		m.courses.EXPECT().GetCourseById(gomock.Any(), uint(3)).Return(courseThree, nil)
		m.streams.EXPECT().CreateStream(gomock.Any()).DoAndReturn(func(s *model.Stream) error {
			if s.StreamKey == "secret" || s.StreamKey == "" {
				t.Errorf("stream key = %q, want a fresh one", s.StreamKey)
			}
			s.ID = 99
			return nil
		})

		if _, err := api.CopyLecture(asCaller(caller), &protobuf.CopyLectureRequest{
			CourseId: 1, StreamId: 7, TargetCourseId: 3,
		}); err != nil {
			t.Fatalf("CopyLecture: %v", err)
		}
	})

	t.Run("a failed create deletes nothing", func(t *testing.T) {
		api, m := newLectureAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(withChildren(), nil)
		m.courses.EXPECT().GetCourseById(gomock.Any(), uint(3)).Return(courseThree, nil)
		m.streams.EXPECT().CreateStream(gomock.Any()).Return(errors.New("disk full"))

		_, err := api.CopyLecture(asCaller(caller), &protobuf.CopyLectureRequest{
			CourseId: 1, StreamId: 7, TargetCourseId: 3, Move: true,
		})
		// WithStatus answers a 500 as Unknown.
		wantCode(t, err, codes.Unknown)
	})

	t.Run("a target the caller does not administer is 404, like a missing one", func(t *testing.T) {
		api, m := newLectureAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(withChildren(), nil)
		m.courses.EXPECT().GetCourseById(gomock.Any(), uint(2)).Return(model.Course{Model: gorm.Model{ID: 2}, UserID: 77}, nil)

		_, err := api.CopyLecture(asCaller(caller), &protobuf.CopyLectureRequest{
			CourseId: 1, StreamId: 7, TargetCourseId: 2, Move: true,
		})
		wantCode(t, err, codes.NotFound)
	})

	t.Run("a missing target is 404", func(t *testing.T) {
		api, m := newLectureAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(withChildren(), nil)
		m.courses.EXPECT().GetCourseById(gomock.Any(), uint(42)).Return(model.Course{}, nil)

		_, err := api.CopyLecture(asCaller(caller), &protobuf.CopyLectureRequest{
			CourseId: 1, StreamId: 7, TargetCourseId: 42,
		})
		wantCode(t, err, codes.NotFound)
	})

	t.Run("another course's lecture is 404 and nothing is copied", func(t *testing.T) {
		api, m := newLectureAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(foreignLecture(), nil)

		_, err := api.CopyLecture(asCaller(caller), &protobuf.CopyLectureRequest{
			CourseId: 1, StreamId: 7, TargetCourseId: 3,
		})
		wantCode(t, err, codes.NotFound)
	})

	t.Run("refuses to move a lecture into its own course", func(t *testing.T) {
		api, m := newLectureAPI(t)
		m.streams.EXPECT().GetStreamByID(gomock.Any(), "7").Return(withChildren(), nil)
		m.courses.EXPECT().GetCourseById(gomock.Any(), uint(1)).Return(model.Course{Model: gorm.Model{ID: 1}}, nil)

		_, err := api.CopyLecture(asCaller(caller), &protobuf.CopyLectureRequest{
			CourseId: 1, StreamId: 7, TargetCourseId: 1, Move: true,
		})
		wantCode(t, err, codes.InvalidArgument)
	})
}
