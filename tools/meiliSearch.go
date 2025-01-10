package tools

import (
	"fmt"

	"github.com/meilisearch/meilisearch-go"
)

//go:generate mockgen -source=meiliSearch.go -destination ../mock_tools/meiliSearch.go

type MeiliSearchInterface interface {
	SearchSubtitles(q string, streamID uint) *meilisearch.SearchResponse
	Search(q string, limit int64, searchType int, courseFilter string, streamFilter string, subtitleFilter string) *meilisearch.MultiSearchResponse
}

type meiliSearchFunctions struct{}

func NewMeiliSearchFunctions() MeiliSearchInterface {
	return &meiliSearchFunctions{}
}

func (d *meiliSearchFunctions) SearchSubtitles(q string, streamID uint) *meilisearch.SearchResponse {
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

func getCourseWideSubtitleSearchRequest(q string, limit int64, streamFilter string) meilisearch.SearchRequest {
	req := meilisearch.SearchRequest{
		IndexUID:             "SUBTITLES",
		Query:                q,
		Limit:                limit,
		Filter:               streamFilter,
		AttributesToRetrieve: []string{"streamID", "timestamp", "textPrev", "text", "textNext"},
	}
	return req
}

func getStreamsSearchRequest(q string, limit int64, streamFilter string) meilisearch.SearchRequest {
	req := meilisearch.SearchRequest{
		IndexUID:             "STREAMS",
		Query:                q,
		Limit:                limit + 2,
		Filter:               streamFilter,
		AttributesToRetrieve: []string{"ID", "name", "description", "courseName", "year", "semester"},
	}
	return req
}

func getCoursesSearchRequest(q string, limit int64, courseFilter string) meilisearch.SearchRequest {
	req := meilisearch.SearchRequest{
		IndexUID:             "COURSES",
		Query:                q,
		Limit:                limit + 2,
		Filter:               courseFilter,
		AttributesToRetrieve: []string{"name", "slug", "year", "semester"},
	}
	return req
}

// Search passes search requests on to MeiliSearch instance and returns the results
//
// searchType specifies bit-wise which indexes should be searched (lowest bit set to 1: Index SUBTITLES | second-lowest bit set to 1: Index STREAMS | third-lowest bit set to 1: Index COURSES)
func (d *meiliSearchFunctions) Search(q string, limit int64, searchType int, courseFilter string, streamFilter string, subtitleFilter string) *meilisearch.MultiSearchResponse {
	c, err := Cfg.GetMeiliClient()
	if err != nil {
		return nil
	}

	bitOperator := 1
	var reqs []meilisearch.SearchRequest

	for i := 0; i < 4; i++ {
		switch searchType & bitOperator {
		case 0:
			break
		case 1:
			reqs = append(reqs, getCourseWideSubtitleSearchRequest(q, limit, subtitleFilter))
		case 2:
			reqs = append(reqs, getStreamsSearchRequest(q, limit, streamFilter))
		case 4:
			reqs = append(reqs, getCoursesSearchRequest(q, limit, courseFilter))
		}
		bitOperator <<= 1
	}

	// multisearch Request
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
		Filter:               filter,
		Limit:                10,
		AttributesToRetrieve: []string{"name", "slug", "year", "semester"},
	})
	if err != nil {
		logger.Error("could not search courses in meili", "err", err)
		return nil
	}
	return response
}
