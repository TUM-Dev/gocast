package api

import (
	"github.com/gin-gonic/gin"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/tools"
	"github.com/microcosm-cc/bluemonday"
	"github.com/russross/blackfriday/v2"
	"net/http"
)

type markdownRoutes struct {
	dao.DaoWrapper
}

func configGinMarkdownRouter(r *gin.Engine, daos dao.DaoWrapper) {
	routes := markdownRoutes{
		DaoWrapper: daos,
	}
	r.POST("/api/markdown", routes.getMarkdown)
}

type markdownRequest struct {
	Markdown string `json:"markdown"`
}

func (r markdownRoutes) getMarkdown(c *gin.Context) {
	var err error
	var req markdownRequest
	err = c.BindJSON(&req)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "can not bind body",
			Err:           err,
		})
		return
	}
	unsafe := blackfriday.Run([]byte(req.Markdown), blackfriday.WithExtensions(blackfriday.CommonExtensions|blackfriday.HardLineBreak))
	html := bluemonday.
		UGCPolicy().
		AddTargetBlankToFullyQualifiedLinks(true).
		AllowRelativeURLs(false).
		SanitizeBytes(unsafe)
	c.JSON(http.StatusOK, gin.H{"html": string(html)})
}
