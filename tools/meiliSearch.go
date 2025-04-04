package tools

import (
	"fmt"

	"github.com/meilisearch/meilisearch-go"
)

//go:generate mockgen -source=meiliSearch.go -destination ../mock_tools/meiliSearch.go

type MeiliSearchInterface interface {
	SearchSubtitles(q string, streamID uint) *meilisearch.SearchResponse
	Search(q string, limit int64, searchType int, courseFilter string, streamFilter string, customStreamFilter string, subtitleFilter string) *MeiliSearchResponseBundle
}

type meiliSearchFunctions struct{}

func NewMeiliSearchFunctions() MeiliSearchInterface {
	return &meiliSearchFunctions{}
}

type MeiliSearchRequestBundle struct {
	SearchRequests               []*meilisearch.SearchRequest
	StreamFederatedSearchRequest *meilisearch.MultiSearchRequest
}

type MeiliSearchResponseBundle struct {
	SearchResponses               []meilisearch.SearchResponse    // all responses of non-federated search requests
	StreamFederatedSearchResponse meilisearch.MultiSearchResponse // response of the federated request containing custom stream titles and normal streams
}

const (
	CourseWideSubtitleSearchType = 1 << iota
	CustomStreamSearchType       // federated search both for custom stream titles and "normal" streams
	StreamSearchType             // const StreamSearchType must follow directly after CustomStreamSearchType for Search function to work as expected
	CourseSearchType
)

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

func getCourseWideSubtitleSearchRequest(q string, limit int64, streamFilter string) *meilisearch.SearchRequest {
	req := meilisearch.SearchRequest{
		IndexUID:             "SUBTITLES",
		Query:                q,
		Limit:                limit,
		Filter:               streamFilter,
		AttributesToRetrieve: []string{"streamID", "timestamp", "textPrev", "text", "textNext"},
	}
	return &req
}

func getStreamsSearchRequest(q string, limit int64, streamFilter string) *meilisearch.SearchRequest {
	req := meilisearch.SearchRequest{
		IndexUID:             "STREAMS",
		Query:                q,
		Filter:               streamFilter,
		AttributesToRetrieve: []string{"streamID", "name", "description", "courseName", "year", "semester"},
	}
	// Federated search fails if Limit is set
	if limit != 0 {
		req.Limit = limit + 2
	}
	return &req
}

func getCustomStreamsSearchRequest(q string, limit int64, customStreamFilter string) *meilisearch.SearchRequest {
	req := meilisearch.SearchRequest{
		IndexUID:             "STREAMSCUSTOMTITLE",
		Query:                q,
		Filter:               customStreamFilter,
		AttributesToRetrieve: []string{"userID", "streamID", "name", "year", "semester"},
	}
	// Federated search fails if Limit is set
	if limit != 0 {
		req.Limit = limit + 2
	}
	return &req
}

func getCoursesSearchRequest(q string, limit int64, courseFilter string) *meilisearch.SearchRequest {
	req := meilisearch.SearchRequest{
		IndexUID:             "COURSES",
		Query:                q,
		Limit:                limit + 2,
		Filter:               courseFilter,
		AttributesToRetrieve: []string{"name", "slug", "year", "semester"},
	}
	return &req
}

// Search passes search requests on to MeiliSearch instance and returns the results
//
// searchType specifies bit-wise which indexes should be searched, defined by CourseWideSubtitleSearchType, CustomStreamSearchType, StreamSearchType, CourseSearchType.
// Both StreamSearchTypes are mutually exclusive
func (d *meiliSearchFunctions) Search(q string, limit int64, searchType int, courseFilter string, streamFilter string, customStreamFilter string, subtitleFilter string) *MeiliSearchResponseBundle {
	c, err := Cfg.GetMeiliClient()
	if err != nil {
		return nil
	}

	bitOperator := 1
	reqs := MeiliSearchRequestBundle{
		SearchRequests:               []*meilisearch.SearchRequest{},
		StreamFederatedSearchRequest: nil,
	}

	for i := 0; i < 4; i++ {
		switch searchType & bitOperator {
		case CourseWideSubtitleSearchType:
			reqs.SearchRequests = append(reqs.SearchRequests, getCourseWideSubtitleSearchRequest(q, limit, subtitleFilter))
		case CustomStreamSearchType:
			reqs.StreamFederatedSearchRequest = &meilisearch.MultiSearchRequest{
				Federation: &meilisearch.MultiSearchFederation{
					Limit: limit,
				},
				Queries: []*meilisearch.SearchRequest{getStreamsSearchRequest(q, 0, streamFilter), getCustomStreamsSearchRequest(q, 0, customStreamFilter)},
			}
			bitOperator <<= 1 // skip appending stream search as non-federated search
		case StreamSearchType:
			reqs.SearchRequests = append(reqs.SearchRequests, getStreamsSearchRequest(q, limit, streamFilter))
		case CourseSearchType:
			reqs.SearchRequests = append(reqs.SearchRequests, getCoursesSearchRequest(q, limit, courseFilter))
		default:
			break
		}
		bitOperator <<= 1
	}

	responses := MeiliSearchResponseBundle{
		SearchResponses:               []meilisearch.SearchResponse{},
		StreamFederatedSearchResponse: meilisearch.MultiSearchResponse{},
	}
	// all non-federated requests bundled into one multisearch request
	res, err := c.MultiSearch(&meilisearch.MultiSearchRequest{Queries: reqs.SearchRequests})
	if err != nil {
		logger.Error("could not search in meili", "err", err)
		return nil
	}
	responses.SearchResponses = res.Results

	if reqs.StreamFederatedSearchRequest != nil {
		res, err = c.MultiSearch(reqs.StreamFederatedSearchRequest)
		if err != nil {
			logger.Error("could not search in meili", "err", err)
			return nil
		}
		responses.StreamFederatedSearchResponse = *res
	}

	return &responses
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
