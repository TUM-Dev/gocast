package apiv2

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"go.uber.org/mock/gomock"

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
