// Package api_v2 provides API endpoints for the application.
package api_v2

import (
	"context"
	"errors"
	"net/http"

	e "github.com/TUM-Dev/gocast/api_v2/errors"
	h "github.com/TUM-Dev/gocast/api_v2/helpers"
	"github.com/TUM-Dev/gocast/api_v2/protobuf"
	s "github.com/TUM-Dev/gocast/api_v2/services"
	"gorm.io/gorm"
)

// GetPublicCourses retrieves the public courses based on the context and request.
// It filters the courses by year, term, and query if they are specified in the request.
// It returns a GetPublicCoursesResponse or an error if one occurs.
func (a *API) GetPublicCourses(ctx context.Context, req *protobuf.GetPublicCoursesRequest) (*protobuf.GetPublicCoursesResponse, error) {
	a.log.Info("GetPublicCourses")

	uID, err := a.getCurrentID(ctx)
	if err != nil && err.Error() != "missing cookie header" {
		return nil, e.WithStatus(http.StatusUnauthorized, err)
	}

	courses, err := s.FetchCourses(a.db, req, &uID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	resp := make([]*protobuf.Course, len(courses))

	for i, c := range courses {
		resp[i] = h.ParseCourseToProto(c)
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
		Current:   h.ParseSemesterToProto(currentSemester),
		Semesters: resp,
	}, nil
}

func (a *API) GetCourseStreams(ctx context.Context, req *protobuf.GetCourseStreamsRequest) (*protobuf.GetCourseStreamsResponse, error) {
	a.log.Info("GetCourseStreams")

	if req.CourseID == 0 {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("course id must not be empty"))
	}

	uID, err := a.getCurrentID(ctx)
	if err != nil && err.Error() != "missing cookie header" {
		return nil, e.WithStatus(http.StatusUnauthorized, err)
	}

	c, err := h.CheckAuthorized(a.db, uID, uint(req.CourseID))
	if err != nil {
		return nil, err
	}

	streams, err := s.GetStreamsByCourseID(a.db, uint(req.CourseID))
	if err != nil {
		return nil, err
	}

	resp := make([]*protobuf.Stream, len(streams))
	for i, stream := range streams {
		downloads, err := h.SignStream(stream, c, uID)
		if err != nil {
			return nil, err
		}

		s, err := h.ParseStreamToProto(stream, downloads)
		if err != nil {
			return nil, err
		}
		resp[i] = s
	}

	return &protobuf.GetCourseStreamsResponse{Streams: resp}, nil
}
