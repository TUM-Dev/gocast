package apiv2

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/emptypb"
	"gorm.io/gorm"

	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
)

// courseAdminMocks are the daos the course administration handlers reach. Every
// handler test starts from these, with course 5 loadable, and expects the rest.
type courseAdminMocks struct {
	courses *mock_dao.MockCoursesDao
	users   *mock_dao.MockUsersDao
	streams *mock_dao.MockStreamsDao
	halls   *mock_dao.MockLectureHallsDao
	audits  *mock_dao.MockAuditDao
	emails  *mock_dao.MockEmailDao
}

var (
	adminCourse = model.Course{
		Model: gorm.Model{ID: 5}, UserID: 1, Name: "Brauereiwesen", Slug: "brau", Year: 2022, TeachingTerm: "S",
		Visibility: "loggedin", VODEnabled: true, ChatEnabled: true, AnonymousChatEnabled: true,
		Language: sql.NullString{String: "de", Valid: true},
	}
	courseLecturer = &model.User{Model: gorm.Model{ID: 2}, Name: "Peter", Role: model.LecturerType}
)

func courseAdminAPI(t *testing.T, course model.Course) (*API, courseAdminMocks) {
	t.Helper()
	ctrl := gomock.NewController(t)
	m := courseAdminMocks{
		courses: mock_dao.NewMockCoursesDao(ctrl),
		users:   mock_dao.NewMockUsersDao(ctrl),
		streams: mock_dao.NewMockStreamsDao(ctrl),
		halls:   mock_dao.NewMockLectureHallsDao(ctrl),
		audits:  mock_dao.NewMockAuditDao(ctrl),
		emails:  mock_dao.NewMockEmailDao(ctrl),
	}
	m.courses.EXPECT().GetCourseById(gomock.Any(), uint(5)).Return(course, nil).AnyTimes()

	return &API{
		dao: dao.DaoWrapper{
			CoursesDao: m.courses, UsersDao: m.users, StreamsDao: m.streams,
			LectureHallsDao: m.halls, AuditDao: m.audits, EmailDao: m.emails,
		},
		log: slog.Default(),
	}, m
}

// Course 0 is how the statistics queries spell "every course". The policy refuses
// it already; each handler refuses it again rather than trusting that, and must do
// so before reaching any dao (the mocks expect nothing for it).
func TestCourseAdminRefusesCourseZero(t *testing.T) {
	api, m := courseAdminAPI(t, adminCourse)
	// What GetCourseById's Find answers for course 0.
	m.courses.EXPECT().GetCourseById(gomock.Any(), uint(0)).Return(model.Course{}, nil).AnyTimes()
	ctx := asCaller(courseLecturer)

	calls := map[string]func() error{
		"getCourseAdmin": func() error {
			_, err := api.GetCourseAdmin(ctx, &protobuf.GetCourseAdminRequest{})
			return err
		},
		"updateCourseSettings": func() error {
			v := "public"
			_, err := api.UpdateCourseSettings(ctx, &protobuf.UpdateCourseSettingsRequest{Visibility: &v})
			return err
		},
		"copyCourse": func() error {
			_, err := api.CopyCourse(ctx, &protobuf.CopyCourseRequest{Year: 2026, Term: "W"})
			return err
		},
		"deleteCourse": func() error {
			_, err := api.DeleteCourse(ctx, &protobuf.DeleteCourseRequest{})
			return err
		},
		"listCourseAdmins": func() error {
			_, err := api.ListCourseAdmins(ctx, &protobuf.ListCourseAdminsRequest{})
			return err
		},
		"addCourseAdmin": func() error {
			_, err := api.AddCourseAdmin(ctx, &protobuf.AddCourseAdminRequest{UserId: 3})
			return err
		},
		"removeCourseAdmin": func() error {
			_, err := api.RemoveCourseAdmin(ctx, &protobuf.RemoveCourseAdminRequest{UserId: 3})
			return err
		},
		"searchUsersForCourse": func() error {
			_, err := api.SearchUsersForCourse(ctx, &protobuf.SearchUsersForCourseRequest{Q: "prof"})
			return err
		},
		"listCourseLectureHallSettings": func() error {
			_, err := api.ListCourseLectureHallSettings(ctx, &protobuf.ListCourseLectureHallSettingsRequest{})
			return err
		},
		"updateCourseLectureHallSettings": func() error {
			_, err := api.UpdateCourseLectureHallSettings(ctx, &protobuf.UpdateCourseLectureHallSettingsRequest{})
			return err
		},
		"listCourseParticipants": func() error {
			_, err := api.ListCourseParticipants(ctx, &protobuf.ListCourseParticipantsRequest{})
			return err
		},
		"inviteCourseParticipants": func() error {
			_, err := api.InviteCourseParticipants(ctx, &protobuf.InviteCourseParticipantsRequest{
				Invitees: []*protobuf.CourseInvitee{{Name: "Tim", Email: "tim@example.org"}},
			})
			return err
		},
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) { wantCode(t, call(), codes.NotFound) })
	}
}

