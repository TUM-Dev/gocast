// Package apiv2 provides API endpoints for the application.
package apiv2

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	e "github.com/TUM-Dev/gocast/apiv2/errors"
	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/tools"
)

// clampTTL returns a TTL (in seconds) clamped to [300, 86400].
// A zero or negative value is treated as the default of 7200 seconds.
func clampTTL(ttlSeconds int) int {
	const defaultTTL = 7200
	const minTTL = 300
	const maxTTL = 86400

	if ttlSeconds <= 0 {
		return defaultTTL
	}
	if ttlSeconds < minTTL {
		return minTTL
	}
	if ttlSeconds > maxTTL {
		return maxTTL
	}
	return ttlSeconds
}

// ListAdministeredCourses implements IntegrationService/listAdministeredCourses (EP1).
//
// Authentication: requires a valid service-account bearer token (ServiceType user
// with TokenScopeService). Does NOT accept session cookies.
//
// The LrzId in the request identifies the target user whose directly-administered
// courses are to be returned. "Directly-administered" means the user has an explicit
// entry in course_admins — global AdminType users do NOT implicitly receive all
// courses via this endpoint.
func (a *API) ListAdministeredCourses(ctx context.Context, req *protobuf.ListAdministeredCoursesRequest) (*protobuf.ListAdministeredCoursesResponse, error) {
	if _, err := a.getServiceAccount(ctx); err != nil {
		return nil, err
	}

	target, err := a.dao.UsersDao.GetUserByLrzID(req.LrzId)
	if err != nil {
		return nil, e.WithStatus(http.StatusNotFound, errors.New("user not found"))
	}

	courses, err := a.dao.CoursesDao.GetDirectlyAdministeredCoursesByUserId(ctx, target.ID, req.Term, int(req.Year))
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	out := make([]*protobuf.IntegrationCourse, 0, len(courses))
	for _, c := range courses {
		out = append(out, &protobuf.IntegrationCourse{
			Id:           uint32(c.ID),
			Name:         c.Name,
			Slug:         c.Slug,
			Year:         int32(c.Year),
			TeachingTerm: c.TeachingTerm,
			VodEnabled:   c.VODEnabled,
			Visibility:   c.Visibility,
		})
	}

	return &protobuf.ListAdministeredCoursesResponse{Courses: out}, nil
}

// ListCourseStreams implements IntegrationService/listCourseStreams (EP8).
//
// Authentication: requires a valid service-account bearer token (ServiceType
// user with TokenScopeService). The service account must be an admin of the
// requested course (i.e. the course binding is established).
//
// Streams preload note: GetCourseById (called inside requireServiceCourseAdmin)
// already preloads Streams with sub-preloads for TranscodingProgresses,
// VideoSections, and Files ordered by start desc. Therefore course.Streams is
// fully populated on return and no additional DAO call is needed.
func (a *API) ListCourseStreams(ctx context.Context, req *protobuf.ListCourseStreamsRequest) (*protobuf.ListCourseStreamsResponse, error) {
	_, course, err := a.requireServiceCourseAdmin(ctx, uint(req.CourseId))
	if err != nil {
		return nil, err
	}

	out := make([]*protobuf.IntegrationStream, 0, len(course.Streams))
	for _, s := range course.Streams {
		out = append(out, &protobuf.IntegrationStream{
			StreamId: uint32(s.ID),
			Name:     s.GetName(),
			Private:  s.Private,
			Start:    timestamppb.New(s.Start),
			End:      timestamppb.New(s.End),
		})
	}

	return &protobuf.ListCourseStreamsResponse{Streams: out}, nil
}

// GetPlaybackToken implements IntegrationService/getPlaybackToken (EP2).
//
// Authentication: requires a valid service-account bearer token. The service
// account must be an admin of the requested course (i.e. the binding is
// established). An on-behalf-of LRZ ID (X-On-Behalf-Of header) identifies the
// acting user whose eligibility is enforced independently.
//
// The returned URLs are JWT-signed with a TTL clamped to [300, 86400] seconds
// (default 7200 when the caller passes 0 or a negative value).
func (a *API) GetPlaybackToken(ctx context.Context, req *protobuf.GetPlaybackTokenRequest) (*protobuf.GetPlaybackTokenResponse, error) {
	// (1) Service account must be bound to this course (course admin).
	_, course, err := a.requireServiceCourseAdmin(ctx, uint(req.CourseId))
	if err != nil {
		return nil, err
	}

	// (2) The acting user must exist and be independently eligible.
	obo, err := a.getOnBehalfOfUser(ctx)
	if err != nil {
		return nil, err
	}

	// (3) Load the stream.
	stream, err := a.dao.GetStreamByID(ctx, fmt.Sprintf("%d", req.StreamId))
	if err != nil {
		return nil, e.WithStatus(http.StatusNotFound, errors.New("stream not found"))
	}

	// (4) Stream must belong to the path course. We deliberately return 400
	//     (not 404) here: the stream exists, just not in this course, and we do
	//     not want to confirm a stream's existence in some other course to a
	//     caller that is only bound to this one.
	if stream.CourseID != course.ID {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("stream does not belong to course"))
	}

	// (5) OBO user eligibility: must be eligible to watch the course, and if the
	//     stream is private, must be an admin of the course.
	if !obo.IsEligibleToWatchCourse(course) || (stream.Private && !obo.IsAdminOfCourse(course)) {
		return nil, e.WithStatus(http.StatusForbidden, errors.New("user not eligible to watch stream"))
	}

	// (6) Clamp the requested TTL. int(req.TtlSeconds) is safe: TtlSeconds is an
	//     int32 and clampTTL bounds the result to [300, 86400].
	ttl := clampTTL(int(req.TtlSeconds))
	ttlDur := time.Duration(ttl) * time.Second

	// (7) Sign each non-empty playlist variant; surface signing errors.
	sign := func(raw string) (string, error) {
		if raw == "" {
			return "", nil
		}
		return tools.SignPlaylistURL(obo, raw, stream.ID, course.ID, course.DownloadsEnabled, ttlDur)
	}

	comb, err := sign(stream.PlaylistUrl)
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	pres, err := sign(stream.PlaylistUrlPRES)
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	cam, err := sign(stream.PlaylistUrlCAM)
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	// (8) At least one variant must be playable.
	if comb == "" && pres == "" && cam == "" {
		return nil, e.WithStatus(http.StatusNotFound, errors.New("no playable variants for stream"))
	}

	return &protobuf.GetPlaybackTokenResponse{
		PlaylistUrl:     comb,
		PlaylistUrlPres: pres,
		PlaylistUrlCam:  cam,
		// int32(ttl) is safe: ttl is bounded to [300, 86400] by clampTTL.
		ExpiresIn: int32(ttl),
	}, nil
}
