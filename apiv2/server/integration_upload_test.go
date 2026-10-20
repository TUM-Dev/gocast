package apiv2

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"

	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
)

const testUploadRef = "artemis-unit-7-attempt-1"

func integrationCaller() context.Context {
	return context.WithValue(context.Background(), callerKey{}, &caller{integration: &model.Integration{ID: 7, Name: "Artemis"}})
}

func TestCreateIntegrationUploadHandsOutATicketWhoseHashIsStored(t *testing.T) {
	ctrl := gomock.NewController(t)
	grants := mock_dao.NewMockIntegrationGrantDao(ctrl)
	audits := mock_dao.NewMockAuditDao(ctrl)
	var storedHash []byte
	before := time.Now()
	grants.EXPECT().CreateIntegrationUpload(gomock.Any(), uint(7), uint(123), testUploadRef, gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _, _ uint, _ string, lecture model.Stream, hash []byte, expiresAt time.Time) (model.Stream, bool, error) {
			assert.Equal(t, "Lecture 3", lecture.Name)
			assert.WithinDuration(t, before, lecture.Start, time.Minute)
			assert.Equal(t, time.Hour, lecture.End.Sub(lecture.Start))
			assert.WithinDuration(t, before.Add(10*time.Minute), expiresAt, time.Minute)
			storedHash = hash
			lecture.ID, lecture.CourseID = 55, 456
			return lecture, true, nil
		})
	audits.EXPECT().Create(gomock.Any()).DoAndReturn(func(entry *model.Audit) error {
		assert.Nil(t, entry.User)
		assert.Equal(t, model.AuditStreamCreate, entry.Type)
		assert.Contains(t, entry.Message, "'Artemis' (7)")
		assert.Equal(t, uint(456), *entry.CourseID)
		assert.Equal(t, uint(55), *entry.StreamID)
		return nil
	})
	api := &API{dao: dao.DaoWrapper{IntegrationGrantDao: grants, AuditDao: audits}, log: slog.Default()}

	response, err := api.CreateIntegrationUpload(integrationCaller(), &protobuf.CreateIntegrationUploadRequest{GrantId: 123, ExternalRef: testUploadRef, Title: "  Lecture 3 "})

	require.NoError(t, err)
	assert.Equal(t, uint32(55), response.StreamId)
	raw, ok := strings.CutPrefix(response.UploadPath, "/api/v2/integration/uploads/")
	require.True(t, ok)
	ticket, err := base64.RawURLEncoding.DecodeString(raw)
	require.NoError(t, err)
	require.Len(t, ticket, 32)
	hash := sha256.Sum256(ticket)
	assert.Equal(t, hash[:], storedHash, "only the ticket's hash is stored")
}

func TestCreateIntegrationUploadRenewsTheTicketOfAKnownReferenceWithoutAudit(t *testing.T) {
	ctrl := gomock.NewController(t)
	grants := mock_dao.NewMockIntegrationGrantDao(ctrl)
	grants.EXPECT().CreateIntegrationUpload(gomock.Any(), uint(7), uint(123), testUploadRef, gomock.Any(), gomock.Any(), gomock.Any()).
		Return(model.Stream{Model: gorm.Model{ID: 55}}, false, nil)
	api := &API{dao: dao.DaoWrapper{IntegrationGrantDao: grants, AuditDao: mock_dao.NewMockAuditDao(ctrl)}, log: slog.Default()}

	response, err := api.CreateIntegrationUpload(integrationCaller(), &protobuf.CreateIntegrationUploadRequest{GrantId: 123, ExternalRef: testUploadRef, Title: "Lecture 3"})

	require.NoError(t, err)
	assert.Equal(t, uint32(55), response.StreamId)
}

func TestCreateIntegrationUploadRejectsInvalidRequestsWithoutQuery(t *testing.T) {
	for name, request := range map[string]*protobuf.CreateIntegrationUploadRequest{
		"no grant":        {ExternalRef: testUploadRef, Title: "Lecture"},
		"short reference": {GrantId: 1, ExternalRef: "short", Title: "Lecture"},
		"long reference":  {GrantId: 1, ExternalRef: strings.Repeat("a", 65), Title: "Lecture"},
		"reference chars": {GrantId: 1, ExternalRef: "artemis/unit/7/attempt", Title: "Lecture"},
		"blank title":     {GrantId: 1, ExternalRef: testUploadRef, Title: "   "},
		"long title":      {GrantId: 1, ExternalRef: testUploadRef, Title: strings.Repeat("ä", 256)},
	} {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			api := &API{dao: dao.DaoWrapper{IntegrationGrantDao: mock_dao.NewMockIntegrationGrantDao(ctrl)}, log: slog.Default()}

			_, err := api.CreateIntegrationUpload(integrationCaller(), request)

			assert.Equal(t, codes.InvalidArgument, status.Code(err))
		})
	}
}

