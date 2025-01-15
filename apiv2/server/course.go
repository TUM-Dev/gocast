// Package apiv2 provides API endpoints for the application.
package apiv2

import (
	"context"
	"errors"
	"net/http"

	e "github.com/TUM-Dev/gocast/apiv2/errors"
	h "github.com/TUM-Dev/gocast/apiv2/helpers"
	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools/tum"
	"google.golang.org/protobuf/types/known/emptypb"
	"gorm.io/gorm"
)

// GetLiveStreams retrieves the currently live streams.
func (a *API) GetLiveStreams(ctx context.Context, req *emptypb.Empty) (*protobuf.GetLiveStreamsResponse, error) {
	a.log.Info("GetLiveStreams")

	streams, err := a.dao.GetCurrentLive(context.Background())
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, e.WithStatus(http.StatusNotFound, err)
	}

	user := &model.User{}
	resp := make([]*protobuf.CourseStream, 0)

	for _, stream := range streams {
		courseForLiveStream, _ := a.dao.GetCourseById(context.Background(), stream.CourseID)

		// only show streams for logged-in users if they are logged in
		if courseForLiveStream.Visibility == "loggedin" && user == nil {
			continue
		}
		// only show "enrolled" streams to users which are enrolled or admins
		if courseForLiveStream.Visibility == "enrolled" {
			if !user.IsAllowedToWatchPrivateCourse(courseForLiveStream) {
				continue
			}
		}
		// Only show hidden streams to course admins
		if courseForLiveStream.Visibility == "hidden" && (user == nil || !user.IsAdminOfCourse(courseForLiveStream)) {
			continue
		}
		// Only show private streams to course admins
		if stream.Private && (user == nil || !user.IsAdminOfCourse(courseForLiveStream)) {
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
			Stream:      h.ParseStreamToProto(stream, nil),
			LectureHall: h.ParseLectureHallToProto(lectureHall),
			// Viewers:     viewers,
		})
	}

	return &protobuf.GetLiveStreamsResponse{Streams: resp}, nil
}

// GetPublicCourses retrieves the public courses for a given semester.
func (a *API) GetPublicCourses(ctx context.Context, req *protobuf.GetPublicCoursesRequest) (*protobuf.GetPublicCoursesResponse, error) {
	a.log.Info("GetPublicCourses")

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
	a.log.Info("GetCourseBySlug")

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

	course, err := a.dao.GetCourseBySlugYearAndTerm(context.Background(), req.Slug, term, year)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, e.WithStatus(http.StatusNotFound, errors.New("can't find course"))
		}
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	if (course.IsLoggedIn() && user == nil) || (course.IsEnrolled() && !user.IsAllowedToWatchPrivateCourse(course)) {
		return nil, e.WithStatus(http.StatusUnauthorized, errors.New("unauthorized"))
	}

	streams := make([]*protobuf.Stream, len(course.Streams))
	for i, stream := range course.Streams {
		if !stream.Private || user.IsAdminOfCourse(course) {
			streams[i] = h.ParseStreamToProto(stream, nil)
		}
	}

	courseDTO := h.ParseCourseToProto(course, user)
	courseDTO.Streams = streams

	return &protobuf.GetCourseBySlugResponse{Course: courseDTO}, nil
}

// GetUserCourses retrieves the courses for a user for a given semester.
func (a *API) GetUserCourses(ctx context.Context, req *protobuf.GetUserCoursesRequest) (*protobuf.GetUserCoursesResponse, error) {
	a.log.Info("GetUserCourses")

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
		courses = a.dao.GetAllCoursesForSemester(year, term, context.Background())
	case model.LecturerType:
		courses = user.CoursesForSemester(year, term, context.Background())
		coursesForLecturer, err := a.dao.GetAdministeredCoursesByUserId(context.Background(), user.ID, term, year)
		if err == nil {
			courses = append(courses, coursesForLecturer...)
		}
	default:
		courses = user.CoursesForSemester(year, term, context.Background())
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
	a.log.Info("GetPinnedCourses")

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
	a.log.Info("GetPinForCourse")

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
	a.log.Info("PinCourse")

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
