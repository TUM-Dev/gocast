package apiv2

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/netip"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"golang.org/x/net/idna"
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

// integrationUploadRouteOf matches the upload a ticket stands for, and the preflight
// of a browser that sends it.
func integrationUploadRouteOf(r *http.Request) (string, bool) {
	if r.Method != http.MethodPost && r.Method != http.MethodOptions {
		return "", false
	}
	return strings.CutPrefix(r.URL.Path, integrationUploadPrefix)
}

// handleIntegrationUpload takes the recording for a lecture an integration created. It
// needs no login: only the integration, with its API key and an active grant, can get a
// ticket, and it hands the ticket to the browser of one of its users. CORS is allowed for
// the origin of the integration's registered return URL only, since a browser refuses an
// upload with progress from another origin otherwise; the ticket is the protection. The
// ticket is used up before the body is read, and the recording is always the combined
// version.
func (a *API) handleIntegrationUpload(c *gin.Context, rawTicket string) {
	notFound := e.WithStatus(http.StatusNotFound, errors.New("upload not found"))
	ticket, err := base64.RawURLEncoding.DecodeString(rawTicket)
	if err != nil || len(ticket) != 32 || base64.RawURLEncoding.EncodeToString(ticket) != rawTicket {
		writeUploadError(c, notFound)
		return
	}
	hash := sha256.Sum256(ticket)
	ctx, now := c.Request.Context(), time.Now()

	origin := c.GetHeader("Origin")
	if c.Request.Method == http.MethodOptions || origin != "" {
		returnURL, err := a.dao.IntegrationGrantDao.GetIntegrationUploadReturnURL(ctx, hash[:], now)
		if err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				a.log.Error("integration upload lookup failed", "err", err)
			}
			writeUploadError(c, notFound)
			return
		}
		if origin == "" || origin != originOf(returnURL) {
			writeUploadError(c, e.WithStatus(http.StatusForbidden, errors.New("uploads are only accepted from the integration's origin")))
			return
		}
		header := c.Writer.Header()
		header.Set("Access-Control-Allow-Origin", origin)
		header.Add("Vary", "Origin")
		if c.Request.Method == http.MethodOptions {
			header.Set("Access-Control-Allow-Methods", "POST")
			header.Set("Access-Control-Allow-Headers", "Content-Type")
			header.Set("Access-Control-Max-Age", "600")
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
	}

	stream, err := a.dao.IntegrationGrantDao.ConsumeIntegrationUploadTicket(ctx, hash[:], now)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			a.log.Error("integration upload ticket failed", "err", err)
		}
		writeUploadError(c, notFound)
		return
	}
	a.proxyLectureMedia(c, stream, model.VideoTypeCombined)
}

// originOf is the origin a browser sends for pages under rawURL: the host in its
// canonical form (ASCII for an internationalized name, the short form of an IPv6
// address) and without the scheme's default port, which browsers leave out.
func originOf(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Hostname() == "" {
		return ""
	}
	scheme, host := strings.ToLower(u.Scheme), u.Hostname()
	if addr, err := netip.ParseAddr(host); err == nil {
		host = addr.WithZone("").String()
		if addr.Is6() {
			host = "[" + host + "]"
		}
	} else if host, err = idna.Lookup.ToASCII(host); err != nil {
		return ""
	}
	if port := u.Port(); port != "" && !(scheme == "https" && port == "443") && !(scheme == "http" && port == "80") {
		host += ":" + port
	}
	return scheme + "://" + host
}
