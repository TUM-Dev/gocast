package apiv2

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/TUM-Dev/gocast/api"
	e "github.com/TUM-Dev/gocast/apiv2/errors"
	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/model"
)

// Search is what the search RPC runs on; api.SearchCatalog in production, which
// carries v1's filters so both APIs find the same things.
type Search interface {
	Search(ctx context.Context, user *model.User, query string, limit int64, scope api.SearchScope) (api.SearchHits, error)
}

// WithSearch gives the API its search. Without one, searching answers 503.
func WithSearch(s Search) Option {
	return func(a *API) { a.search = s }
}

const (
	searchDefaultLimit = 10
	searchMaxLimit     = 100
)

// Search finds courses, lectures and subtitle lines the caller may see.
func (a *API) Search(ctx context.Context, req *protobuf.SearchRequest) (*protobuf.SearchResponse, error) {
	if a.search == nil {
		return nil, e.WithStatus(http.StatusServiceUnavailable, errors.New("search is not available"))
	}
	user, err := a.currentOrAnonymous(ctx)
	if err != nil {
		return nil, err
	}

	query := strings.TrimSpace(req.GetQuery())
	if query == "" {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("query is required"))
	}
	limit := int64(req.GetLimit())
	if limit <= 0 {
		limit = searchDefaultLimit
	}
	if limit > searchMaxLimit {
		limit = searchMaxLimit
	}

	scope := api.SearchScope{CoursesOnly: req.GetCoursesOnly()}
	if scope.Semesters, err = api.ParseSemesterKeys(req.GetSemesters()); err != nil {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("semesters are written like 2022S"))
	}
	if (req.GetFirstSemester() == "") != (req.GetLastSemester() == "") {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("first_semester and last_semester go together"))
	}
	if req.GetFirstSemester() != "" {
		bounds, err := api.ParseSemesterKeys([]string{req.GetFirstSemester(), req.GetLastSemester()})
		if err != nil {
			return nil, e.WithStatus(http.StatusBadRequest, errors.New("semesters are written like 2022S"))
		}
		scope.FirstSemester, scope.LastSemester = &bounds[0], &bounds[1]
	}
	if len(req.GetCourses()) > api.FilterMaxCoursesCount {
		return nil, e.WithStatus(http.StatusBadRequest, errors.New("at most three courses can be searched at once"))
	}
	courses, err := api.ParseCourseKeys(ctx, a.dao, req.GetCourses())
	if errors.Is(err, api.ErrUnknownCourse) {
		return nil, e.WithStatus(http.StatusNotFound, errors.New("no such course"))
	}
	if err != nil {
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}
	for _, course := range courses {
		// Someone who may not watch it is told it does not exist, like a missing one:
		// a hidden course's existence is not to be confirmed by its slug.
		if !user.IsEligibleToWatchCourse(course) {
			return nil, e.WithStatus(http.StatusNotFound, errors.New("no such course"))
		}
	}
	scope.Courses = courses

	hits, err := a.search.Search(ctx, user, query, limit, scope)
	if errors.Is(err, api.ErrSearchUnavailable) {
		return nil, e.WithStatus(http.StatusServiceUnavailable, err)
	}
	if err != nil {
		a.log.Error("search failed", "err", err)
		return nil, e.WithStatus(http.StatusInternalServerError, err)
	}

	resp := &protobuf.SearchResponse{}
	for _, c := range hits.Courses {
		resp.Courses = append(resp.Courses, &protobuf.SearchCourseHit{
			Name: c.Name, Slug: c.Slug, Year: uint32(c.Year), Term: c.TeachingTerm,
		})
	}
	for _, s := range hits.Streams {
		resp.Streams = append(resp.Streams, &protobuf.SearchStreamHit{
			Id: uint32(s.ID), Name: s.Name, Description: s.Description,
			CourseName: s.CourseName, CourseSlug: s.CourseSlug, Year: uint32(s.Year), Term: s.TeachingTerm,
		})
	}
	for _, s := range hits.Subtitles {
		resp.Subtitles = append(resp.Subtitles, &protobuf.SearchSubtitleHit{
			StreamId: uint32(s.StreamID), Timestamp: s.Timestamp,
			TextPrev: s.TextPrev, Text: s.Text, TextNext: s.TextNext,
			StreamName: s.StreamName, StreamStart: timestamppb.New(s.StreamStartTime), StreamEnd: timestamppb.New(s.StreamEndTime),
			CourseName: s.CourseName, CourseSlug: s.CourseSlug, Year: uint32(s.CourseYear), Term: s.CourseTeachingTerm,
		})
	}
	return resp, nil
}
