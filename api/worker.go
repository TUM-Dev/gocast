package api

import (
	"github.com/gin-gonic/gin"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/tools"
	log "github.com/sirupsen/logrus"
	"net/http"
)

func configWorkerRouter(r *gin.Engine, daoWrapper dao.DaoWrapper) {
	g := r.Group("/api/workers")
	g.Use(tools.Admin)

	routes := workerRoutes{dao: daoWrapper.WorkerDao}

	g.DELETE("/:id", routes.deleteWorker)
}

type workerRoutes struct {
	dao dao.WorkerDao
}

func (r workerRoutes) deleteWorker(c *gin.Context) {
	id := c.Param("id")
	err := r.dao.DeleteWorker(id)
	if err != nil {
		log.WithError(err).Error("can not delete worker")
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not delete worker",
			Err:           err,
		})
		return
	}
}