func TestGetCourseAdmin(t *testing.T) {
	api, _ := courseAdminAPI(t, adminCourse)

	resp, err := api.GetCourseAdmin(asCaller(courseLecturer), &protobuf.GetCourseAdminRequest{CourseId: 5})
	if err != nil {
		t.Fatalf("GetCourseAdmin: %v", err)
	}
	if resp.Id != 5 || resp.Slug != "brau" || resp.Term != "S" || resp.Visibility != "loggedin" {
		t.Errorf("got %+v", resp)
	}
	if !resp.VodEnabled || resp.DownloadsEnabled || !resp.ChatEnabled || resp.Language != "de" || resp.UserId != 1 {
		t.Errorf("settings = %+v", resp)
	}
}

func TestUpdateCourseSettings(t *testing.T) {
	t.Run("writes only the fields sent, false included, and audits them", func(t *testing.T) {
		api, m := courseAdminAPI(t, adminCourse)
		var written model.Course
		m.courses.EXPECT().UpdateCourseColumns(gomock.Any(), gomock.Any(), "visibility", "vod_enabled").
			DoAndReturn(func(_ context.Context, c model.Course, _ ...string) error {
				written = c
				return nil
			})
		m.audits.EXPECT().Create(gomock.Any()).DoAndReturn(func(a *model.Audit) error {
			if a.Type != model.AuditCourseEdit || !strings.Contains(a.Message, "visibility=hidden") || a.User.ID != courseLecturer.ID {
				t.Errorf("audit = %+v", a)
			}
			return nil
		})

		visibility, vod := "hidden", false
		resp, err := api.UpdateCourseSettings(asCaller(courseLecturer), &protobuf.UpdateCourseSettingsRequest{
			CourseId: 5, Visibility: &visibility, VodEnabled: &vod,
		})
		if err != nil {
			t.Fatalf("UpdateCourseSettings: %v", err)
		}
		if written.Visibility != "hidden" || written.VODEnabled {
			t.Errorf("written = %q vod=%v", written.Visibility, written.VODEnabled)
		}
		if resp.Visibility != "hidden" || resp.VodEnabled || !resp.ChatEnabled {
			t.Errorf("answered %+v", resp)
		}
	})

	t.Run("writes nothing when nothing is sent", func(t *testing.T) {
		api, _ := courseAdminAPI(t, adminCourse)

		resp, err := api.UpdateCourseSettings(asCaller(courseLecturer), &protobuf.UpdateCourseSettingsRequest{CourseId: 5})
		if err != nil {
			t.Fatalf("UpdateCourseSettings: %v", err)
		}
		if resp.Visibility != "loggedin" {
			t.Errorf("visibility = %q", resp.Visibility)
		}
	})

	// v1's unanchored regexp let any value containing a visibility through.
	for _, v := range []string{"", "publicly", "everyone"} {
		t.Run("rejects the visibility "+v, func(t *testing.T) {
			api, _ := courseAdminAPI(t, adminCourse)
			_, err := api.UpdateCourseSettings(asCaller(courseLecturer), &protobuf.UpdateCourseSettingsRequest{CourseId: 5, Visibility: &v})
			wantCode(t, err, codes.InvalidArgument)
		})
	}

	t.Run("reports a failed write", func(t *testing.T) {
		api, m := courseAdminAPI(t, adminCourse)
		m.courses.EXPECT().UpdateCourseColumns(gomock.Any(), gomock.Any(), "chat_enabled").Return(errors.New("gone"))

		chat := false
		_, err := api.UpdateCourseSettings(asCaller(courseLecturer), &protobuf.UpdateCourseSettingsRequest{CourseId: 5, ChatEnabled: &chat})
		wantCode(t, err, codes.Unknown)
	})
}

