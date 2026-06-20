// Package apiv2 provides API endpoints for the application.
package apiv2

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc/metadata"
	"gorm.io/gorm"

	e "github.com/TUM-Dev/gocast/apiv2/errors"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
)

// getCurrent retrieves the current user based on the context.
// It returns a User or an error if one occurs.
func (a *API) getCurrent(ctx context.Context) (*model.User, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, errors.New("no metadata")
	}

	jwtStr, err := a.extractJWTFromMetadata(md)
	if err != nil {
		return nil, err
	}

	claims, err := a.parseJWT(jwtStr)
	if err != nil {
		return nil, err
	}

	return a.getUserFromClaims(ctx, claims)
}

// extractJWTFromMetadata extracts the JWT cookie from the metadata.
// It returns a string or an error if one occurs.
func (a *API) extractJWTFromMetadata(md metadata.MD) (string, error) {
	cookies, ok := md["grpcgateway-cookie"]
	if !ok || len(cookies) < 1 {
		return "", errors.New("missing cookie header")
	}

	return extractTokenFromCookie(cookies[0])
}

// extractTokenFromCookie extracts the actual JWT from the cookie header.
// It returns a string or an error if one occurs.
func extractTokenFromCookie(cookieHeader string) (string, error) {
	cookies := strings.Split(cookieHeader, ";")
	for _, cookie := range cookies {
		cookie = strings.TrimSpace(cookie)
		if strings.HasPrefix(cookie, "jwt=") {
			return strings.TrimPrefix(cookie, "jwt="), nil
		}
	}

	return "", errors.New("jwt cookie not found")
}

// parseJWT parses the JWT string.
// It returns JWTClaims or an error if one occurs.
func (a *API) parseJWT(jwtStr string) (*tools.JWTClaims, error) {
	token, err := jwt.ParseWithClaims(jwtStr, &tools.JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		return tools.Cfg.GetJWTKey().Public(), nil
	})
	if err != nil {
		a.log.Info("JWT parsing error", "err", err)
		return nil, err
	}

	if !token.Valid {
		a.log.Info("JWT token is not valid")
		return nil, errors.New("JWT token is not valid")
	}

	claims, ok := token.Claims.(*tools.JWTClaims)
	if !ok {
		return nil, errors.New("error extracting claims from token")
	}

	return claims, nil
}

// getUserFromClaims retrieves the user from the claims.
// It returns a User or an error if one occurs.
func (a *API) getUserFromClaims(ctx context.Context, claims *tools.JWTClaims) (*model.User, error) {
	user, err := a.dao.GetUserByID(ctx, claims.UserID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	return &user, nil
}

// ---------------------------------------------------------------------------
// Service-account bearer auth helpers (G3)
// ---------------------------------------------------------------------------

// extractBearerFromMetadata extracts the raw token string from the
// "authorization" metadata key (which the shared header matcher maps from the
// HTTP Authorization header).
func extractBearerFromMetadata(md metadata.MD) (string, error) {
	vals := md.Get("authorization")
	if len(vals) == 0 {
		return "", e.WithStatus(http.StatusUnauthorized, errors.New("missing authorization"))
	}
	after, ok := strings.CutPrefix(vals[0], "Bearer ")
	if !ok {
		return "", e.WithStatus(http.StatusUnauthorized, errors.New("malformed bearer token"))
	}
	return after, nil
}

// getServiceAccount authenticates the incoming RPC using the bearer token
// carried in the "authorization" metadata key, validates that the token has
// service scope, loads the user, and enforces that the user has ServiceType
// role (defense-in-depth). It records a "token used" timestamp on success.
func (a *API) getServiceAccount(ctx context.Context) (*model.User, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, e.WithStatus(http.StatusUnauthorized, errors.New("no metadata"))
	}
	raw, err := extractBearerFromMetadata(md)
	if err != nil {
		return nil, err
	}
	tok, err := a.dao.TokenDao.GetToken(raw) // already filters expired tokens
	if err != nil {
		return nil, e.WithStatus(http.StatusUnauthorized, errors.New("invalid token"))
	}
	if tok.Scope != model.TokenScopeService {
		return nil, e.WithStatus(http.StatusForbidden, errors.New("token not authorized for integration"))
	}
	if err := a.dao.TokenDao.TokenUsed(tok); err != nil {
		a.log.Warn("failed to record token last_use", "err", err)
	}
	user, err := a.dao.GetUserByID(ctx, tok.UserID)
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	if user.Role != model.ServiceType {
		// Defense-in-depth: a service-scoped token on a non-service-type user
		// must not gain integration access.
		return nil, e.WithStatus(http.StatusForbidden, errors.New("token not bound to a service account"))
	}
	return &user, nil
}

