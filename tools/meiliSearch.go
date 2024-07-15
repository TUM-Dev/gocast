package tools

import (
	"fmt"
	"github.com/meilisearch/meilisearch-go"
	"strconv"
	"strings"
)

func SearchSubtitles(q string, streamID uint) *meilisearch.SearchResponse {
	c, err := Cfg.GetMeiliClient()
	if err != nil {
		return nil
	}
	response, err := c.Index("SUBTITLES").Search(q, &meilisearch.SearchRequest{
		Filter: fmt.Sprintf("streamID = %d", streamID),
		Limit:  10,
	})
	if err != nil {
		logger.Error("could not search meili", "err", err)
		return nil
	}
	return response
}

func SearchCourses(q string, year int, t string, searchableCourseIDs *[]uint) *meilisearch.SearchResponse {
	c, err := Cfg.GetMeiliClient()
	if err != nil {
		return nil
	}
	if t != "W" && t != "S" {
		return nil
	}

	courseIdFilter := courseIdFilter(searchableCourseIDs)
	response, err := c.Index("COURSES").Search(q, &meilisearch.SearchRequest{
		Filter: fmt.Sprintf("year = %d AND teachingTerm = %s AND ID IN %s", year, t, courseIdFilter),
		Limit:  10,
	})

	if err != nil {
		logger.Error("could not search courses in meili", "err", err)
		return nil
	}
	return response
}

// returns a string conforming to Meili Search Filters Format containing each courseId passed onto the function
func courseIdFilter(searchableCourseIDs *[]uint) string {
	var courseIDsAsStringArray []string
	courseIDsAsStringArray = make([]string, len(*searchableCourseIDs))
	for i, courseID := range *searchableCourseIDs {
		courseIDsAsStringArray[i] = strconv.FormatUint(uint64(courseID), 10)
	}
	courseIdFilter := "[" + strings.Join(courseIDsAsStringArray, ", ") + "]"
	return courseIdFilter
}
