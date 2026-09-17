package apiv2

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"go.uber.org/mock/gomock"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"

	campusonline "github.com/RBG-TUM/CAMPUSOnline"
	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
)

// fakeCampusScheduleClient is a hand-rolled double for campusScheduleClient: the real
// implementation talks to TUMonline over HTTP, so tests substitute this instead of
// reaching for gomock over an interface with only two methods.
type fakeCampusScheduleClient struct {
	xCalResult campusonline.ICalendar
	xCalErr    error

	enrichResult []campusonline.Course
	enrichErr    error
}

func (f *fakeCampusScheduleClient) GetXCalOrg(_ time.Time, _ time.Time, _ int) (campusonline.ICalendar, error) {
	return f.xCalResult, f.xCalErr
}

func (f *fakeCampusScheduleClient) EnrichCourse(_ []campusonline.Course) ([]campusonline.Course, error) {
	return f.enrichResult, f.enrichErr
}

// withCampusScheduleClient swaps newCampusScheduleClient for the duration of a test
// and restores it afterwards, so tests cannot bleed into each other or, worse, into a
// real TUMonline request from a test run without a configured token.
func withCampusScheduleClient(t *testing.T, client campusScheduleClient, err error) {
	t.Helper()
	original := newCampusScheduleClient
	newCampusScheduleClient = func() (campusScheduleClient, error) { return client, err }
	t.Cleanup(func() { newCampusScheduleClient = original })
}

func TestSearchCourseImportSchedule(t *testing.T) {
	from := timestamppb.New(time.Date(2025, 10, 1, 0, 0, 0, 0, time.UTC))
	to := timestamppb.New(time.Date(2025, 10, 8, 0, 0, 0, 0, time.UTC))

	t.Run("rejects a request missing the date range", func(t *testing.T) {
		api := &API{log: slog.Default()}

		_, err := api.SearchCourseImportSchedule(context.Background(), &protobuf.CourseImportSearchRequest{DepartmentId: 53598})
		if err == nil {
			t.Fatal("a missing date range was not rejected")
		}
	})

	t.Run("rejects a request missing the department", func(t *testing.T) {
		api := &API{log: slog.Default()}

		_, err := api.SearchCourseImportSchedule(context.Background(), &protobuf.CourseImportSearchRequest{From: from, To: to})
		if err == nil {
			t.Fatal("a missing department id was not rejected")
		}
	})

	t.Run("surfaces a TUMonline schedule fetch failure instead of returning an empty page", func(t *testing.T) {
		// The v1 handler this replaces logged this error and returned, leaving the
		// caller waiting on a response that never came. That bug is what this guards.
		withCampusScheduleClient(t, &fakeCampusScheduleClient{xCalErr: errors.New("TUMonline is down")}, nil)
		api := &API{log: slog.Default()}

		_, err := api.SearchCourseImportSchedule(context.Background(), &protobuf.CourseImportSearchRequest{
			From: from, To: to, DepartmentId: 53598,
		})
		if err == nil {
			t.Fatal("a failed schedule fetch was not reported as an error")
		}
	})

	t.Run("surfaces a course enrichment failure", func(t *testing.T) {
		withCampusScheduleClient(t, &fakeCampusScheduleClient{enrichErr: errors.New("contacts API is down")}, nil)
		api := &API{log: slog.Default()}

		_, err := api.SearchCourseImportSchedule(context.Background(), &protobuf.CourseImportSearchRequest{
			From: from, To: to, DepartmentId: 53598,
		})
		if err == nil {
			t.Fatal("a failed enrichment call was not reported as an error")
		}
	})

	t.Run("surfaces a client construction failure, such as a missing campus token", func(t *testing.T) {
		withCampusScheduleClient(t, nil, errors.New("no campus token configured"))
		api := &API{log: slog.Default()}

		_, err := api.SearchCourseImportSchedule(context.Background(), &protobuf.CourseImportSearchRequest{
			From: from, To: to, DepartmentId: 53598,
		})
		if err == nil {
			t.Fatal("a client construction failure was not reported as an error")
		}
	})

	t.Run("returns the enriched courses found in the schedule", func(t *testing.T) {
		withCampusScheduleClient(t, &fakeCampusScheduleClient{
			enrichResult: []campusonline.Course{
				{
					Title: "Grundlagen", Slug: "GDB", CourseID: 42, Language: "de",
					Events: []campusonline.Event{
						{RoomName: "MI HS1", Start: time.Now(), End: time.Now().Add(time.Hour), EventID: "1"},
					},
					Contacts: []campusonline.ContactPerson{
						{FirstName: "Ada", LastName: "Lovelace", Email: "ada@example.com", MainContact: true},
					},
				},
			},
		}, nil)
		api := &API{log: slog.Default()}

		resp, err := api.SearchCourseImportSchedule(context.Background(), &protobuf.CourseImportSearchRequest{
			From: from, To: to, DepartmentId: 53598,
		})
		if err != nil {
			t.Fatalf("SearchCourseImportSchedule: %v", err)
		}
		if len(resp.GetCourses()) != 1 {
			t.Fatalf("got %d courses, want 1", len(resp.GetCourses()))
		}
		course := resp.GetCourses()[0]
		if course.GetTitle() != "Grundlagen" || course.GetCourseId() != 42 || course.GetLanguage() != "de" {
			t.Errorf("got %+v", course)
		}
		if !course.GetImport() {
			t.Error("a freshly found course should default to selected for import")
		}
		if len(course.GetEvents()) != 1 || len(course.GetContacts()) != 1 {
			t.Errorf("events/contacts were not carried over: %+v", course)
		}
	})
}