func TestCopyCourse(t *testing.T) {
	withStreams := adminCourse
	withStreams.Streams = []model.Stream{
		{Model: gorm.Model{ID: 10}, CourseID: 5, Name: "VL 1", Files: []model.File{{Model: gorm.Model{ID: 3}}}},
		{Model: gorm.Model{ID: 11}, CourseID: 5, Name: "VL 2"},
	}

	t.Run("copies the course, its lectures and its admins into the semester", func(t *testing.T) {
		api, m := courseAdminAPI(t, withStreams)
		m.courses.EXPECT().GetCourseBySlugYearAndTerm(gomock.Any(), "brau", "W", 2026).Return(model.Course{}, gorm.ErrRecordNotFound)
		m.courses.EXPECT().GetCourseAdmins(uint(5)).Return([]model.User{{Model: gorm.Model{ID: 2}}, {Model: gorm.Model{ID: 3}}}, nil)
		m.courses.EXPECT().CreateCourse(gomock.Any(), gomock.Any(), true).
			DoAndReturn(func(_ context.Context, c *model.Course, _ bool) error {
				if c.ID != 0 || c.Year != 2026 || c.TeachingTerm != "W" || c.Slug != "brau" || len(c.Streams) != 0 {
					t.Errorf("created %+v", c)
				}
				c.ID = 77
				return nil
			})
		var copied []model.Stream
		m.streams.EXPECT().CreateStream(gomock.Any()).DoAndReturn(func(s *model.Stream) error {
			copied = append(copied, *s)
			return nil
		}).Times(2)
		m.courses.EXPECT().AddAdminToCourse(uint(2), uint(77)).Return(nil)
		m.courses.EXPECT().AddAdminToCourse(uint(3), uint(77)).Return(errors.New("gone"))
		m.audits.EXPECT().Create(gomock.Any()).Return(nil)

		resp, err := api.CopyCourse(asCaller(courseLecturer), &protobuf.CopyCourseRequest{CourseId: 5, Year: 2026, Term: "W"})
		if err != nil {
			t.Fatalf("CopyCourse: %v", err)
		}
		if resp.CourseId != 77 || resp.NumErrors != 1 {
			t.Errorf("got %+v, want course 77 with one error", resp)
		}
		for _, s := range copied {
			if s.ID != 0 || s.CourseID != 77 || len(s.Files) != 0 {
				t.Errorf("copied stream %d to course %d with %d files", s.ID, s.CourseID, len(s.Files))
			}
		}
	})

	t.Run("refuses a semester that already has the slug", func(t *testing.T) {
		api, m := courseAdminAPI(t, withStreams)
		m.courses.EXPECT().GetCourseBySlugYearAndTerm(gomock.Any(), "brau", "W", 2026).Return(model.Course{}, nil)

		_, err := api.CopyCourse(asCaller(courseLecturer), &protobuf.CopyCourseRequest{CourseId: 5, Year: 2026, Term: "W"})
		wantCode(t, err, codes.AlreadyExists)
	})

	for name, req := range map[string]*protobuf.CopyCourseRequest{
		"a two-digit year": {CourseId: 5, Year: 26, Term: "W"},
		"v1's term names":  {CourseId: 5, Year: 2026, Term: "Wintersemester"},
	} {
		t.Run("rejects "+name, func(t *testing.T) {
			api, _ := courseAdminAPI(t, withStreams)
			_, err := api.CopyCourse(asCaller(courseLecturer), req)
			wantCode(t, err, codes.InvalidArgument)
		})
	}
}

