package apiv2

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

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
