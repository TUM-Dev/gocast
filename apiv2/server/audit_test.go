package apiv2

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"strings"
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
	auditMock.EXPECT().Count(gomock.Any()).Return(int64(0), nil).Times(1)

	api := &API{dao: dao.DaoWrapper{AuditDao: auditMock}, log: slog.Default()}

	if _, err := api.ListAudits(context.Background(), &protobuf.ListAuditsRequest{}); err != nil {
		t.Fatalf("ListAudits: %v", err)
	}
}

func TestListAuditsPassesThroughLimitAndOffset(t *testing.T) {
	ctrl := gomock.NewController(t)
	auditMock := mock_dao.NewMockAuditDao(ctrl)
	auditMock.EXPECT().Find(5, 20, gomock.Any()).Return([]model.Audit{}, nil).Times(1)
	auditMock.EXPECT().Count(gomock.Any()).Return(int64(0), nil).Times(1)

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
	auditMock.EXPECT().Count(gomock.Any()).Return(int64(0), nil).Times(1)

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
	auditMock.EXPECT().Count(gomock.Any()).Return(int64(0), nil).Times(1)

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
	if model.AuditImpersonation.String() != "Impersonation" {
		t.Errorf("impersonation label = %q", model.AuditImpersonation.String())
	}
	if entry.Type != "Camera Moved" {
		t.Errorf("type = %q, want %q", entry.Type, "Camera Moved")
	}
}

func TestListAuditsWithNoneFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	auditMock := mock_dao.NewMockAuditDao(ctrl)
	auditMock.EXPECT().Find(gomock.Any(), gomock.Any(), gomock.Any()).Return([]model.Audit{}, nil).Times(1)
	auditMock.EXPECT().Count(gomock.Any()).Return(int64(0), nil).Times(1)

	api := &API{dao: dao.DaoWrapper{AuditDao: auditMock}, log: slog.Default()}

	resp, err := api.ListAudits(context.Background(), &protobuf.ListAuditsRequest{})
	if err != nil {
		t.Fatalf("ListAudits: %v", err)
	}
	if len(resp.Audits) != 0 {
		t.Errorf("got %d audits, want none", len(resp.Audits))
	}
}

func TestListAuditsFiltersByTypeAndReportsTheTotal(t *testing.T) {
	ctrl := gomock.NewController(t)
	auditMock := mock_dao.NewMockAuditDao(ctrl)
	auditMock.EXPECT().Find(10, 0, model.AuditUserDelete).Return([]model.Audit{}, nil).Times(1)
	auditMock.EXPECT().Count(model.AuditUserDelete).Return(int64(42), nil).Times(1)

	api := &API{dao: dao.DaoWrapper{AuditDao: auditMock}, log: slog.Default()}

	resp, err := api.ListAudits(context.Background(), &protobuf.ListAuditsRequest{Type: "User Deleted"})
	if err != nil {
		t.Fatalf("ListAudits: %v", err)
	}
	if resp.Total != 42 {
		t.Errorf("total = %d, want 42", resp.Total)
	}
}

func TestListAuditsRejectsAnUnknownType(t *testing.T) {
	api := &API{dao: dao.DaoWrapper{AuditDao: mock_dao.NewMockAuditDao(gomock.NewController(t))}, log: slog.Default()}

	if _, err := api.ListAudits(context.Background(), &protobuf.ListAuditsRequest{Type: "Nope"}); err == nil {
		t.Fatal("want an error for an unknown type")
	}
}

func TestAuditTypeNamesRoundTrip(t *testing.T) {
	for _, typ := range model.GetAllAuditTypes() {
		got, ok := model.AuditTypeByName(typ.String())
		if !ok || got != typ {
			t.Errorf("%d: %q does not round-trip", typ, typ.String())
		}
	}
	// A row written by a newer version must not panic the page.
	if got := model.AuditType(999).String(); got != "Unknown" {
		t.Errorf("out-of-range type = %q, want Unknown", got)
	}
}

// auditSink accepts any number of audit entries, for tests of handlers that audit
// but are not about the audit. Tests of what is audited expect Create themselves.
func auditSink(t *testing.T) dao.AuditDao {
	t.Helper()
	sink := mock_dao.NewMockAuditDao(gomock.NewController(t))
	sink.EXPECT().Create(gomock.Any()).Return(nil).AnyTimes()
	return sink
}

// The frontend's filter offers a copy of the type labels (AUDIT_TYPES), since the API
// answers the label rather than a code. A type added here and not there could be seen
// in the log but never filtered for.
func TestFrontendOffersEveryAuditType(t *testing.T) {
	src, err := os.ReadFile("../../frontend/src/lib/audits.ts")
	if err != nil {
		t.Skipf("frontend sources not available: %v", err)
	}
	for _, typ := range model.GetAllAuditTypes() {
		if !strings.Contains(string(src), `"`+typ.String()+`"`) {
			t.Errorf("frontend/src/lib/audits.ts AUDIT_TYPES lacks %q", typ.String())
		}
	}
}
