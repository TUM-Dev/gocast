package api

import (
	"github.com/gin-gonic/gin"
	"github.com/joschahenningsen/TUM-Live/dao"
	log "github.com/sirupsen/logrus"
	"net/http"
	"strconv"
)

type searchResponse struct {
	StreamIds []uint
}

type Searchable interface {
	Search(string, uint) ([]uint, error)
}

func configGinSearchRouter(router *gin.Engine, daoWrapper dao.DaoWrapper) {
	streamsSearch := router.Group("/api/search/streams")
	streamsSearch.GET("/sections", search(daoWrapper.VideoSectionDao))
	streamsSearch.GET("/streams", search(daoWrapper.StreamsDao))
	streamsSearch.GET("/chats", search(daoWrapper.ChatDao))
}

func search(searchable Searchable) gin.HandlerFunc {
	return func(c *gin.Context) {
		q := c.Query("q")
		if q == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, "query parameter 'q' missing")
			return
		}

		courseIdS := c.Query("courseId")
		if courseIdS == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, "query parameter 'courseId' missing")
			return
		}

		courseId, err := strconv.Atoi(courseIdS)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, "can not parse courseId")
			return
		}

		streamIDs, err := searchable.Search(q, uint(courseId))
		if err != nil {
			log.WithError(err).Error("could not perform search")
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		c.JSON(http.StatusOK, searchResponse{streamIDs})
	}
}
