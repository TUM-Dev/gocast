// Package apiv2 provides API endpoints for the application.
package apiv2

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc/metadata"
	"gorm.io/gorm"

	e "github.com/TUM-Dev/gocast/apiv2/errors"
	"github.com/TUM-Dev/gocast/apiv2/visibility"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
)

// getCurrent returns the user making the request, or an error if there is none.
//
// The resolveCaller interceptor does the work once per request; this reads what it
// left behind, and falls back to resolving inline when no interceptor has run (a
// unit test calling a handler directly).
func (a *API) getCurrent(ctx context.Context) (*model.User, error) {
	if c, ok := ctx.Value(callerKey{}).(*caller); ok {
		return c.user, c.err
	}

	return a.resolveCurrent(ctx)
}

// ErrNoCredentials means nothing was presented to authenticate with, as opposed to
// something presented and rejected. On endpoints serving anonymous callers the first
// answers normally and the second is an expired token that needs a 401 to refresh.
var ErrNoCredentials = errors.New("no credentials presented")

// currentOrAnonymous returns the caller, or nil when the request carried no
// credentials. For endpoints that serve anonymous callers: a rejected credential is
// a 401 rather than a silent downgrade to the logged-out view.
func (a *API) currentOrAnonymous(ctx context.Context) (*model.User, error) {
	user, err := a.getCurrent(ctx)
	if err == nil {
		return user, nil
	}

	if errors.Is(err, ErrNoCredentials) {
		return nil, nil
	}

	return nil, e.WithStatus(http.StatusUnauthorized, err)
}

// resolveCurrent authenticates the request from its metadata.
// It returns a User or an error if one occurs.
func (a *API) resolveCurrent(ctx context.Context) (*model.User, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, fmt.Errorf("%w: no metadata", ErrNoCredentials)
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

// extractJWTFromMetadata extracts the caller's JWT from the request metadata.
//
// A bearer token wins over the session cookie; the cookie is only still supported for
// the server-rendered pages and can go once the last one does. grpc-gateway forwards
// `Authorization` unprefixed and `Cookie` as `grpcgateway-cookie`.
func (a *API) extractJWTFromMetadata(md metadata.MD) (string, error) {
	if token, ok := extractBearerToken(md); ok {
		return token, nil
	}

	cookies, ok := md["grpcgateway-cookie"]
	if !ok || len(cookies) < 1 {
		return "", fmt.Errorf("%w: no bearer token or session cookie", ErrNoCredentials)
	}

	return extractTokenFromCookie(cookies[0])
}

// extractBearerToken returns the token from an `Authorization: Bearer <token>`
// header, if one is present and non-empty.
func extractBearerToken(md metadata.MD) (string, bool) {
	for _, header := range md["authorization"] {
		rest, found := strings.CutPrefix(header, "Bearer ")
		if !found {
			continue
		}
		if token := strings.TrimSpace(rest); token != "" {
			return token, true
		}
	}

	return "", false
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

	return "", fmt.Errorf("%w: no jwt cookie", ErrNoCredentials)
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

	// GetUserByID uses gorm's Find, which reports a missing row as a zero-value user
	// and a nil error. Without the ID check, a token naming a deleted user would
	// authenticate as user 0.
	if err != nil || user.ID == 0 {
		return nil, e.WithStatus(http.StatusUnauthorized, errors.New("no user for the presented credentials"))
	}

	return &user, nil
}

type StreamRequest interface {
	GetStreamId() uint32
}

func (a *API) authorizeUserForStreamCourse(ctx context.Context, req StreamRequest) (*model.User, model.Stream, model.Course, error) {
	var stream model.Stream
	var course model.Course

	stream, err := a.dao.GetStreamByID(ctx, strconv.FormatUint(uint64(req.GetStreamId()), 10))
	if err != nil {
		return nil, stream, course, e.FromGorm(err, "can't find stream")
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

	user, err := a.currentOrAnonymous(ctx)
	if err != nil {
		return nil, stream, course, err
	}
	if !user.IsEligibleToWatchCourse(course) {
		return nil, stream, course, e.WithStatus(http.StatusForbidden, errors.New("User is not eligible to access course content"))
	}

	if !visibility.StreamVisible(user, course, stream) {
		return nil, stream, course, e.WithStatus(http.StatusForbidden, errors.New("User is not allowed to access private stream"))
	}

	return user, stream, course, nil
}
