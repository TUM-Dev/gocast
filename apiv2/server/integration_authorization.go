package apiv2

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"
	"uuid"

	"google.golang.org/grpc/metadata"

	e "github.com/TUM-Dev/gocast/apiv2/errors"
	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/model"
)

const integrationCodeLifetime = 5 * time.Minute

var (
	errAuthorizationState = errors.New("this authorization link is invalid; return to the application and start again")
	errAuthorizationGone  = errors.New("this authorization request is no longer available; return to the application and start again")
)

// validAuthorizationState accepts only what an application is told to send: 32 random
// bytes as unpadded URL-safe base64, in its canonical spelling.
func validAuthorizationState(state string) ([]byte, bool) {
	raw, err := base64.RawURLEncoding.DecodeString(state)
	return raw, err == nil && len(raw) == 32 && base64.RawURLEncoding.EncodeToString(raw) == state
}

// canAuthorizeCourse needs an explicit course role; CanAdminister also permits
// instance admins, who must not connect applications to courses they do not run.
func canAuthorizeCourse(user *model.User, course model.Course) bool {
	if user == nil || user.ID == 0 {
		return false
	}
	if course.UserID == user.ID {
		return true
	}
	for _, admin := range course.Admins {
		if admin.ID == user.ID {
			return true
		}
	}
	return false
}

// GetIntegrationAuthorization is what the consent page shows. It changes nothing.
func (a *API) GetIntegrationAuthorization(ctx context.Context, req *protobuf.GetIntegrationAuthorizationRequest) (*protobuf.IntegrationAuthorization, error) {
	user, err := a.getCurrent(ctx)
	if err != nil {
		return nil, err
	}
	if _, ok := validAuthorizationState(req.GetState()); !ok || req.GetIntegrationId() == 0 {
		return nil, e.WithStatus(http.StatusBadRequest, errAuthorizationState)
	}
	integration, err := a.dao.IntegrationGrantDao.GetIntegrationByID(ctx, uint(req.GetIntegrationId()))
	if err != nil {
		return nil, e.FromGorm(err, errAuthorizationGone.Error())
	}
	courses, err := a.dao.IntegrationGrantDao.GetAuthorizableCourses(ctx, user.ID)
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	response := &protobuf.IntegrationAuthorization{
		IntegrationName: integration.Name,
		UserName:        user.GetPreferredName(),
		Courses:         make([]*protobuf.AuthorizableCourse, 0, len(courses)),
	}
	for _, course := range courses {
		response.Courses = append(response.Courses, &protobuf.AuthorizableCourse{
			Id: uint32(course.ID), Name: course.Name, TeachingTerm: course.TeachingTerm, Year: uint32(course.Year),
		})
	}
	return response, nil
}

// DecideIntegrationAuthorization records the user's answer and says where to send the
// browser. Approving is the one step here that hands a credential to a third party,
// so it is a deliberate request that the page makes with a bearer token:
//
//   - A cookie is attached by the browser to a request any page can trigger, so a
//     cookie-authenticated caller is refused. A bearer token has to be set by script
//     of this origin, and a foreign page cannot do that without a CORS preflight,
//     which the API never grants. (The server-rendered form relied on SameSite
//     defaults for this.)
//   - The page that calls it is served with frame-ancestors 'none', so it cannot be
//     clicked through from a framing page; see serveIntegrationAuthorization.
func (a *API) DecideIntegrationAuthorization(ctx context.Context, req *protobuf.DecideIntegrationAuthorizationRequest) (*protobuf.DecideIntegrationAuthorizationResponse, error) {
	user, err := a.getCurrent(ctx)
	if err != nil {
		return nil, err
	}
	md, _ := metadata.FromIncomingContext(ctx)
	if _, bearer := extractBearerToken(md); !bearer {
		return nil, e.WithStatus(http.StatusForbidden, errors.New("authorizing an application needs a bearer token, not a session cookie alone"))
	}
	state := req.GetState()
	stateRaw, validState := validAuthorizationState(state)
	if !validState || req.GetIntegrationId() == 0 {
		return nil, e.WithStatus(http.StatusBadRequest, errAuthorizationState)
	}
	integrationID := uint(req.GetIntegrationId())
	integration, err := a.dao.IntegrationGrantDao.GetIntegrationByID(ctx, integrationID)
	if err != nil {
		return nil, e.FromGorm(err, errAuthorizationGone.Error())
	}

	switch req.GetDecision() {
	case protobuf.IntegrationAuthorizationDecision_INTEGRATION_AUTHORIZATION_DECISION_CANCEL:
		return &protobuf.DecideIntegrationAuthorizationResponse{
			RedirectUrl: integrationReturnURL(integration.ReturnURL, state, "error", "access_denied"),
		}, nil
	case protobuf.IntegrationAuthorizationDecision_INTEGRATION_AUTHORIZATION_DECISION_APPROVE:
	default:
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("choose whether to authorize or cancel"))
	}

	if req.GetCourseId() == 0 {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("choose a course"))
	}
	courseID := uint(req.GetCourseId())
	course, err := a.dao.IntegrationGrantDao.GetCourseForAuthorization(ctx, courseID)
	if err != nil {
		return nil, e.FromGorm(err, "that course is no longer available")
	}
	if !canAuthorizeCourse(user, course) {
		return nil, e.WithStatus(http.StatusForbidden, errors.New("you no longer have permission to authorize applications for this course"))
	}
	code := uuid.NewV4()
	codeHash, stateHash := sha256.Sum256(code[:]), sha256.Sum256(stateRaw)
	grantID, err := a.dao.IntegrationGrantDao.ApproveIntegrationCourse(ctx, integrationID, courseID, codeHash[:], stateHash[:], time.Now().Add(integrationCodeLifetime))
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	a.audit(ctx, model.AuditCourseEdit, fmt.Sprintf("%s:'%s' authorize integration: %s (%d)", course.Name, course.Slug, integration.Name, grantID))
	return &protobuf.DecideIntegrationAuthorizationResponse{
		RedirectUrl: integrationReturnURL(integration.ReturnURL, state, "code", code.String()),
	}, nil
}

func integrationReturnURL(raw, state, resultName, result string) string {
	u, _ := url.Parse(raw)
	query := u.Query()
	query.Set("state", state)
	query.Set(resultName, result)
	u.RawQuery = query.Encode()
	return u.String()
}
