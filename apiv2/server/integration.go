// Package apiv2 provides API endpoints for the application.
package apiv2

import (
	"context"
	"errors"
	"net/http"

	e "github.com/TUM-Dev/gocast/apiv2/errors"
	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
)

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
