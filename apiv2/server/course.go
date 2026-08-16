// Package apiv2 provides API endpoints for the application.
package apiv2

import (
	"context"
	"errors"
	"net/http"

	"google.golang.org/protobuf/types/known/emptypb"
	"gorm.io/gorm"

	e "github.com/TUM-Dev/gocast/apiv2/errors"
	h "github.com/TUM-Dev/gocast/apiv2/helpers"
	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/apiv2/visibility"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools/tum"
)

// GetLiveCourses retrieves the currently live courses and their streams.
func (a *API) GetLiveCourses(ctx context.Context, req *emptypb.Empty) (*protobuf.GetLiveCoursesResponse, error) {
	streams, err := a.dao.GetCurrentLive(ctx)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, e.WithStatus(http.StatusNotFound, err)
	}

	user, _ := a.getCurrent(ctx) // ignore error as endpoint can also be used by logged-out users

	resp := make([]*protobuf.CourseStream, 0)

	// GetCourseById is uncached and preloads four relations, and a course with
	// several streams live at once would ask for it on every pass.
	courses := make(map[uint]model.Course)

	for _, stream := range streams {
		courseForLiveStream, seen := courses[stream.CourseID]
		if !seen {
			courseForLiveStream, _ = a.dao.GetCourseById(ctx, stream.CourseID)
			courses[stream.CourseID] = courseForLiveStream
		}

		if !visibility.Listed(user, courseForLiveStream) {
			continue
		}
		if !visibility.StreamVisible(user, courseForLiveStream, stream) {
			continue
		}

		var lectureHall *model.LectureHall
		if stream.LectureHallID != 0 {
			lh, err := a.dao.LectureHallsDao.GetLectureHallByID(stream.LectureHallID)
			if err != nil {
				a.log.Error("Could not get Lecture Hall ID", "err", err)
			} else {
				lectureHall = &lh
			}
		}

		// viewers := uint(0)
		// for sID, sessions := range sessionsMap {
		// 	if sID == stream.ID {
		// 		viewers = uint(len(sessions))
		// 	}
		// }

		resp = append(resp, &protobuf.CourseStream{
			Course:      h.ParseCourseToProto(courseForLiveStream, user),
			Stream:      h.ParseStreamToProto(stream, courseForLiveStream, user),
			LectureHall: h.ParseLectureHallToProto(lectureHall),
			// Viewers:     viewers,
		})
	}

	return &protobuf.GetLiveCoursesResponse{LiveCourses: resp}, nil
}

// GetPublicCourses retrieves the public courses for a given semester.
func (a *API) GetPublicCourses(ctx context.Context, req *protobuf.GetPublicCoursesRequest) (*protobuf.GetPublicCoursesResponse, error) {
	user, _ := a.getCurrent(ctx) // ignore error as endpoint can also be used by logged-out users

	year, term := tum.GetCurrentSemester()
	if req.Year != 0 {
		year = int(req.Year)
	}
	if req.Term != "" {
		term = req.Term
	}

	var courses []model.Course

	var err error
	if user != nil {
		courses, err = a.dao.GetPublicAndLoggedInCourses(year, term)
	} else {
		courses, err = a.dao.GetPublicCourses(year, term)
	}
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	resp := make([]*protobuf.Course, len(courses))
	for i, course := range courses {
		resp[i] = h.ParseCourseToProto(course, user)
	}

	return &protobuf.GetPublicCoursesResponse{Courses: resp}, nil
}