func TestCreateIntegrationUploadMapsFailures(t *testing.T) {
	for name, test := range map[string]struct {
		err  error
		code codes.Code
		msg  string
	}{
		"missing or revoked grant":   {gorm.ErrRecordNotFound, codes.NotFound, "grant not found"},
		"reference already used":     {dao.ErrIntegrationUploadRefused, codes.AlreadyExists, dao.ErrIntegrationUploadRefused.Error()},
		"database failure is hidden": {errors.New("secret database detail"), codes.Unknown, "could not create upload"},
	} {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			grants := mock_dao.NewMockIntegrationGrantDao(ctrl)
			grants.EXPECT().CreateIntegrationUpload(gomock.Any(), uint(7), uint(123), testUploadRef, gomock.Any(), gomock.Any(), gomock.Any()).Return(model.Stream{}, false, test.err)
			api := &API{dao: dao.DaoWrapper{IntegrationGrantDao: grants}, log: slog.Default()}

			_, err := api.CreateIntegrationUpload(integrationCaller(), &protobuf.CreateIntegrationUploadRequest{GrantId: 123, ExternalRef: testUploadRef, Title: "Lecture"})

			assert.Equal(t, test.code, status.Code(err))
			assert.Equal(t, test.msg, status.Convert(err).Message())
		})
	}
}

const artemisOrigin = "https://artemis.example"

func uploadTicket() (string, []byte) {
	ticket := bytes.Repeat([]byte{5}, 32)
	hash := sha256.Sum256(ticket)
	return "/api/v2/integration/uploads/" + base64.RawURLEncoding.EncodeToString(ticket), hash[:]
}

func serveIntegrationUpload(api *API, req *http.Request) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Any("/api/v2/*any", func(c *gin.Context) {
		ticket, ok := integrationUploadRouteOf(c.Request)
		if !ok {
			c.Status(http.StatusTeapot)
			return
		}
		api.handleIntegrationUpload(c, ticket)
	})
	rec := httptest.NewRecorder()
	r.ServeHTTP(closeNotifyingRecorder{rec}, req)
	return rec
}

func TestIntegrationUploadTakesTheRecordingAsTheCombinedVersion(t *testing.T) {
	path, hash := uploadTicket()
	var gotFile string
	uploads := &fakeUploads{}
	uploads.target = fakeRunner(t, func(w http.ResponseWriter, r *http.Request) {
		f, _, err := r.FormFile("file")
		if err == nil {
			b, _ := io.ReadAll(f)
			gotFile = string(b)
		}
	})
	ctrl := gomock.NewController(t)
	grants := mock_dao.NewMockIntegrationGrantDao(ctrl)
	streams := mock_dao.NewMockStreamsDao(ctrl)
	grants.EXPECT().GetIntegrationUploadReturnURL(gomock.Any(), hash, gomock.Any()).Return(artemisOrigin+"/api/callback", nil)
	grants.EXPECT().ConsumeIntegrationUploadTicket(gomock.Any(), hash, gomock.Any()).Return(model.Stream{Model: gorm.Model{ID: 55}}, nil)
	streams.EXPECT().GetTranscodingProgressByVersion(model.COMB, uint(55)).Return(model.TranscodingProgress{}, gorm.ErrRecordNotFound)
	streams.EXPECT().SaveTranscodingProgress(model.TranscodingProgress{StreamID: 55, Version: model.COMB}).Return(nil)
	api := &API{dao: dao.DaoWrapper{IntegrationGrantDao: grants, StreamsDao: streams}, log: slog.Default(), uploads: uploads}

	req := uploadRequest(t, path+"?type=PRES", nil, "lecture.mp4", "video/mp4")
	req.Header.Set("Origin", artemisOrigin)
	rec := serveIntegrationUpload(api, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, artemisOrigin, rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, uint(55), uploads.streamID)
	assert.Equal(t, model.COMB, uploads.version, "the query cannot pick another version")
	assert.Equal(t, "content", gotFile)
}

