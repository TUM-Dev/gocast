package apiv2

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"log/slog"
	"net/url"
	"testing"
	"time"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"gorm.io/gorm"

	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
)

func authorizationState(fill byte) string {
	return base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{fill}, 32))
}

// withBearer marks a request as made with an Authorization header, as the SPA does.
func withBearer(ctx context.Context) context.Context {
	return metadata.NewIncomingContext(ctx, metadata.Pairs("authorization", "Bearer x"))
}

func authorizationAPI(t *testing.T) (*API, *mock_dao.MockIntegrationGrantDao, *mock_dao.MockAuditDao) {
	t.Helper()
	ctrl := gomock.NewController(t)
	grants := mock_dao.NewMockIntegrationGrantDao(ctrl)
	audits := mock_dao.NewMockAuditDao(ctrl)
	return &API{dao: dao.DaoWrapper{IntegrationGrantDao: grants, AuditDao: audits}, log: slog.Default()}, grants, audits
}

const (
	approve = protobuf.IntegrationAuthorizationDecision_INTEGRATION_AUTHORIZATION_DECISION_APPROVE
	cancel  = protobuf.IntegrationAuthorizationDecision_INTEGRATION_AUTHORIZATION_DECISION_CANCEL
)

func TestGetIntegrationAuthorization(t *testing.T) {
	api, grants, _ := authorizationAPI(t)
	user := &model.User{Model: gorm.Model{ID: 1}, Name: "Prof One", Role: model.LecturerType}
	grants.EXPECT().GetIntegrationByID(gomock.Any(), uint(7)).Return(model.Integration{ID: 7, Name: "Course portal"}, nil)
	grants.EXPECT().GetAuthorizableCourses(gomock.Any(), uint(1)).Return([]model.Course{
		{Model: gorm.Model{ID: 11}, Name: "Example course", TeachingTerm: "W", Year: 2026},
	}, nil)

	response, err := api.GetIntegrationAuthorization(asCaller(user), &protobuf.GetIntegrationAuthorizationRequest{
		IntegrationId: 7, State: authorizationState(1),
	})

	require.NoError(t, err)
	assert.Equal(t, "Course portal", response.IntegrationName)
	require.Len(t, response.Courses, 1)
	assert.Equal(t, uint32(11), response.Courses[0].Id)
	assert.Equal(t, "W", response.Courses[0].TeachingTerm)
}

func TestGetIntegrationAuthorizationRefusals(t *testing.T) {
	user := &model.User{Model: gorm.Model{ID: 1}}
	api, grants, _ := authorizationAPI(t)
	_, err := api.GetIntegrationAuthorization(asCaller(user), &protobuf.GetIntegrationAuthorizationRequest{IntegrationId: 7, State: "short"})
	wantCode(t, err, codes.InvalidArgument)

	grants.EXPECT().GetIntegrationByID(gomock.Any(), uint(7)).Return(model.Integration{}, gorm.ErrRecordNotFound)
	_, err = api.GetIntegrationAuthorization(asCaller(user), &protobuf.GetIntegrationAuthorizationRequest{IntegrationId: 7, State: authorizationState(1)})
	wantCode(t, err, codes.NotFound)
}

func TestDecideApprovesOnlyOwnerOrDelegate(t *testing.T) {
	state := authorizationState(3)
	course := model.Course{
		Name: "Example course", Slug: "example", UserID: 1,
		Admins: []model.User{{Model: gorm.Model{ID: 2}}},
	}
	for _, test := range []struct {
		name    string
		userID  uint
		role    uint
		allowed bool
	}{
		{name: "owner", userID: 1, role: model.LecturerType, allowed: true},
		{name: "delegate", userID: 2, role: model.LecturerType, allowed: true},
		{name: "outsider", userID: 3, role: model.LecturerType},
		{name: "instance admin without course grant", userID: 4, role: model.AdminType},
	} {
		t.Run(test.name, func(t *testing.T) {
			api, grants, audits := authorizationAPI(t)
			user := &model.User{Model: gorm.Model{ID: test.userID}, Role: test.role}
			grants.EXPECT().GetIntegrationByID(gomock.Any(), uint(7)).Return(model.Integration{
				ID: 7, Name: "Course portal", ReturnURL: "https://client.example/callback?existing=1",
			}, nil)
			grants.EXPECT().GetCourseForAuthorization(gomock.Any(), uint(11)).Return(course, nil)
			var codeHash, stateHash []byte
			var expiresAt time.Time
			if test.allowed {
				grants.EXPECT().ApproveIntegrationCourse(gomock.Any(), uint(7), uint(11), gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _, _ uint, gotCode, gotState []byte, gotExpiresAt time.Time) (uint, error) {
						codeHash, stateHash, expiresAt = bytes.Clone(gotCode), bytes.Clone(gotState), gotExpiresAt
						return 19, nil
					})
				courseID := course.ID
				audits.EXPECT().Create(&model.Audit{
					User:     user,
					Message:  "Example course:'example' authorize integration: Course portal (19)",
					Type:     model.AuditCourseEdit,
					CourseID: &courseID,
				}).Return(nil)
			}

			response, err := api.DecideIntegrationAuthorization(withBearer(asCaller(user)), &protobuf.DecideIntegrationAuthorizationRequest{
				IntegrationId: 7, State: state, Decision: approve, CourseId: 11,
			})

			if !test.allowed {
				wantCode(t, err, codes.PermissionDenied)
				return
			}
			require.NoError(t, err)
			location, err := url.Parse(response.RedirectUrl)
			require.NoError(t, err)
			code, err := uuid.Parse(location.Query().Get("code"))
			require.NoError(t, err)
			wantCodeHash := sha256.Sum256(code[:])
			wantStateHash := sha256.Sum256(bytes.Repeat([]byte{3}, 32))
			assert.True(t, isV4RFC4122(code))
			assert.Equal(t, wantCodeHash[:], codeHash)
			assert.Equal(t, wantStateHash[:], stateHash)
			assert.WithinDuration(t, time.Now().Add(integrationCodeLifetime), expiresAt, 2*time.Second)
			assert.Equal(t, state, location.Query().Get("state"))
			assert.Equal(t, "1", location.Query().Get("existing"))
		})
	}
}

