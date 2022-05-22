package api

import (
	"github.com/gin-gonic/gin"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools"
	log "github.com/sirupsen/logrus"
	"net/http"
	"strconv"
)

func configGinKeywordRouter(router *gin.Engine, daoWrapper dao.DaoWrapper) {
	routes := keywordRoutes{daoWrapper}
	router.Use(tools.Admin) // Todo: Can workers post as admin?
	router.POST("/api/:streamID/keywords", routes.PostKeywords)
}

type keywordRoutes struct {
	dao.DaoWrapper
}

type postKeywordRequest struct {
	Keywords []string `json:"keywords"`
	Language string   `json:"language"`
}

func (r keywordRoutes) PostKeywords(c *gin.Context) {
	streamIdStr := c.Param("streamID")
	if len(streamIdStr) == 0 {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	streamId, err := strconv.Atoi(streamIdStr)

	var req postKeywordRequest
	err = c.ShouldBind(req)
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	for _, text := range req.Keywords {
		err := r.KeywordDao.NewKeyword(&model.Keyword{
			StreamID: uint(streamId),
			Text:     text,
			Language: req.Language,
		})

		if err != nil {
			log.WithError(err).Println("Could not insert keyword")
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
	}
}