func TestIntegrationUploadPreflightIsAnsweredForTheIntegrationsOriginOnly(t *testing.T) {
	path, hash := uploadTicket()
	for name, test := range map[string]struct {
		origin, returnURL string
		lookupErr         error
		status            int
	}{
		"registered origin":      {artemisOrigin, artemisOrigin + "/api/callback", nil, http.StatusNoContent},
		"origin with other host": {"https://evil.example", artemisOrigin + "/api/callback", nil, http.StatusForbidden},
		"origin with other port": {artemisOrigin + ":8443", artemisOrigin + "/api/callback", nil, http.StatusForbidden},
		"unknown or used ticket": {artemisOrigin, "", gorm.ErrRecordNotFound, http.StatusNotFound},
	} {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			grants := mock_dao.NewMockIntegrationGrantDao(ctrl)
			grants.EXPECT().GetIntegrationUploadReturnURL(gomock.Any(), hash, gomock.Any()).Return(test.returnURL, test.lookupErr)
			api := &API{dao: dao.DaoWrapper{IntegrationGrantDao: grants}, log: slog.Default()}
			req := httptest.NewRequest(http.MethodOptions, path, nil)
			req.Header.Set("Origin", test.origin)
			req.Header.Set("Access-Control-Request-Method", http.MethodPost)

			rec := serveIntegrationUpload(api, req)

			assert.Equal(t, test.status, rec.Code)
			if test.status == http.StatusNoContent {
				assert.Equal(t, test.origin, rec.Header().Get("Access-Control-Allow-Origin"))
				assert.Equal(t, "POST", rec.Header().Get("Access-Control-Allow-Methods"))
				assert.Equal(t, "Origin", rec.Header().Get("Vary"))
				assert.Empty(t, rec.Header().Get("Access-Control-Allow-Credentials"))
			} else {
				assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
			}
		})
	}
}

func TestIntegrationUploadKeepsTheTicketWhenRefusedBeforeUse(t *testing.T) {
	path, hash := uploadTicket()
	t.Run("another origin", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		grants := mock_dao.NewMockIntegrationGrantDao(ctrl)
		grants.EXPECT().GetIntegrationUploadReturnURL(gomock.Any(), hash, gomock.Any()).Return(artemisOrigin+"/api/callback", nil)
		api := &API{dao: dao.DaoWrapper{IntegrationGrantDao: grants}, log: slog.Default()}
		req := uploadRequest(t, path, nil, "lecture.mp4", "video/mp4")
		req.Header.Set("Origin", "https://evil.example")

		rec := serveIntegrationUpload(api, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
	})
	for name, ticketPath := range map[string]string{
		"not base64url": "/api/v2/integration/uploads/not*base64+url",
		"too short":     "/api/v2/integration/uploads/" + base64.RawURLEncoding.EncodeToString([]byte("short")),
		"padded":        path + "=",
	} {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			api := &API{dao: dao.DaoWrapper{IntegrationGrantDao: mock_dao.NewMockIntegrationGrantDao(ctrl)}, log: slog.Default()}

			rec := serveIntegrationUpload(api, uploadRequest(t, ticketPath, nil, "lecture.mp4", "video/mp4"))

			assert.Equal(t, http.StatusNotFound, rec.Code)
		})
	}
}

func TestIntegrationUploadAnswersAnUnusableTicketWithNotFound(t *testing.T) {
	path, hash := uploadTicket()
	for name, err := range map[string]error{
		"used, expired, revoked or deleted": gorm.ErrRecordNotFound,
		"database failure is hidden":        errors.New("secret database detail"),
	} {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			grants := mock_dao.NewMockIntegrationGrantDao(ctrl)
			grants.EXPECT().ConsumeIntegrationUploadTicket(gomock.Any(), hash, gomock.Any()).Return(model.Stream{}, err)
			api := &API{dao: dao.DaoWrapper{IntegrationGrantDao: grants}, log: slog.Default(), uploads: &fakeUploads{}}

			// Without an Origin: a server-side client, not a browser.
			rec := serveIntegrationUpload(api, uploadRequest(t, path, nil, "lecture.mp4", "video/mp4"))

			assert.Equal(t, http.StatusNotFound, rec.Code)
			_, msg := gatewayError(t, rec.Body.Bytes())
			assert.Equal(t, "upload not found", msg)
		})
	}
}

func TestOriginOfMatchesWhatBrowsersSend(t *testing.T) {
	for returnURL, origin := range map[string]string{
		"https://artemis.example/api/callback":      "https://artemis.example",
		"https://Artemis.Example:443/api/callback":  "https://artemis.example",
		"https://artemis.example:8443/api/callback": "https://artemis.example:8443",
		"http://artemis.localhost:80/callback":      "http://artemis.localhost",
		"http://artemis.localhost:18080/callback":   "http://artemis.localhost:18080",
		"http://[::1]:8080/callback":                "http://[::1]:8080",
		"http://[0:0:0:0:0:0:0:1]:8080/callback":    "http://[::1]:8080",
		"http://127.0.0.1:8080/callback":            "http://127.0.0.1:8080",
		"https://bücher.example/callback":           "https://xn--bcher-kva.example",
		"https://BÜCHER.example:443/callback":       "https://xn--bcher-kva.example",
		"not a url":                                 "",
	} {
		assert.Equal(t, origin, originOf(returnURL), returnURL)
	}
}
