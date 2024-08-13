package tools

import (
	"fmt"
	"github.com/meilisearch/meilisearch-go"
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

/*
func Search(q string, searchType int, filter string) *meilisearch.SearchResponse {

	bit_operator := 1
	var _ []meilisearch.SearchRequest

	for i := 0; i < 4; i++ {
		switch searchType & bit_operator {
		case 0:
			continue
		case 1:
			// add Subtitles Request
		case 2:
			// add Other request
		}
		bit_operator <<= 1
	}

	//multisearch request
}*/

func SearchCourses(q string, year int, t string, courseIdFilter string) *meilisearch.SearchResponse {
	c, err := Cfg.GetMeiliClient()
	if err != nil {
		return nil
	}

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