// getOnBehalfOfUser resolves the user identified by the "x-on-behalf-of"
// metadata key (mapped from the X-On-Behalf-Of HTTP header) via their LRZ ID.
func (a *API) getOnBehalfOfUser(ctx context.Context) (*model.User, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	vals := md.Get("x-on-behalf-of")
	if len(vals) == 0 || vals[0] == "" {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("missing X-On-Behalf-Of"))
	}
	user, err := a.dao.UsersDao.GetUserByLrzID(vals[0])
	if err != nil {
		return nil, e.WithStatus(http.StatusNotFound, errors.New("on-behalf-of user not found"))
	}
	return &user, nil
}

// requireServiceCourseAdmin verifies that:
//  1. the request carries a valid service-account bearer token (getServiceAccount), and
//  2. that service account is an admin of the requested course.
//
// Returns the authenticated service user and the loaded course on success.
//
// Note on IsAdminOfCourse: getServiceAccount guarantees that svc.Role ==
// ServiceType. IsAdminOfCourse's AdminType shortcut (which would make any
// global AdminType user appear as admin of every course) therefore cannot fire
// here — the check is a genuine course-admin membership lookup through the
// AdministeredCourses association, ensuring only explicitly bound service
// accounts can act on a course.
func (a *API) requireServiceCourseAdmin(ctx context.Context, courseID uint) (*model.User, model.Course, error) {
	svc, err := a.getServiceAccount(ctx)
	if err != nil {
		return nil, model.Course{}, err
	}
	course, err := a.dao.CoursesDao.GetCourseById(ctx, courseID)
	if err != nil {
		return nil, model.Course{}, e.WithStatus(http.StatusNotFound, err)
	}
	if !svc.IsAdminOfCourse(course) {
		return nil, course, e.WithStatus(http.StatusForbidden, errors.New("service account not bound to course"))
	}
	return svc, course, nil
}

// ---------------------------------------------------------------------------
// Stream request authorization (existing)
// ---------------------------------------------------------------------------

type StreamRequest interface {
	GetStreamId() uint32
}

func (a *API) authorizeUserForStreamCourse(ctx context.Context, req StreamRequest) (*model.User, model.Stream, model.Course, error) {
	var stream model.Stream
	var course model.Course

	stream, err := a.dao.GetStreamByID(ctx, strconv.FormatUint(uint64(req.GetStreamId()), 10))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, stream, course, e.WithStatus(http.StatusNotFound, err)
		}
		return nil, stream, course, e.WithStatus(http.StatusInternalServerError, err)
	}

	course, err = a.dao.GetCourseById(ctx, stream.CourseID)
	if err != nil {
		return nil, stream, course, e.WithStatus(http.StatusInternalServerError, err)
	}

	// Only check slug if request requires it
	type slugGetter interface {
		GetSlug() string
	}
	if r, ok := req.(slugGetter); ok {
		if r.GetSlug() != course.Slug {
			return nil, stream, course, e.WithStatus(http.StatusBadRequest, errors.New("slug does not match course"))
		}
	}

	user, _ := a.getCurrent(ctx)
	if !user.IsEligibleToWatchCourse(course) {
		return nil, stream, course, e.WithStatus(http.StatusForbidden, errors.New("User is not eligible to access course content"))
	}

	if stream.Private && (user == nil || !user.IsAdminOfCourse(course)) {
		return nil, stream, course, e.WithStatus(http.StatusForbidden, errors.New("User is not allowed to access private stream"))
	}

	return user, stream, course, nil
}
