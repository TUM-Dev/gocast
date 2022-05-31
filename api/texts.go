package api

import (
	"github.com/gin-gonic/gin"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools"
	"net/http"
	"strconv"
)

func configTextRouter(router *gin.Engine, wrapper dao.DaoWrapper) {
	routes := markdownTextRoutes{wrapper}
	api := router.Group("/api")
	{
		api.Use(tools.Admin)
		api.PUT("/texts/:id", routes.updateText)
	}
}

type markdownTextRoutes struct {
	dao.DaoWrapper
}

type updateTextDao struct {
	Name       string `json:"name"`
	RawContent string `json:"content"`
	Type       uint   `json:"type"`
}

func (r markdownTextRoutes) updateText(c *gin.Context) {
	reqBody := updateTextDao{
		Type: model.TEXT_MARKDOWN, // Use Markdown as default
	}

	if err := c.BindJSON(&reqBody); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err = r.TextDao.Update(uint(id), &model.Text{
		Name:       reqBody.Name,
		RawContent: reqBody.RawContent,
		Type:       reqBody.Type,
	})

	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
}
