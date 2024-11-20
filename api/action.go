package api

import (
	"context"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"net/http"
)

func configActionRouter(r *gin.Engine, wrapper dao.DaoWrapper) {
	g := r.Group("/api/Actions")
	g.Use(tools.Admin)

	routes := actionRoutes{dao: wrapper.ActionDao}

	g.GET("/failed", routes.getFailedActions)
	g.GET("/:id", routes.getActionById)
}

type actionRoutes struct {
	dao dao.ActionDao
}

func (a actionRoutes) getFailedActions(c *gin.Context) {
	log.Info("Getting failed actions")
	ctx := context.Background()
	models, err := a.dao.GetAllFailedActions(ctx)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "Can't fetch failed actions",
			Err:           err,
		})
		return
	}
	res := make([]gin.H, len(models))
	c.JSON(http.StatusOK, res)
}

func (a actionRoutes) getActionById(c *gin.Context) {
	ctx := context.Background()
	model, err := a.dao.GetActionByID(ctx, c.Param("id"))
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusNotFound,
			CustomMessage: "Action not found",
			Err:           err,
		})
		return
	}
	c.JSON(http.StatusOK, model)
}
