// Package api_v2 provides API endpoints for the application.
package api_v2

import (
	"context"
	"errors"
	e "github.com/TUM-Dev/gocast/api_v2/errors"
	h "github.com/TUM-Dev/gocast/api_v2/helpers"
	s "github.com/TUM-Dev/gocast/api_v2/services"
	"github.com/TUM-Dev/gocast/api_v2/protobuf"
	"gorm.io/gorm"
	"net/http"
)

// GetPublicCourses retrieves the public courses based on the context and request.
// It filters the courses by year, term, and query if they are specified in the request.
// It returns a GetPublicCoursesResponse or an error if one occurs.
func (a *API) GetPublicCourses(ctx context.Context, req *protobuf.GetPublicCoursesRequest) (*protobuf.GetPublicCoursesResponse, error) {
	a.log.Info("GetPublicCourses")
	courses, err := s.FetchCourses(a.db, req)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	resp := make([]*protobuf.Course, len(courses))

    for i, course := range courses {
		resp[i] = h.ParseCourseToProto(course)
    }

	return &protobuf.GetPublicCoursesResponse{
		Courses: resp,
	}, nil
}

// GetSemesters retrieves all distinct semesters stored and the current semester.
// It returns a GetSemestersResponse or an error if one occurs.
func (a *API) GetSemesters(context.Context, *protobuf.GetSemestersRequest) (*protobuf.GetSemestersResponse, error) {
	a.log.Info("GetSemesters")
	semesters, err := s.FetchSemesters(a.db)
    if err != nil {
        return nil, err
    }

    resp := make([]*protobuf.Semester, len(semesters))

    for i, semester := range semesters {
        resp[i] = h.ParseSemesterToProto(semester)
    }

	currentSemester, err := s.FetchCurrentSemester(a.db)
    if err != nil {
        return nil, err
    }

    return &protobuf.GetSemestersResponse{
        Current: h.ParseSemesterToProto(currentSemester), 
        Semesters: resp,
    }, nil
}