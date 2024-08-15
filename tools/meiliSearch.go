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

func getCourseWideSubtitleSearchRequest(q string, limit int, streamFilter string) meilisearch.SearchRequest {
	req := meilisearch.SearchRequest{
		IndexUID: "SUBTITLES",
		Query:    q,
		Limit:    int64(limit) + 2,
		Filter:   streamFilter,
	}
	return req
}

func getStreamsSearchRequest(q string, limit int, streamFilter string) meilisearch.SearchRequest {
	req := meilisearch.SearchRequest{
		IndexUID: "STREAMS",
		Query:    q,
		Limit:    int64(limit) + 2,
		Filter:   streamFilter,
	}
	return req
}

func getCoursesSearchRequest(q string, limit int, courseFilter string) meilisearch.SearchRequest {
	req := meilisearch.SearchRequest{
		IndexUID: "COURSES",
		Query:    q,
		Limit:    int64(limit) + 2,
		Filter:   courseFilter,
	}
	return req
}

func Search(q string, limit int, searchType int, courseFilter string, streamFilter string) *meilisearch.MultiSearchResponse {
	c, err := Cfg.GetMeiliClient()
	if err != nil {
		return nil
	}

	bitOperator := 1
	var reqs []meilisearch.SearchRequest

	for i := 0; i < 4; i++ {
		switch searchType & bitOperator {
		case 0:
			continue
		case 1:
			// add Subtitles Request
			reqs = append(reqs, getCourseWideSubtitleSearchRequest(q, limit, streamFilter))
		case 2:
			reqs = append(reqs, getStreamsSearchRequest(q, limit, streamFilter))
		case 4:
			reqs = append(reqs, getCoursesSearchRequest(q, limit, courseFilter))
		}
		bitOperator <<= 1
	}

	//multisearch Request
	response, err := c.MultiSearch(&meilisearch.MultiSearchRequest{Queries: reqs})
	if err != nil {
		logger.Error("could not search in meili", "err", err)
		return nil
	}
	return response
}

func SearchCourses(q string, filter string) *meilisearch.SearchResponse {
	c, err := Cfg.GetMeiliClient()
	if err != nil {
		return nil
	}

	response, err := c.Index("COURSES").Search(q, &meilisearch.SearchRequest{
		Filter: filter,
		Limit:  10,
	})

	if err != nil {
		logger.Error("could not search courses in meili", "err", err)
		return nil
	}
	print(response.ProcessingTimeMs)
	return response
}