func TestDeleteCourse(t *testing.T) {
	api, m := courseAdminAPI(t, adminCourse)
	m.audits.EXPECT().Create(gomock.Any()).DoAndReturn(func(a *model.Audit) error {
		if a.Type != model.AuditCourseDelete || a.Message != "'Brauereiwesen' (2022, S)[5]" {
			t.Errorf("audit = %v %q", a.Type, a.Message)
		}
		return nil
	})
	m.courses.EXPECT().DeleteCourse(gomock.Any()).Do(func(c model.Course) {
		if c.ID != 5 {
			t.Errorf("deleted course %d", c.ID)
		}
	})

	if _, err := api.DeleteCourse(asCaller(courseLecturer), &protobuf.DeleteCourseRequest{CourseId: 5}); err != nil {
		t.Fatalf("DeleteCourse: %v", err)
	}
}

func TestListCourseAdmins(t *testing.T) {
	api, m := courseAdminAPI(t, adminCourse)
	m.courses.EXPECT().GetCourseAdmins(uint(5)).Return([]model.User{
		{Model: gorm.Model{ID: 2}, Name: "Peter", LrzID: "prof1", Role: model.LecturerType},
		{Model: gorm.Model{ID: 9}, Name: "Tim", Email: sql.NullString{String: "tim@example.org", Valid: true}, Role: model.LecturerType},
	}, nil)

	resp, err := api.ListCourseAdmins(asCaller(courseLecturer), &protobuf.ListCourseAdminsRequest{CourseId: 5})
	if err != nil {
		t.Fatalf("ListCourseAdmins: %v", err)
	}
	if len(resp.Admins) != 2 || resp.Admins[0].Login != "prof1" || resp.Admins[1].Login != "tim@example.org" {
		t.Errorf("admins = %v", resp.Admins)
	}
}

func TestAddCourseAdmin(t *testing.T) {
	t.Run("promotes a student to lecturer", func(t *testing.T) {
		api, m := courseAdminAPI(t, adminCourse)
		m.users.EXPECT().GetUserByID(gomock.Any(), uint(4)).
			Return(model.User{Model: gorm.Model{ID: 4}, Name: "Stephanie", Role: model.StudentType}, nil)
		m.courses.EXPECT().AddAdminToCourse(uint(4), uint(5)).Return(nil)
		m.users.EXPECT().UpdateUser(gomock.Any()).DoAndReturn(func(u model.User) error {
			if u.Role != model.LecturerType {
				t.Errorf("role = %d, want lecturer", u.Role)
			}
			return nil
		})
		m.audits.EXPECT().Create(gomock.Any()).Return(nil)

		resp, err := api.AddCourseAdmin(asCaller(courseLecturer), &protobuf.AddCourseAdminRequest{CourseId: 5, UserId: 4})
		if err != nil {
			t.Fatalf("AddCourseAdmin: %v", err)
		}
		if resp.Id != 4 || resp.Role != uint32(model.LecturerType) {
			t.Errorf("got %+v", resp)
		}
	})

	t.Run("leaves a lecturer's role alone", func(t *testing.T) {
		api, m := courseAdminAPI(t, adminCourse)
		m.users.EXPECT().GetUserByID(gomock.Any(), uint(3)).
			Return(model.User{Model: gorm.Model{ID: 3}, Role: model.LecturerType}, nil)
		m.courses.EXPECT().AddAdminToCourse(uint(3), uint(5)).Return(nil)
		// No UpdateUser expected.
		m.audits.EXPECT().Create(gomock.Any()).Return(nil)

		if _, err := api.AddCourseAdmin(asCaller(courseLecturer), &protobuf.AddCourseAdminRequest{CourseId: 5, UserId: 3}); err != nil {
			t.Fatalf("AddCourseAdmin: %v", err)
		}
	})

	t.Run("answers 404 for a user who does not exist", func(t *testing.T) {
		api, m := courseAdminAPI(t, adminCourse)
		// What GetUserByID's Find answers for a missing row.
		m.users.EXPECT().GetUserByID(gomock.Any(), uint(99)).Return(model.User{}, nil)

		_, err := api.AddCourseAdmin(asCaller(courseLecturer), &protobuf.AddCourseAdminRequest{CourseId: 5, UserId: 99})
		wantCode(t, err, codes.NotFound)
	})
}