// A cookie is sent with any request a foreign page can trigger; only a bearer token
// proves the request came from this origin's script.
func TestDecideRefusesACookieOnlyCaller(t *testing.T) {
	api, _, _ := authorizationAPI(t)
	user := &model.User{Model: gorm.Model{ID: 1}}
	_, err := api.DecideIntegrationAuthorization(asCaller(user), &protobuf.DecideIntegrationAuthorizationRequest{
		IntegrationId: 7, State: authorizationState(1), Decision: approve, CourseId: 11,
	})
	wantCode(t, err, codes.PermissionDenied)
}

func TestDecideRejectsInvalidStateBeforeAnyRead(t *testing.T) {
	api, _, _ := authorizationAPI(t)
	user := &model.User{Model: gorm.Model{ID: 1}}
	_, err := api.DecideIntegrationAuthorization(withBearer(asCaller(user)), &protobuf.DecideIntegrationAuthorizationRequest{
		IntegrationId: 7, State: "short", Decision: approve, CourseId: 11,
	})
	wantCode(t, err, codes.InvalidArgument)
}

func TestDecideCancelDoesNotPersist(t *testing.T) {
	api, grants, _ := authorizationAPI(t)
	grants.EXPECT().GetIntegrationByID(gomock.Any(), uint(7)).Return(model.Integration{ID: 7, ReturnURL: "https://client.example/callback"}, nil)
	state := authorizationState(6)

	response, err := api.DecideIntegrationAuthorization(withBearer(asCaller(&model.User{Model: gorm.Model{ID: 1}})), &protobuf.DecideIntegrationAuthorizationRequest{
		IntegrationId: 7, State: state, Decision: cancel,
	})

	require.NoError(t, err)
	location, err := url.Parse(response.RedirectUrl)
	require.NoError(t, err)
	assert.Equal(t, "access_denied", location.Query().Get("error"))
	assert.Equal(t, state, location.Query().Get("state"))
	assert.Empty(t, location.Query().Get("code"))
}

func TestDecideRequiresADecisionAndACourse(t *testing.T) {
	user := &model.User{Model: gorm.Model{ID: 1}}
	for name, req := range map[string]*protobuf.DecideIntegrationAuthorizationRequest{
		"no decision": {IntegrationId: 7, State: authorizationState(1), CourseId: 11},
		"no course":   {IntegrationId: 7, State: authorizationState(1), Decision: approve},
	} {
		t.Run(name, func(t *testing.T) {
			api, grants, _ := authorizationAPI(t)
			grants.EXPECT().GetIntegrationByID(gomock.Any(), uint(7)).Return(model.Integration{ID: 7}, nil)
			_, err := api.DecideIntegrationAuthorization(withBearer(asCaller(user)), req)
			wantCode(t, err, codes.InvalidArgument)
		})
	}
}

func TestDecideReportsWriteFailure(t *testing.T) {
	api, grants, _ := authorizationAPI(t)
	grants.EXPECT().GetIntegrationByID(gomock.Any(), uint(7)).Return(model.Integration{ID: 7}, nil)
	grants.EXPECT().GetCourseForAuthorization(gomock.Any(), uint(11)).Return(model.Course{UserID: 1}, nil)
	grants.EXPECT().ApproveIntegrationCourse(gomock.Any(), uint(7), uint(11), gomock.Any(), gomock.Any(), gomock.Any()).Return(uint(0), errors.New("write failed"))
	_, err := api.DecideIntegrationAuthorization(withBearer(asCaller(&model.User{Model: gorm.Model{ID: 1}})), &protobuf.DecideIntegrationAuthorizationRequest{
		IntegrationId: 7, State: authorizationState(10), Decision: approve, CourseId: 11,
	})
	wantCode(t, err, codes.Unknown)
}
