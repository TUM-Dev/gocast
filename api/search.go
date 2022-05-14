package api

import (
	"github.com/gin-gonic/gin"
	"github.com/joschahenningsen/TUM-Live/dao"
	log "github.com/sirupsen/logrus"
	"net/http"
	"strconv"
)

func configGinSearchRouter(router *gin.Engine, dao dao.VideoSectionDao) {
	routes := searchRoutes{dao}
	api := router.Group("/api/search")
	api.GET("/sections", routes.sections)
	api.GET("/streams", routes.streams)
}

type searchRoutes struct {
	VideoSectionDao dao.VideoSectionDao
}

type searchResponse struct {
	StreamIds []uint
}

func (r searchRoutes) sections(c *gin.Context) {
	q := c.Query("q")
	if q == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, "query parameter 'q' missing")
	}

	courseIdS := c.Query("courseId")
	if courseIdS == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, "query parameter 'courseId' missing")
	}

	courseId, err := strconv.Atoi(courseIdS)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, "can not parse courseId")
	}

	streamIDs, err := r.VideoSectionDao.Search(q, uint(courseId))
	if err != nil {
		log.WithError(err).Error("could not perform video section search")
		c.AbortWithStatus(http.StatusInternalServerError)
	}

	c.JSON(http.StatusOK, searchResponse{streamIDs})
}

func (r searchRoutes) streams(c *gin.Context) {
	q := c.Query("q")
	if q == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, "empty query (?q= missing)")
	}

	courseIdS := c.Query("courseId")
	if courseIdS == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, "query parameter 'courseId' missing")
	}

	courseId, err := strconv.Atoi(courseIdS)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, "can not parse courseId")
	}

	streamIDs, err := dao.Search(q, uint(courseId))
	if err != nil {
		log.WithError(err).Error("could not perform stream search")
		c.AbortWithStatus(http.StatusInternalServerError)
	}

	c.JSON(http.StatusOK, searchResponse{streamIDs})
}
