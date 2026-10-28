package apiv2

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"go.uber.org/mock/gomock"
	"gorm.io/gorm"

	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
)

// auditCapture records every entry written, for tests of what a handler audits.
func auditCapture(t *testing.T) (dao.AuditDao, *[]model.Audit) {
	t.Helper()
	var got []model.Audit
	m := mock_dao.NewMockAuditDao(gomock.NewController(t))
	m.EXPECT().Create(gomock.Any()).DoAndReturn(func(a *model.Audit) error {
		got = append(got, *a)
		return nil
	}).AnyTimes()
	return m, &got
}

func TestCreateTokenAuditsWithoutTheSecret(t *testing.T) {
	admin := user(1, "Anja Admin", "anja@example.org", model.AdminType)
	tokens := mock_dao.NewMockTokenDao(gomock.NewController(t))
	tokens.EXPECT().AddToken(gomock.Any()).Return(nil)
	audits, got := auditCapture(t)
	api := &API{dao: dao.DaoWrapper{AuditDao: audits, TokenDao: tokens}, log: slog.Default()}

	resp, err := api.CreateToken(ctxAs(admin), &protobuf.CreateTokenRequest{Scope: model.TokenScopeLecturer})
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	if len(*got) != 1 || (*got)[0].Type != model.AuditTokenCreate {
		t.Fatalf("audits = %+v, want one token creation", *got)
	}
	if strings.Contains((*got)[0].Message, resp.Token) {
		t.Error("the audit entry carries the token's secret")
	}
	if (*got)[0].User == nil || (*got)[0].User.ID != admin.ID {
		t.Errorf("audit user = %+v, want the caller", (*got)[0].User)
	}
}

func TestDeleteUserAuditsTheNameAndOnlyOnSuccess(t *testing.T) {
	admin := user(1, "Anja Admin", "anja@example.org", model.AdminType)
	target := user(2, "Tim Student", "tim@example.org", model.LecturerType)

	for name, deleteErr := range map[string]error{"deleted": nil, "failed": errors.New("db down")} {
		t.Run(name, func(t *testing.T) {
			users := mock_dao.NewMockUsersDao(gomock.NewController(t))
			users.EXPECT().IsUserAdmin(gomock.Any(), uint(2)).Return(false, nil)
			users.EXPECT().GetUserByID(gomock.Any(), uint(2)).Return(target, nil)
			users.EXPECT().DeleteUser(gomock.Any(), uint(2)).Return(deleteErr)
			audits, got := auditCapture(t)
			api := &API{dao: dao.DaoWrapper{AuditDao: audits, UsersDao: users}, log: slog.Default()}

			_, err := api.DeleteUser(ctxAs(admin), &protobuf.DeleteUserRequest{UserId: 2})

			if deleteErr != nil {
				if err == nil || len(*got) != 0 {
					t.Fatalf("err = %v, audits = %+v: a failed delete must not be audited", err, *got)
				}
				return
			}
			if err != nil || len(*got) != 1 || (*got)[0].Type != model.AuditUserDelete {
				t.Fatalf("err = %v, audits = %+v, want one user deletion", err, *got)
			}
			if !strings.Contains((*got)[0].Message, "tim@example.org") {
				t.Errorf("message %q does not name the deleted account", (*got)[0].Message)
			}
		})
	}
}

func TestUpdateUserRoleAuditsTheChange(t *testing.T) {
	admin := user(1, "Anja Admin", "anja@example.org", model.AdminType)
	target := user(2, "Tim Student", "tim@example.org", model.StudentType)
	users := mock_dao.NewMockUsersDao(gomock.NewController(t))
	users.EXPECT().GetUserByID(gomock.Any(), uint(2)).Return(target, nil)
	users.EXPECT().UpdateUser(gomock.Any()).Return(nil)
	audits, got := auditCapture(t)
	api := &API{dao: dao.DaoWrapper{AuditDao: audits, UsersDao: users}, log: slog.Default()}

	if _, err := api.UpdateUserRole(ctxAs(admin), &protobuf.UpdateUserRoleRequest{UserId: 2, Role: model.LecturerType}); err != nil {
		t.Fatalf("UpdateUserRole: %v", err)
	}

	if len(*got) != 1 || (*got)[0].Type != model.AuditUserEdit {
		t.Fatalf("audits = %+v, want one user edit", *got)
	}
	if want := "role: 4 -> 2"; !strings.Contains((*got)[0].Message, want) {
		t.Errorf("message %q does not contain %q", (*got)[0].Message, want)
	}
}

func TestDeleteRunnerAuditsOnlyOnSuccess(t *testing.T) {
	for name, deleteErr := range map[string]error{"deleted": nil, "failed": errors.New("db down")} {
		t.Run(name, func(t *testing.T) {
			runners := mock_dao.NewMockRunnerDao(gomock.NewController(t))
			runners.EXPECT().Delete(gomock.Any(), "runner-1").Return(deleteErr)
			audits, got := auditCapture(t)
			api := &API{dao: dao.DaoWrapper{AuditDao: audits, RunnerDao: runners}, log: slog.Default()}

			_, err := api.DeleteRunner(context.Background(), &protobuf.DeleteRunnerRequest{Hostname: "runner-1"})

			if (err == nil) != (len(*got) == 1) {
				t.Fatalf("err = %v, audits = %+v", err, *got)
			}
		})
	}
}