func TestRemoveCourseAdmin(t *testing.T) {
	two := []model.User{{Model: gorm.Model{ID: 2}, Name: "Peter"}, {Model: gorm.Model{ID: 3}, Name: "Pauline"}}

	t.Run("removes an admin and audits it", func(t *testing.T) {
		api, m := courseAdminAPI(t, adminCourse)
		m.courses.EXPECT().GetCourseAdmins(uint(5)).Return(two, nil)
		m.courses.EXPECT().RemoveAdminFromCourse(uint(3), uint(5)).Return(nil)
		m.audits.EXPECT().Create(gomock.Any()).DoAndReturn(func(a *model.Audit) error {
			if a.Message != "Brauereiwesen:'brau' remove: Pauline (3)" {
				t.Errorf("audit = %q", a.Message)
			}
			return nil
		})

		resp, err := api.RemoveCourseAdmin(asCaller(courseLecturer), &protobuf.RemoveCourseAdminRequest{CourseId: 5, UserId: 3})
		if err != nil {
			t.Fatalf("RemoveCourseAdmin: %v", err)
		}
		if resp.Id != 3 {
			t.Errorf("got %+v", resp)
		}
	})

	t.Run("refuses to remove the last admin", func(t *testing.T) {
		api, m := courseAdminAPI(t, adminCourse)
		m.courses.EXPECT().GetCourseAdmins(uint(5)).Return(two[:1], nil)

		_, err := api.RemoveCourseAdmin(asCaller(courseLecturer), &protobuf.RemoveCourseAdminRequest{CourseId: 5, UserId: 2})
		wantCode(t, err, codes.InvalidArgument)
	})

	t.Run("answers 404 for someone who is not an admin", func(t *testing.T) {
		api, m := courseAdminAPI(t, adminCourse)
		m.courses.EXPECT().GetCourseAdmins(uint(5)).Return(two, nil)

		_, err := api.RemoveCourseAdmin(asCaller(courseLecturer), &protobuf.RemoveCourseAdminRequest{CourseId: 5, UserId: 4})
		wantCode(t, err, codes.NotFound)
	})
}

func TestSearchUsersForCourse(t *testing.T) {
	t.Run("answers names, logins and roles", func(t *testing.T) {
		api, m := courseAdminAPI(t, adminCourse)
		// The LIKE-significant characters are stripped before the query.
		m.users.EXPECT().SearchUser("prof").Return([]model.User{
			{Model: gorm.Model{ID: 2}, Name: "Peter", LrzID: "prof1", Role: model.LecturerType},
		}, nil)

		resp, err := api.SearchUsersForCourse(asCaller(courseLecturer), &protobuf.SearchUsersForCourseRequest{CourseId: 5, Q: "pr%of"})
		if err != nil {
			t.Fatalf("SearchUsersForCourse: %v", err)
		}
		if len(resp.Users) != 1 || resp.Users[0].Login != "prof1" || resp.Users[0].Role != uint32(model.LecturerType) {
			t.Errorf("users = %v", resp.Users)
		}
	})

	t.Run("rejects a query shorter than three characters", func(t *testing.T) {
		api, _ := courseAdminAPI(t, adminCourse)
		_, err := api.SearchUsersForCourse(asCaller(courseLecturer), &protobuf.SearchUsersForCourseRequest{CourseId: 5, Q: "p%%"})
		wantCode(t, err, codes.InvalidArgument)
	})
}