func TestImportCourseImportCourses(t *testing.T) {
	ctxWithUser := func(u *model.User) context.Context {
		return context.WithValue(context.Background(), callerKey{}, &caller{user: u})
	}
	admin := &model.User{Model: gorm.Model{ID: 1}, Name: "Admin"}

	t.Run("rejects a bad term", func(t *testing.T) {
		api := &API{log: slog.Default()}

		_, err := api.ImportCourseImportCourses(ctxWithUser(admin), &protobuf.CourseImportRequest{Year: 2025, Term: "X"})
		if err == nil {
			t.Fatal("an invalid term was not rejected")
		}
	})

	t.Run("rejects a missing year", func(t *testing.T) {
		api := &API{log: slog.Default()}

		_, err := api.ImportCourseImportCourses(ctxWithUser(admin), &protobuf.CourseImportRequest{Term: "W"})
		if err == nil {
			t.Fatal("a missing year was not rejected")
		}
	})

	t.Run("skips courses not marked for import", func(t *testing.T) {
		api := &API{log: slog.Default(), dao: dao.DaoWrapper{}}

		resp, err := api.ImportCourseImportCourses(ctxWithUser(admin), &protobuf.CourseImportRequest{
			Year: 2025, Term: "W",
			Courses: []*protobuf.CourseImportCourse{{Title: "Skipped", Import: false}},
		})
		if err != nil {
			t.Fatalf("ImportCourseImportCourses: %v", err)
		}
		if len(resp.GetResults()) != 0 {
			t.Fatalf("a course not marked for import was still reported: %+v", resp.GetResults())
		}
	})

	t.Run("reports one course's failure without dropping another's success", func(t *testing.T) {
		// The behaviour the v1 handler lacked: it concatenated every error into one
		// string and 500'd the whole batch, so a single bad course hid every other
		// result from the caller.
		ctrl := gomock.NewController(t)
		coursesMock := mock_dao.NewMockCoursesDao(ctrl)
		coursesMock.EXPECT().CreateCourse(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("duplicate slug")).Times(1)
		coursesMock.EXPECT().CreateCourse(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)

		api := &API{log: slog.Default(), dao: dao.DaoWrapper{CoursesDao: coursesMock}}

		resp, err := api.ImportCourseImportCourses(ctxWithUser(admin), &protobuf.CourseImportRequest{
			Year: 2025, Term: "W",
			Courses: []*protobuf.CourseImportCourse{
				{Title: "Broken", Slug: "broken", Import: true},
				{Title: "Fine", Slug: "fine", Import: true},
			},
		})
		if err != nil {
			t.Fatalf("ImportCourseImportCourses: %v", err)
		}
		if len(resp.GetResults()) != 2 {
			t.Fatalf("got %d results, want 2", len(resp.GetResults()))
		}
		if resp.GetResults()[0].GetSuccess() || resp.GetResults()[0].GetError() == "" {
			t.Errorf("the failing course should report success=false with its error: %+v", resp.GetResults()[0])
		}
		if !resp.GetResults()[1].GetSuccess() {
			t.Errorf("the course after a failing one should still succeed: %+v", resp.GetResults()[1])
		}
	})

	t.Run("does not create a stream for a room TUMonline reports that we have no lecture hall for", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		lectureHallMock := mock_dao.NewMockLectureHallsDao(ctrl)
		lectureHallMock.EXPECT().GetLectureHallByPartialName("Unknown Room").
			Return(model.LectureHall{}, errors.New("not found")).Times(1)
		coursesMock := mock_dao.NewMockCoursesDao(ctrl)
		coursesMock.EXPECT().CreateCourse(gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, c *model.Course, _ bool) error {
				if len(c.Streams) != 0 {
					t.Errorf("a stream was created for an unresolvable room: %+v", c.Streams)
				}
				return nil
			}).Times(1)

		api := &API{log: slog.Default(), dao: dao.DaoWrapper{CoursesDao: coursesMock, LectureHallsDao: lectureHallMock}}

		resp, err := api.ImportCourseImportCourses(ctxWithUser(admin), &protobuf.CourseImportRequest{
			Year: 2025, Term: "W",
			Courses: []*protobuf.CourseImportCourse{{
				Title: "Course", Slug: "course", Import: true,
				Events: []*protobuf.CourseImportEvent{{RoomName: "Unknown Room", Import: true}},
			}},
		})
		if err != nil {
			t.Fatalf("ImportCourseImportCourses: %v", err)
		}
		if !resp.GetResults()[0].GetSuccess() {
			t.Errorf("the course should still import without the one unresolvable event: %+v", resp.GetResults()[0])
		}
	})
}
