package api

import (
	"context"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"net/http"
)

func configRunnerRouter(r *gin.Engine, daoWrapper dao.DaoWrapper) {
	g := r.Group("/api/runner")
	g.Use(tools.Admin)

	routes := runnerRoutes{dao: daoWrapper.RunnerDao}

	g.DELETE("/:HostName", routes.deleteRunner)
}

type runnerRoutes struct {
	dao dao.RunnerDao
}

func (r runnerRoutes) deleteRunner(c *gin.Context) {
	log.Info("delete runner with hostname: ", c.Param("Hostname"))
	ctx := context.Background()
	err := r.dao.Delete(ctx, c.Param("Hostname"))
	if err != nil {
		//logging for later
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not delete runner",
			Err:           err,
		})
		return
	}

}