func TestCourseLectureHallSettings(t *testing.T) {
	inHall := adminCourse
	inHall.Streams = []model.Stream{{LectureHallID: 1}, {LectureHallID: 1}, {LectureHallID: 0}}
	inHall.SourcePreferences = `[{"lecture_hall_id":1,"source_mode":2}]`
	inHall.CameraPresetPreferences = `[{"lecture_hall_id":1,"preset_id":4}]`
	hall := model.LectureHall{
		Model: gorm.Model{ID: 1}, Name: "HS1",
		CameraPresets: []model.CameraPreset{{PresetID: 3, Name: "Board", LectureHallID: 1}, {PresetID: 4, Name: "Desk", LectureHallID: 1}},
	}

	t.Run("lists each hall the course has lectures in once, with its choices", func(t *testing.T) {
		api, m := courseAdminAPI(t, inHall)
		m.halls.EXPECT().GetLectureHallByID(uint(1)).Return(hall, nil).Times(1)

		resp, err := api.ListCourseLectureHallSettings(asCaller(courseLecturer), &protobuf.ListCourseLectureHallSettingsRequest{CourseId: 5})
		if err != nil {
			t.Fatalf("ListCourseLectureHallSettings: %v", err)
		}
		if len(resp.LectureHalls) != 1 {
			t.Fatalf("halls = %v", resp.LectureHalls)
		}
		got := resp.LectureHalls[0]
		if got.LectureHallName != "HS1" || got.SourceMode != protobuf.CourseSourceMode_COURSE_SOURCE_MODE_CAMERA_ONLY ||
			got.SelectedPresetId != 4 || len(got.Presets) != 2 {
			t.Errorf("got %+v", got)
		}
	})

	t.Run("replaces the settings and audits it", func(t *testing.T) {
		api, m := courseAdminAPI(t, inHall)
		m.halls.EXPECT().GetLectureHallByID(uint(1)).Return(hall, nil)
		m.courses.EXPECT().UpdateCourseColumns(gomock.Any(), gomock.Any(), "camera_preset_preferences", "source_preferences").
			DoAndReturn(func(_ context.Context, c model.Course, _ ...string) error {
				if c.GetSourceModeForLectureHall(1) != model.SourceModePRESOnly {
					t.Errorf("source preferences = %s", c.SourcePreferences)
				}
				if prefs := c.GetCameraPresetPreference(); len(prefs) != 1 || prefs[0].PresetID != 3 {
					t.Errorf("preset preferences = %s", c.CameraPresetPreferences)
				}
				return nil
			})
		m.audits.EXPECT().Create(gomock.Any()).Return(nil)

		resp, err := api.UpdateCourseLectureHallSettings(asCaller(courseLecturer), &protobuf.UpdateCourseLectureHallSettingsRequest{
			CourseId: 5,
			LectureHalls: []*protobuf.CourseLectureHallSettingUpdate{{
				LectureHallId: 1, SourceMode: protobuf.CourseSourceMode_COURSE_SOURCE_MODE_PRESENTATION_ONLY, SelectedPresetId: 3,
			}},
		})
		if err != nil {
			t.Fatalf("UpdateCourseLectureHallSettings: %v", err)
		}
		if resp.LectureHalls[0].SelectedPresetId != 3 {
			t.Errorf("answered %+v", resp.LectureHalls[0])
		}
	})

	for name, update := range map[string]*protobuf.CourseLectureHallSettingUpdate{
		"a hall the course has no lectures in": {LectureHallId: 2},
		"a preset the hall does not have":      {LectureHallId: 1, SelectedPresetId: 9},
		"an unknown source mode":               {LectureHallId: 1, SourceMode: 7},
	} {
		t.Run("rejects "+name, func(t *testing.T) {
			api, m := courseAdminAPI(t, inHall)
			m.halls.EXPECT().GetLectureHallByID(uint(1)).Return(hall, nil)

			_, err := api.UpdateCourseLectureHallSettings(asCaller(courseLecturer), &protobuf.UpdateCourseLectureHallSettingsRequest{
				CourseId: 5, LectureHalls: []*protobuf.CourseLectureHallSettingUpdate{update},
			})
			wantCode(t, err, codes.InvalidArgument)
		})
	}
}

