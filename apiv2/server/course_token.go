package apiv2

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"google.golang.org/protobuf/types/known/emptypb"

	e "github.com/TUM-Dev/gocast/apiv2/errors"
	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/model"
)

// The course token is the credential: it reaches a lecturer by email when their
// course is imported, before they have an account, so these answer anonymous callers
// and name no one. Whoever is signed in is written to the audit entry.

// courseByToken finds the course a token belongs to, soft-deleted or not.
func (a *API) courseByToken(token string) (model.Course, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return model.Course{}, e.WithStatus(http.StatusBadRequest, errors.New("token is required"))
	}
	course, err := a.dao.CoursesDao.GetCourseByToken(token)
	if err != nil {
		return model.Course{}, e.WithStatus(http.StatusNotFound, errors.New("no course has this token"))
	}
	return course, nil
}

// GetCourseByToken names the course a token opens, for the page to show before asking.
func (a *API) GetCourseByToken(ctx context.Context, req *protobuf.CourseTokenRequest) (*protobuf.CourseByToken, error) {
	course, err := a.courseByToken(req.GetToken())
	if err != nil {
		return nil, err
	}
	return &protobuf.CourseByToken{
		Id:       uint32(course.ID),
		Name:     course.Name,
		Slug:     course.Slug,
		Year:     uint32(course.Year),
		Term:     course.TeachingTerm,
		OptedOut: course.DeletedAt.Valid,
	}, nil
}

// OptInCourseByToken switches an imported course on: visible to anyone signed in, with
// recordings, and no longer deleted if it was opted out before.
func (a *API) OptInCourseByToken(ctx context.Context, req *protobuf.CourseTokenRequest) (*emptypb.Empty, error) {
	course, err := a.courseByToken(req.GetToken())
	if err != nil {
		return nil, err
	}
	if err := a.dao.CoursesDao.UnDeleteCourse(ctx, course); err != nil {
		a.log.Error("can't opt a course in", "err", err, "course", course.ID)
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	// v1 set these on the loaded course and then only cleared deleted_at, so they
	// never reached the database; the import had already left most courses this way.
	course.VODEnabled = true
	course.Visibility = "loggedin"
	if err := a.dao.CoursesDao.UpdateCourseColumns(ctx, course, "vod_enabled", "visibility"); err != nil {
		a.log.Error("can't save an opted-in course", "err", err, "course", course.ID)
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	a.audit(ctx, model.AuditCourseCreate, fmt.Sprintf("opted in by token, %s:'%s'", course.Name, course.Slug))
	return &emptypb.Empty{}, nil
}

// OptOutCourseByToken deletes an imported course the lecturer does not want streamed.
func (a *API) OptOutCourseByToken(ctx context.Context, req *protobuf.CourseTokenRequest) (*emptypb.Empty, error) {
	course, err := a.courseByToken(req.GetToken())
	if err != nil {
		return nil, err
	}
	if course.DeletedAt.Valid {
		// Already out; deleting its lectures again would only log errors.
		return &emptypb.Empty{}, nil
	}
	a.audit(ctx, model.AuditCourseDelete, fmt.Sprintf("opted out by token, '%s' (%d, %s)[%d]", course.Name, course.Year, course.TeachingTerm, course.ID))
	a.dao.CoursesDao.DeleteCourse(course)
	return &emptypb.Empty{}, nil
}
