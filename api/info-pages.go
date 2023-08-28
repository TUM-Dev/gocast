package api

import (
	"github.com/gin-gonic/gin"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
	"net/http"
	"strconv"
)

func configInfoPageRouter(router *gin.Engine, wrapper dao.DaoWrapper) {
	routes := infoPageRoutes{wrapper}
	api := router.Group("/api")
	{
		api.Use(tools.Admin)
		api.PUT("/texts/:id", routes.updateText)
	}
}

type infoPageRoutes struct {
	dao.DaoWrapper
}

type updateTextDao struct {
	Name       string             `json:"name"`
	RawContent string             `json:"content"`
	Type       model.InfoPageType `json:"type"`
}

func (r infoPageRoutes) updateText(c *gin.Context) {
	reqBody := updateTextDao{
		Type: model.INFOPAGE_MARKDOWN, // Use Markdown as default
	}

	if err := c.BindJSON(&reqBody); err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "can not bind body",
			Err:           err,
		})
		return
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "invalid param 'id'",
			Err:           err,
		})
		return
	}

	err = r.InfoPageDao.Update(uint(id), &model.InfoPage{
		Name:       reqBody.Name,
		RawContent: reqBody.RawContent,
		Type:       reqBody.Type,
	})

	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not update info page",
			Err:           err,
		})
		return
	}
}
