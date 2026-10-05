package api

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/meilisearch/meilisearch-go"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
)

// SearchCatalog is the search the v2 API exposes. It runs on v1's filter building
// and post-filtering above, so both APIs agree on who may find what for as long as
// both exist; v1's handlers go with the search page, the filters stay.
type SearchCatalog struct {
	Dao   dao.DaoWrapper
	Meili tools.MeiliSearchInterface
}

// SearchScope narrows a search the way v1's URL parameters did: named semesters, a
// range of them, or up to FilterMaxCoursesCount courses the caller may watch.
type SearchScope struct {
	Semesters []model.Semester
	// A range; both or neither. Ignored when Semesters is set.
	FirstSemester, LastSemester *model.Semester
	// Searching inside courses looks at their lectures and subtitles instead of at
	// courses, hidden ones included: naming a course is opting into it. The caller
	// has checked the user may watch each.
	Courses []model.Course
	// Only courses, never lectures (v1's /api/search/courses).
	CoursesOnly bool
}

// SearchHits is what a search found, already reduced to what the user may see.
type SearchHits struct {
	Courses   []SearchCourseDTO
	Streams   []SearchStreamDTO
	Subtitles []SearchSubtitlesDTO
}

// ErrSearchUnavailable means Meilisearch did not answer; locally that is the norm.
var ErrSearchUnavailable = errors.New("search is not available")

// Search runs the query for user (nil for an anonymous caller) and keeps at most
// limit hits of each kind.
func (s SearchCatalog) Search(ctx context.Context, user *model.User, query string, limit int64, scope SearchScope) (SearchHits, error) {
	if s.Meili == nil {
		return SearchHits{}, ErrSearchUnavailable
	}

	var res *meilisearch.MultiSearchResponse
	inCourses := len(scope.Courses) > 0
	switch {
	case inCourses:
		// The gin context the filters take is never read; v1 passes it along by habit.
		res = s.Meili.Search(query, limit, 3, "",
			meiliStreamFilter(nil, user, model.Semester{}, scope.Courses),
			meiliSubtitleFilter(user, scope.Courses))
	case len(scope.Semesters) > 0 || (scope.FirstSemester != nil && scope.LastSemester != nil):
		var first, last model.Semester
		if scope.FirstSemester != nil && scope.LastSemester != nil {
			first, last = *scope.FirstSemester, *scope.LastSemester
		}
		single, semester := determineSingleSemester(first, last, scope.Semesters)
		if !scope.CoursesOnly && single {
			res = s.Meili.Search(query, limit, 6,
				meiliCourseFilter(nil, user, semester, semester, nil),
				meiliStreamFilter(nil, user, semester, nil), "")
		} else {
			res = s.Meili.Search(query, limit, 4, meiliCourseFilter(nil, user, first, last, scope.Semesters), "", "")
		}
	default:
		// Every semester there is, as v1 did without parameters.
		res = s.Meili.Search(query, limit, 4,
			meiliCourseFilter(nil, user, model.Semester{TeachingTerm: "S"}, model.Semester{TeachingTerm: "W", Year: 3000}, nil), "", "")
	}
	if res == nil {
		return SearchHits{}, ErrSearchUnavailable
	}

	checkAndFillResponse(ctx, user, limit, s.Dao, res, inCourses)

	var hits SearchHits
	for _, r := range res.Results {
		var err error
		switch r.IndexUID {
		case "COURSES":
			err = hitsInto(r.Hits, &hits.Courses)
		case "STREAMS":
			err = hitsInto(r.Hits, &hits.Streams)
		case "SUBTITLES":
			err = hitsInto(r.Hits, &hits.Subtitles)
		}
		if err != nil {
			return SearchHits{}, err
		}
	}
	return hits, nil
}

// hitsInto reads the hits checkAndFillResponse rebuilt from the DTOs back into them.
func hitsInto(hits []meilisearch.Hit, into any) error {
	if len(hits) == 0 {
		return nil
	}
	raw, err := json.Marshal(hits)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, into)
}

// ParseSemesterKeys reads semesters written as v1's URL parameter did: "2022S".
func ParseSemesterKeys(keys []string) ([]model.Semester, error) {
	if len(keys) == 0 {
		return nil, nil
	}
	return parseSemesters(strings.Join(keys, ","))
}

// ErrUnknownCourse is a course key no course matches.
var ErrUnknownCourse = errors.New("unknown course")

// ParseCourseKeys reads courses written as v1's URL parameter did: "<slug><year><term>",
// "brauereiwesen2022S". It answers ErrUnknownCourse for a key no course matches or
// that is malformed, which the caller should treat as a missing course.
func ParseCourseKeys(ctx context.Context, d dao.DaoWrapper, keys []string) ([]model.Course, error) {
	if len(keys) == 0 {
		return nil, nil
	}
	courses, code := parseCourses(ctx, d, strings.Join(keys, ","))
	switch code {
	case 0:
		return courses, nil
	case 1:
		return nil, ErrUnknownCourse
	default:
		return nil, errors.New("could not read the course keys")
	}
}
