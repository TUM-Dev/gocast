package tum

import (
	"fmt"

	meilisearch "github.com/meilisearch/meilisearch-go"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/model/search"
	"github.com/TUM-Dev/gocast/tools"
)

// Catalog is TUMOnline as the course creation page uses it: the course list
// PrefetchCourses copies into the search backend, and the lectures and enrolments
// fetched for a course once it exists.
type Catalog struct {
	Dao dao.DaoWrapper
}

// SearchCourses searches the courses PrefetchCourses stored, as v1's searchCourse did.
func (c Catalog) SearchCourses(q string, limit int64) ([]search.PrefetchedCourse, error) {
	client, err := tools.Cfg.GetMeiliClient()
	if err != nil {
		return nil, fmt.Errorf("connecting to the search backend: %w", err)
	}

	resp, err := client.Index("PREFETCHED_COURSES").Search(q, &meilisearch.SearchRequest{Limit: limit})
	if err != nil {
		return nil, fmt.Errorf("searching prefetched courses: %w", err)
	}

	var res []search.PrefetchedCourse
	if err := resp.Hits.Decode(&res); err != nil {
		return nil, fmt.Errorf("reading prefetched courses: %w", err)
	}
	return res, nil
}

// SyncCourse fetches a new course's lectures and enrolments from TUMOnline in the
// background, and refreshes the courses it knows of, as v1's createCourse did.
func (c Catalog) SyncCourse(course model.Course) {
	courses := []model.Course{course}
	go GetEventsForCourses(courses, c.Dao)
	go FindStudentsForCourses(courses, c.Dao)
	go FetchCourses(c.Dao)
}
