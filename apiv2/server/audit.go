package apiv2

import (
	"context"
	"errors"
	"net/http"

	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"

	e "github.com/TUM-Dev/gocast/apiv2/errors"
	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/model"
)

// defaultAuditPageSize matches the page size the old admin page used, so a client
// that omits the limit still gets a sensible page rather than nothing (limit 0 is a
// valid SQL LIMIT and returns zero rows).
const defaultAuditPageSize = 10

// ListAudits returns one page of the server-wide audit log, newest first. Gated on
// PermAdministerServer by its policy in services.go.
func (a *API) ListAudits(ctx context.Context, req *protobuf.ListAuditsRequest) (*protobuf.ListAuditsResponse, error) {
	return a.auditPage(req.GetType(), req.GetLimit(), req.GetOffset(),
		func(limit, offset int, types ...model.AuditType) ([]model.Audit, error) {
			return a.dao.AuditDao.Find(limit, offset, types...)
		},
		func(types ...model.AuditType) (int64, error) { return a.dao.AuditDao.Count(types...) })
}

// ListCourseAudits returns one page of the audit log entries about a course and its
// lectures, newest first. Gated on administering the course by its policy in
// services.go: the log names who did what, which the course's administrators may see
// of their own course and no other.
func (a *API) ListCourseAudits(ctx context.Context, req *protobuf.ListCourseAuditsRequest) (*protobuf.ListAuditsResponse, error) {
	courseID := uint(req.GetCourseId())
	return a.auditPage(req.GetType(), req.GetLimit(), req.GetOffset(),
		func(limit, offset int, types ...model.AuditType) ([]model.Audit, error) {
			return a.dao.AuditDao.FindByCourse(courseID, limit, offset, types...)
		},
		func(types ...model.AuditType) (int64, error) { return a.dao.AuditDao.CountByCourse(courseID, types...) })
}

// auditPage is what ListAudits and ListCourseAudits share: the page defaults, the type
// filter and the entry mapping. Only where the rows come from differs.
func (a *API) auditPage(
	typeName string,
	limit32, offset32 uint32,
	find func(limit, offset int, types ...model.AuditType) ([]model.Audit, error),
	count func(types ...model.AuditType) (int64, error),
) (*protobuf.ListAuditsResponse, error) {
	limit := int(limit32)
	if limit <= 0 {
		limit = defaultAuditPageSize
	}

	types := model.GetAllAuditTypes()
	if typeName != "" {
		typ, ok := model.AuditTypeByName(typeName)
		if !ok {
			return nil, e.WithStatus(http.StatusBadRequest, errors.New("unknown audit type"))
		}
		types = []model.AuditType{typ}
	}

	found, err := find(limit, int(offset32), types...)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	total, err := count(types...)
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	out := make([]*protobuf.AuditEntry, 0, len(found))
	for _, audit := range found {
		out = append(out, auditMessage(audit))
	}

	return &protobuf.ListAuditsResponse{Audits: out, Total: uint32(total)}, nil
}

func auditMessage(audit model.Audit) *protobuf.AuditEntry {
	var userID uint32
	if audit.UserID != nil {
		userID = uint32(*audit.UserID)
	}

	return &protobuf.AuditEntry{
		Id:        uint32(audit.ID),
		CreatedAt: timestamppb.New(audit.CreatedAt),
		Type:      audit.Type.String(),
		Message:   audit.Message,
		UserId:    userID,
		// GetLoginString has a nil receiver guard, so a system audit's nil User needs
		// no special case here.
		UserName: audit.User.GetLoginString(),
	}
}
