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
			CustomMessage: "Can't fatch failed actions",
			Err:           err,
		})
		return
	}
	res := make([]gin.H, len(models))
	c.JSON(http.StatusOK, res)
}
