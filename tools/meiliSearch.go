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
