package apiv2

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"

	e "github.com/TUM-Dev/gocast/apiv2/errors"
	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
)

// integrationUploadPrefix is the path a ticket from createIntegrationUpload is used on.
const integrationUploadPrefix = "/api/v2/integration/uploads/"

// integrationUploadTicketTTL is how long a ticket waits for its upload to start. A
// ticket is handed to the integration's browser page, which starts at once.
const integrationUploadTicketTTL = 10 * time.Minute

var externalRefPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{16,64}$`)

// CreateIntegrationUpload creates the lecture an integration's upload goes into, start
// now, without a lecture hall, like a VOD upload created in the course settings. One
// reference is one lecture with one upload; a repeated call for a reference whose
// upload has not started (the integration lost the answer) returns the same lecture and
// a new ticket, which invalidates the previous one.
func (a *API) CreateIntegrationUpload(ctx context.Context, req *protobuf.CreateIntegrationUploadRequest) (*protobuf.CreateIntegrationUploadResponse, error) {
	if req.GetGrantId() == 0 {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("grant_id must be positive"))
	}
	if !externalRefPattern.MatchString(req.GetExternalRef()) {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("external_ref must be 16 to 64 characters of A-Z, a-z, 0-9, - and _"))
	}
	title := strings.TrimSpace(req.GetTitle())
	if title == "" || utf8.RuneCountInString(title) > 255 {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("title must have 1 to 255 characters"))
	}
	integration, err := a.getCurrentIntegration(ctx)
	if err != nil {
		return nil, err
	}
	ticket := make([]byte, 32)
	if _, err := rand.Read(ticket); err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, errors.New("could not create upload"))
	}
	hash := sha256.Sum256(ticket)
	now := time.Now()
	lecture := model.Stream{Name: title, Start: now, End: now.Add(placeholderDuration)}
	stream, created, err := a.dao.IntegrationGrantDao.CreateIntegrationUpload(ctx, integration.ID, uint(req.GetGrantId()), req.GetExternalRef(), lecture, hash[:], now.Add(integrationUploadTicketTTL))
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return nil, e.WithStatus(http.StatusNotFound, errors.New("grant not found"))
	case errors.Is(err, dao.ErrIntegrationUploadRefused):
		return nil, e.WithStatus(http.StatusConflict, err)
	case err != nil:
		a.log.Error("integration upload creation failed", "err", err)
		return nil, e.WithStatus(http.StatusInternalServerError, errors.New("could not create upload"))
	}
	if created {
		// No user acted; the integration is named in the message instead.
		message := fmt.Sprintf("Stream '%s' (%d) created by integration '%s' (%d) for an upload. Time: %s", title, stream.ID, integration.Name, integration.ID, now.Format("2006 02 Jan, 15:04"))
		entry := model.Audit{Message: message, Type: model.AuditStreamCreate}
		forStream(stream)(&entry)
		if err := a.dao.AuditDao.Create(&entry); err != nil {
			a.log.Error("can't write audit entry", "err", err, "message", message)
		}
	}
	return &protobuf.CreateIntegrationUploadResponse{
		StreamId:   uint32(stream.ID),
		UploadPath: integrationUploadPrefix + base64.RawURLEncoding.EncodeToString(ticket),
	}, nil
}