// GetCourseBySlug retrieves a course by its slug, year, and term.
func (a *API) GetCourseBySlug(ctx context.Context, req *protobuf.GetCourseBySlugRequest) (*protobuf.GetCourseBySlugResponse, error) {
	user, _ := a.getCurrent(ctx) // ignore error as endpoint can also be used by logged-out users

	if req.Slug == "" {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("slug must not be empty"))
	}

	year, term := tum.GetCurrentSemester()
	if req.Year != 0 {
		year = int(req.Year)
	}
	if req.Term != "" {
		term = req.Term
	}

	course, err := a.dao.GetCourseBySlugYearAndTerm(ctx, req.Slug, term, year)
	if err != nil {
		return nil, e.FromGorm(err, "can't find course")
	}

	// Reachable rather than Listed: a hidden course is unlisted, not private.
	if !visibility.Reachable(user, course) {
		return nil, e.WithStatus(http.StatusUnauthorized, errors.New("unauthorized"))
	}

	visible := visibility.VisibleStreams(user, course)
	streams := make([]*protobuf.Stream, 0, len(visible))
	for _, stream := range visible {
		streams = append(streams, h.ParseStreamToProto(stream, course, user))
	}

	courseDTO := h.ParseCourseToProto(course, user)
	courseDTO.Streams = streams

	return &protobuf.GetCourseBySlugResponse{Course: courseDTO}, nil
}

// GetUserCourses retrieves the courses for a user for a given semester.
func (a *API) GetUserCourses(ctx context.Context, req *protobuf.GetUserCoursesRequest) (*protobuf.GetUserCoursesResponse, error) {
	user, err := a.getCurrent(ctx)
	if err != nil {
		return nil, e.WithStatus(http.StatusUnauthorized, err)
	}

	year, term := tum.GetCurrentSemester()
	if req.Year != 0 {
		year = int(req.Year)
	}
	if req.Term != "" {
		term = req.Term
	}

	var courses []model.Course

	switch user.Role {
	case model.AdminType:
		courses = a.dao.GetAllCoursesForSemester(ctx, year, term)
	case model.LecturerType:
		courses = user.CoursesForSemester(year, term)
		coursesForLecturer, err := a.dao.GetAdministeredCoursesByUserId(ctx, user.ID, term, year)
		if err == nil {
			courses = append(courses, coursesForLecturer...)
		}
	default:
		courses = user.CoursesForSemester(year, term)
	}
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	resp := make([]*protobuf.Course, len(courses))
	for i, course := range courses {
		resp[i] = h.ParseCourseToProto(course, user)
	}

	return &protobuf.GetUserCoursesResponse{Courses: resp}, nil
}

// GetPinnedCourses retrieves the pinned courses for a user.
func (a *API) GetPinnedCourses(ctx context.Context, req *emptypb.Empty) (*protobuf.GetPinnedCoursesResponse, error) {
	user, err := a.getCurrent(ctx)
	if err != nil {
		return nil, e.WithStatus(http.StatusUnauthorized, err)
	}

	pinnedCourses := user.PinnedCourses
	resp := make([]*protobuf.Course, len(pinnedCourses))
	for i, course := range pinnedCourses {
		resp[i] = h.ParseCourseToProto(course, user)
	}

	return &protobuf.GetPinnedCoursesResponse{Courses: resp}, nil
}

// GetPinForCourse checks if the user has pinned the course.
func (a *API) GetPinForCourse(ctx context.Context, req *protobuf.GetPinForCourseRequest) (*protobuf.GetPinForCourseResponse, error) {
	user, err := a.getCurrent(ctx)
	if err != nil {
		return nil, e.WithStatus(http.StatusUnauthorized, err)
	}

	has, err := a.dao.UsersDao.HasPinnedCourse(*user, uint(req.CourseId))
	if err != nil {
		a.log.Error("can't retrieve course", "err", err)
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	return &protobuf.GetPinForCourseResponse{Has: has}, nil
}

// PinCourse pins or unpins a course for the user.
func (a *API) PinCourse(ctx context.Context, req *protobuf.PinCourseRequest) (*protobuf.PinCourseResponse, error) {
	user, err := a.getCurrent(ctx)
	if err != nil {
		return nil, e.WithStatus(http.StatusUnauthorized, err)
	}

	course, err := a.dao.GetCourseById(ctx, uint(req.CourseId))
	if err != nil {
		return nil, e.WithStatus(http.StatusBadRequest, err)
	}

	err = a.dao.UsersDao.PinCourse(*user, course, req.Pin)
	if err != nil {
		a.log.Error("can't update user", "err", err)
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	return &protobuf.PinCourseResponse{Message: "Course pin status updated successfully."}, nil
}
