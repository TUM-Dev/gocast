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
	limit := int(req.GetLimit())
	if limit <= 0 {
		limit = defaultAuditPageSize
	}

	found, err := a.dao.AuditDao.Find(limit, int(req.GetOffset()), model.GetAllAuditTypes()...)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	out := make([]*protobuf.AuditEntry, 0, len(found))
	for _, audit := range found {
		out = append(out, auditMessage(audit))
	}

	return &protobuf.ListAuditsResponse{Audits: out}, nil
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
