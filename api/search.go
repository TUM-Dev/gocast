package api

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/tools"
	"net/http"
	"strconv"
	"time"
)

func configGinSearchRouter(router *gin.Engine, daoWrapper dao.DaoWrapper) {
	routes := searchRoutes{daoWrapper}
	router.GET("/api/search/streams", routes.searchStreams)
	g := router.Group("/api/search/subtitles")
	g.Use(tools.InitStream(daoWrapper))
	g.GET("/:streamID", routes.searchSubtitles)
}

type searchRoutes struct {
	dao.DaoWrapper
}

func (r searchRoutes) searchStreams(c *gin.Context) {
	q, courseIdS := c.Query("q"), c.Query("courseId")
	if q == "" {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "query parameter 'q' missing",
		})
		return
	}

	courseId, err := strconv.Atoi(courseIdS)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "invalid query parameter 'courseId'",
		})
		return
	}

	startTime := time.Now()
	searchResults, err := r.SearchDao.Search(q, uint(courseId))
	endTime := time.Now()
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: fmt.Sprintf("could not perform search: %s", err.Error()),
			Err:           err,
		})
		return
	}

	response := []gin.H{}
	for _, sr := range searchResults {
		sr.Name = sr.GetName()
		response = append(response, gin.H{
			"ID":           sr.ID,
			"name":         sr.Name,
			"friendlyTime": sr.FriendlyTime(),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"duration": endTime.Sub(startTime).Milliseconds(),
		"results":  response,
	})
}

func (r searchRoutes) searchSubtitles(c *gin.Context) {
	s := c.MustGet("TUMLiveContext").(tools.TUMLiveContext).Stream
	q := c.Query("q")
	c.JSON(http.StatusOK, tools.SearchSubtitles(q, s.ID))
}