// An entry with no user reads as a system action, so a failed lookup of the caller
// must at least be loud; the entry itself is still written.
func TestAuditWithoutAUserIsStillWritten(t *testing.T) {
	audits, got := auditCapture(t)
	api := &API{dao: dao.DaoWrapper{AuditDao: audits}, log: slog.Default()}

	api.audit(context.Background(), model.AuditRunner, "x")

	if len(*got) != 1 || (*got)[0].User != nil {
		t.Fatalf("audits = %+v, want one entry without a user", *got)
	}
}

func TestDeleteUserStillAuditsWhenTheLookupFails(t *testing.T) {
	admin := user(1, "Anja Admin", "anja@example.org", model.AdminType)
	users := mock_dao.NewMockUsersDao(gomock.NewController(t))
	users.EXPECT().IsUserAdmin(gomock.Any(), uint(2)).Return(false, nil)
	users.EXPECT().GetUserByID(gomock.Any(), uint(2)).Return(model.User{}, errors.New("blip"))
	users.EXPECT().DeleteUser(gomock.Any(), uint(2)).Return(nil)
	audits, got := auditCapture(t)
	api := &API{dao: dao.DaoWrapper{AuditDao: audits, UsersDao: users}, log: slog.Default()}

	if _, err := api.DeleteUser(ctxAs(admin), &protobuf.DeleteUserRequest{UserId: 2}); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}
	// The id alone, not " (2)" built from an empty user.
	if len(*got) != 1 || (*got)[0].Message != "2" {
		t.Errorf("audits = %+v, want a message naming only the id", *got)
	}
}

func TestListCourseAuditsReadsOnlyThatCourse(t *testing.T) {
	audits := mock_dao.NewMockAuditDao(gomock.NewController(t))
	audits.EXPECT().FindByCourse(uint(5), defaultAuditPageSize, 20, gomock.Any()).
		Return([]model.Audit{{Message: "7: ended", Type: model.AuditStreamEdit}}, nil)
	audits.EXPECT().CountByCourse(uint(5), gomock.Any()).Return(int64(31), nil)
	api := &API{dao: dao.DaoWrapper{AuditDao: audits}, log: slog.Default()}

	resp, err := api.ListCourseAudits(context.Background(), &protobuf.ListCourseAuditsRequest{CourseId: 5, Offset: 20})
	if err != nil {
		t.Fatalf("ListCourseAudits: %v", err)
	}
	if len(resp.Audits) != 1 || resp.Total != 31 {
		t.Errorf("audits = %d, total = %d, want 1 and 31", len(resp.Audits), resp.Total)
	}
}

func TestListCourseAuditsFiltersAndRefusesAnUnknownType(t *testing.T) {
	audits := mock_dao.NewMockAuditDao(gomock.NewController(t))
	audits.EXPECT().FindByCourse(uint(5), defaultAuditPageSize, 0, model.AuditStreamEdit).Return(nil, nil)
	audits.EXPECT().CountByCourse(uint(5), model.AuditStreamEdit).Return(int64(0), nil)
	api := &API{dao: dao.DaoWrapper{AuditDao: audits}, log: slog.Default()}

	if _, err := api.ListCourseAudits(context.Background(), &protobuf.ListCourseAuditsRequest{CourseId: 5, Type: "Stream Edited"}); err != nil {
		t.Fatalf("ListCourseAudits: %v", err)
	}
	if _, err := api.ListCourseAudits(context.Background(), &protobuf.ListCourseAuditsRequest{CourseId: 5, Type: "Nope"}); err == nil {
		t.Fatal("an unknown type was accepted")
	}
}

// What puts an entry on a course's log is the ids it is written with.
func TestAuditScopesLinkTheEntryToItsCourseAndLecture(t *testing.T) {
	audits, got := auditCapture(t)
	api := &API{dao: dao.DaoWrapper{AuditDao: audits}, log: slog.Default()}
	stream := model.Stream{Model: gorm.Model{ID: 7}, CourseID: 5}

	api.audit(context.Background(), model.AuditCourseEdit, "course", forCourse(5))
	api.audit(context.Background(), model.AuditStreamEdit, "stream", forStream(stream))
	api.audit(context.Background(), model.AuditRunner, "server-wide")

	e := *got
	if len(e) != 3 {
		t.Fatalf("entries = %d, want 3", len(e))
	}
	if e[0].CourseID == nil || *e[0].CourseID != 5 || e[0].StreamID != nil {
		t.Errorf("course entry = %+v, want course 5 and no stream", e[0])
	}
	if e[1].CourseID == nil || *e[1].CourseID != 5 || e[1].StreamID == nil || *e[1].StreamID != 7 {
		t.Errorf("stream entry = %+v, want course 5 and stream 7", e[1])
	}
	if e[2].CourseID != nil || e[2].StreamID != nil {
		t.Errorf("server-wide entry = %+v, want neither", e[2])
	}
}