func TestListCourseParticipants(t *testing.T) {
	api, m := courseAdminAPI(t, adminCourse)
	m.courses.EXPECT().GetInvitedUsersForCourse(gomock.Any()).DoAndReturn(func(c *model.Course) error {
		if c.ID != 5 {
			t.Errorf("asked for course %d", c.ID)
		}
		c.Users = []model.User{
			{Model: gorm.Model{ID: 7}, Name: "Tim", Email: sql.NullString{String: "tim@example.org", Valid: true}, Password: "hash"},
			{Model: gorm.Model{ID: 8}, Name: "Anja", Email: sql.NullString{String: "anja@example.org", Valid: true}},
			{Model: gorm.Model{ID: 9}, Name: "No address"},
		}
		return nil
	})

	resp, err := api.ListCourseParticipants(asCaller(courseLecturer), &protobuf.ListCourseParticipantsRequest{CourseId: 5})
	if err != nil {
		t.Fatalf("ListCourseParticipants: %v", err)
	}
	if len(resp.Participants) != 2 || !resp.Participants[0].AccountSetUp || resp.Participants[1].AccountSetUp {
		t.Errorf("participants = %v", resp.Participants)
	}
}

func TestInviteCourseParticipants(t *testing.T) {
	tools.Cfg.Mail = tools.MailConfig{Sender: "from@invalid", Server: "server", MaxMailsPerMinute: 1}

	t.Run("creates and invites a new address, enrols and notifies a known one", func(t *testing.T) {
		api, m := courseAdminAPI(t, adminCourse)
		known := model.User{Model: gorm.Model{ID: 8}, Email: sql.NullString{String: "anja@example.org", Valid: true}}

		m.users.EXPECT().GetUserByEmail(gomock.Any(), "anja@example.org").Return(known, nil)
		m.courses.EXPECT().AddUserToCourse(gomock.Any(), uint(8), uint(5)).Return(nil)
		m.emails.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, mail *model.Email) error {
			if mail.To != "anja@example.org" || !strings.Contains(mail.Body, `"Brauereiwesen"`) {
				t.Errorf("notification = %+v", mail)
			}
			return nil
		})

		// The new address: not found, created, enrolled, then sent the invitation,
		// which looks the account up again by address.
		newUser := model.User{Model: gorm.Model{ID: 9}, Email: sql.NullString{String: "tim@example.org", Valid: true}}
		gomock.InOrder(
			m.users.EXPECT().GetUserByEmail(gomock.Any(), "tim@example.org").Return(model.User{}, gorm.ErrRecordNotFound),
			m.users.EXPECT().CreateUser(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, u *model.User) error {
				if u.Name != "Tim" || u.Role != model.GenericType || u.Password != "" {
					t.Errorf("created %+v", u)
				}
				u.ID = 9
				return nil
			}),
			m.courses.EXPECT().AddUserToCourse(gomock.Any(), uint(9), uint(5)).Return(nil),
			m.users.EXPECT().GetUserByEmail(gomock.Any(), "tim@example.org").Return(newUser, nil),
		)
		m.users.EXPECT().CreateRegisterLink(gomock.Any(), newUser).Return(model.RegisterLink{RegisterSecret: "abc"}, nil)
		m.emails.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, mail *model.Email) error {
			if mail.To != "tim@example.org" || !strings.Contains(mail.Body, "/setPassword/abc") {
				t.Errorf("invitation = %+v", mail)
			}
			return nil
		})

		resp, err := api.InviteCourseParticipants(asCaller(courseLecturer), &protobuf.InviteCourseParticipantsRequest{
			CourseId: 5,
			Invitees: []*protobuf.CourseInvitee{
				{Name: " Anja ", Email: "anja@example.org"},
				{Name: "Tim", Email: " tim@example.org "},
			},
		})
		if err != nil {
			t.Fatalf("InviteCourseParticipants: %v", err)
		}
		if len(resp.Results) != 2 || resp.Results[0].AccountCreated || !resp.Results[1].AccountCreated {
			t.Fatalf("results = %v", resp.Results)
		}
		for _, r := range resp.Results {
			if r.Error != "" {
				t.Errorf("%s: %s", r.Email, r.Error)
			}
		}
	})

	t.Run("does not read a failed lookup as a new address", func(t *testing.T) {
		api, m := courseAdminAPI(t, adminCourse)
		m.users.EXPECT().GetUserByEmail(gomock.Any(), "tim@example.org").Return(model.User{}, errors.New("connection reset"))
		// No CreateUser expected.

		resp, err := api.InviteCourseParticipants(asCaller(courseLecturer), &protobuf.InviteCourseParticipantsRequest{
			CourseId: 5, Invitees: []*protobuf.CourseInvitee{{Name: "Tim", Email: "tim@example.org"}},
		})
		if err != nil {
			t.Fatalf("InviteCourseParticipants: %v", err)
		}
		if resp.Results[0].Error == "" {
			t.Error("the failed lookup was not reported")
		}
	})

	for name, invitees := range map[string][]*protobuf.CourseInvitee{
		"nobody":              nil,
		"a missing name":      {{Name: " ", Email: "tim@example.org"}},
		"an invalid address":  {{Name: "Tim", Email: "tim at example.org"}},
		"a display-name form": {{Name: "Tim", Email: "Tim <tim@example.org>"}},
		"one bad entry of two": {
			{Name: "Anja", Email: "anja@example.org"},
			{Name: "Tim", Email: "tim@"},
		},
	} {
		t.Run("rejects "+name+" before inviting anyone", func(t *testing.T) {
			api, _ := courseAdminAPI(t, adminCourse)
			_, err := api.InviteCourseParticipants(asCaller(courseLecturer), &protobuf.InviteCourseParticipantsRequest{CourseId: 5, Invitees: invitees})
			wantCode(t, err, codes.InvalidArgument)
		})
	}
}

func TestListAdministeredCourses(t *testing.T) {
	api, m := courseAdminAPI(t, adminCourse)
	m.courses.EXPECT().GetAdministeredCoursesByUserId(gomock.Any(), uint(2), "", 0).Return([]model.Course{
		{Model: gorm.Model{ID: 3}, Name: "Golang", Slug: "godev", Year: 2021, TeachingTerm: "W"},
		{Model: gorm.Model{ID: 1}, Name: "Brauereiwesen", Slug: "brau", Year: 2022, TeachingTerm: "S"},
		{Model: gorm.Model{ID: 6}, Name: "Bierkunde", Slug: "bier", Year: 2022, TeachingTerm: "W"},
	}, nil)

	resp, err := api.ListAdministeredCourses(asCaller(courseLecturer), &emptypb.Empty{})
	if err != nil {
		t.Fatalf("ListAdministeredCourses: %v", err)
	}
	var ids []uint32
	for _, c := range resp.Courses {
		ids = append(ids, c.Id)
	}
	// Newest semester first: winter 2022 starts after summer 2022.
	if len(ids) != 3 || ids[0] != 6 || ids[1] != 1 || ids[2] != 3 {
		t.Errorf("order = %v, want [6 1 3]", ids)
	}
}
