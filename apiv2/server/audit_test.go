package apiv2

import (
	"context"
	"database/sql"
	"log/slog"
	"testing"

	"go.uber.org/mock/gomock"
	"gorm.io/gorm"

	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
)

func TestListAuditsDefaultsTheLimitWhenOmitted(t *testing.T) {
	// limit 0 is a valid SQL LIMIT and returns zero rows, so an omitted limit must
	// not reach the database as-is.
	ctrl := gomock.NewController(t)
	auditMock := mock_dao.NewMockAuditDao(ctrl)
	auditMock.EXPECT().Find(defaultAuditPageSize, 0, gomock.Any()).Return([]model.Audit{}, nil).Times(1)

	api := &API{dao: dao.DaoWrapper{AuditDao: auditMock}, log: slog.Default()}

	if _, err := api.ListAudits(context.Background(), &protobuf.ListAuditsRequest{}); err != nil {
		t.Fatalf("ListAudits: %v", err)
	}
}

func TestListAuditsPassesThroughLimitAndOffset(t *testing.T) {
	ctrl := gomock.NewController(t)
	auditMock := mock_dao.NewMockAuditDao(ctrl)
	auditMock.EXPECT().Find(5, 20, gomock.Any()).Return([]model.Audit{}, nil).Times(1)

	api := &API{dao: dao.DaoWrapper{AuditDao: auditMock}, log: slog.Default()}

	if _, err := api.ListAudits(context.Background(), &protobuf.ListAuditsRequest{Limit: 5, Offset: 20}); err != nil {
		t.Fatalf("ListAudits: %v", err)
	}
}

func TestListAuditsMapsASystemAuditWithoutAUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	auditMock := mock_dao.NewMockAuditDao(ctrl)
	auditMock.EXPECT().Find(gomock.Any(), gomock.Any(), gomock.Any()).Return([]model.Audit{
		{Message: "server started", Type: model.AuditInfo, User: nil, UserID: nil},
	}, nil).Times(1)

	api := &API{dao: dao.DaoWrapper{AuditDao: auditMock}, log: slog.Default()}

	resp, err := api.ListAudits(context.Background(), &protobuf.ListAuditsRequest{})
	if err != nil {
		t.Fatalf("ListAudits: %v", err)
	}
	if len(resp.Audits) != 1 {
		t.Fatalf("got %d audits, want 1", len(resp.Audits))
	}

	entry := resp.Audits[0]
	if entry.UserId != 0 {
		t.Errorf("userId = %d, want 0 for a system audit", entry.UserId)
	}
	if entry.UserName != "- System -" {
		t.Errorf("userName = %q, want %q", entry.UserName, "- System -")
	}
	if entry.Type != "Info" {
		t.Errorf("type = %q, want %q", entry.Type, "Info")
	}
}

func TestListAuditsMapsAnAuditWithAUser(t *testing.T) {
	userID := uint(1)
	ctrl := gomock.NewController(t)
	auditMock := mock_dao.NewMockAuditDao(ctrl)
	auditMock.EXPECT().Find(gomock.Any(), gomock.Any(), gomock.Any()).Return([]model.Audit{
		{
			Message: "camera moved",
			Type:    model.AuditCameraMoved,
			UserID:  &userID,
			User: &model.User{
				Model: gorm.Model{ID: userID},
				Email: sql.NullString{String: "admin@example.com", Valid: true},
			},
		},
	}, nil).Times(1)

	api := &API{dao: dao.DaoWrapper{AuditDao: auditMock}, log: slog.Default()}

	resp, err := api.ListAudits(context.Background(), &protobuf.ListAuditsRequest{})
	if err != nil {
		t.Fatalf("ListAudits: %v", err)
	}
	if len(resp.Audits) != 1 {
		t.Fatalf("got %d audits, want 1", len(resp.Audits))
	}

	entry := resp.Audits[0]
	if entry.UserId != uint32(userID) {
		t.Errorf("userId = %d, want %d", entry.UserId, userID)
	}
	if entry.UserName != "admin@example.com" {
		t.Errorf("userName = %q, want %q", entry.UserName, "admin@example.com")
	}
	if entry.Type != "Camera Moved" {
		t.Errorf("type = %q, want %q", entry.Type, "Camera Moved")
	}
}

func TestListAuditsWithNoneFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	auditMock := mock_dao.NewMockAuditDao(ctrl)
	auditMock.EXPECT().Find(gomock.Any(), gomock.Any(), gomock.Any()).Return([]model.Audit{}, nil).Times(1)

	api := &API{dao: dao.DaoWrapper{AuditDao: auditMock}, log: slog.Default()}

	resp, err := api.ListAudits(context.Background(), &protobuf.ListAuditsRequest{})
	if err != nil {
		t.Fatalf("ListAudits: %v", err)
	}
	if len(resp.Audits) != 0 {
		t.Errorf("got %d audits, want none", len(resp.Audits))
	}
}
