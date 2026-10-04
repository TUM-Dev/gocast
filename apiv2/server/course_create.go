package apiv2

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"gorm.io/gorm"

	e "github.com/TUM-Dev/gocast/apiv2/errors"
	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/model/search"
)

// TUMOnline is what course creation needs of TUMOnline: its course list to fill a new
// course in from, and the lectures and enrolments to fetch once one exists. Declared
// here so the tests can stand in a fake; tum.Catalog satisfies it.
type TUMOnline interface {
	SearchCourses(q string, limit int64) ([]search.PrefetchedCourse, error)
	// SyncCourse starts fetching and returns; the work outlives the request.
	SyncCourse(course model.Course)
}

// WithTUMOnline gives the API access to TUMOnline. Without it the course search
// refuses and a new course is created without its lectures and enrolments.
func WithTUMOnline(t TUMOnline) Option {
	return func(a *API) { a.tumOnline = t }
}

// tumOnlineSearchLimit is how many matches the course search returns, as v1's did.
const tumOnlineSearchLimit = 10

var (
	errNoTUMOnline = errors.New("course search is not available")

	// What the old creation form let through. Looser than info pages' slugPattern
	// because that form allowed capitals and underscores, so courses may have them.
	courseSlugPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
)

// SearchTumOnlineCourses replaces v1's searchCourse. Gated on the lecture permission
// by its policy.
func (a *API) SearchTumOnlineCourses(ctx context.Context, req *protobuf.SearchTumOnlineCoursesRequest) (*protobuf.SearchTumOnlineCoursesResponse, error) {
	if a.tumOnline == nil {
		return nil, e.WithStatus(http.StatusServiceUnavailable, errNoTUMOnline)
	}

	found, err := a.tumOnline.SearchCourses(req.GetQ(), tumOnlineSearchLimit)
	if err != nil {
		// The backend being down is the usual cause, and worth retrying.
		a.log.Error("can't search TUMOnline courses", "err", err)
		return nil, e.WithStatus(http.StatusServiceUnavailable, errNoTUMOnline)
	}

	courses := make([]*protobuf.TumOnlineCourse, 0, len(found))
	for _, c := range found {
		courses = append(courses, &protobuf.TumOnlineCourse{
			TumOnlineId: c.CourseID,
			Name:        c.Name,
			Year:        int32(c.Year),
			Term:        c.Term,
		})
	}
	return &protobuf.SearchTumOnlineCoursesResponse{Courses: courses}, nil
}

// CreateCourse replaces v1's createCourse, with the defaults the old form sent. Gated
// on the lecture permission by its policy.
func (a *API) CreateCourse(ctx context.Context, req *protobuf.CreateCourseRequest) (*protobuf.CreateCourseResponse, error) {
	user, err := a.getCurrent(ctx)
	if err != nil {
		return nil, e.WithStatus(http.StatusUnauthorized, err)
	}

	name := strings.TrimSpace(req.GetName())
	if name == "" {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("the course needs a name"))
	}
	slug := strings.TrimSpace(req.GetSlug())
	if !courseSlugPattern.MatchString(slug) {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("the slug may only contain letters, digits, '-' and '_'"))
	}
	// v1 accepted any four-digit year in the teaching term it parsed.
	if req.GetYear() < 1000 || req.GetYear() > 9999 {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("the year must have four digits"))
	}
	term := req.GetTerm()
	if term != "W" && term != "S" {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("the term must be W or S"))
	}
	language := sql.NullString{}
	switch req.GetLanguage() {
	case "de", "en":
		language = sql.NullString{String: req.GetLanguage(), Valid: true}
	case "":
	default:
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("the language must be de, en or empty"))
	}

	_, err = a.dao.CoursesDao.GetCourseBySlugYearAndTerm(ctx, slug, term, int(req.GetYear()))
	switch {
	case err == nil:
		return nil, e.WithStatus(http.StatusConflict, fmt.Errorf("this semester already has a course with the slug %q", slug))
	case !errors.Is(err, gorm.ErrRecordNotFound):
		// v1 read any error here as "free"; a failed lookup is not an answer.
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	course := model.Course{
		UserID:              user.ID,
		Name:                name,
		Slug:                slug,
		Year:                int(req.GetYear()),
		TeachingTerm:        term,
		TUMOnlineIdentifier: strings.TrimSpace(req.GetTumOnlineId()),
		// The old form's defaults; everything else is set on the course's page.
		Visibility:       "loggedin",
		VODEnabled:       true,
		DownloadsEnabled: false,
		ChatEnabled:      false,
		Streams:          []model.Stream{},
		Language:         language,
	}
	// An administrator of every course already administers this one.
	if !user.Can(model.PermAdministerAllCourses) {
		course.Admins = []model.User{*user}
	}

	if err := a.dao.CoursesDao.CreateCourse(ctx, &course, true); err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	// After the course exists, unlike v1, so a failed creation leaves no audit entry.
	if err := a.dao.AuditDao.Create(&model.Audit{
		User:    user,
		Message: fmt.Sprintf("%s:'%s' (%d, %s)", course.Slug, course.Name, course.Year, course.TeachingTerm),
		Type:    model.AuditCourseCreate,
	}); err != nil {
		a.log.Error("can't audit course creation", "err", err)
	}

	if a.tumOnline != nil {
		a.tumOnline.SyncCourse(course)
	}

	return &protobuf.CreateCourseResponse{CourseId: uint32(course.ID)}, nil
}
