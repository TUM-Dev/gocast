package api

import (
	"net/http"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/gin-gonic/gin"
)

func configWorkerRouter(r *gin.Engine, daoWrapper dao.DaoWrapper) {
	g := r.Group("/api/workers")
	g.Use(tools.AdminOrMaintainer)

	routes := workerRoutes{dao: daoWrapper.WorkerDao}

	g.DELETE("/:id", routes.deleteWorker)
	g.POST("/:id/toggleShared", routes.toggleWorkerShared)
}

type workerRoutes struct {
	dao dao.WorkerDao
}

func (r workerRoutes) deleteWorker(c *gin.Context) {
	id := c.Param("id")

	worker, err := r.dao.GetWorkerByID(c, id)
	if err != nil {
		logger.Error("can not get worker", "err", err)
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not get worker",
			Err:           err,
		})
		return
	}

	u := c.MustGet("TUMLiveContext").(tools.TUMLiveContext).User

	if !u.IsAdminOfSchool(worker.SchoolID) {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusForbidden,
			CustomMessage: "you are not allowed to delete this worker",
		})
		return
	}

	err = r.dao.DeleteWorker(id)
	if err != nil {
		logger.Error("can not delete worker", "err", err)
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not delete worker",
			Err:           err,
		})
		return
	}
}

func (r workerRoutes) toggleWorkerShared(c *gin.Context) {
	id := c.Param("id")

	worker, err := r.dao.GetWorkerByID(c, id)
	if err != nil {
		logger.Error("can not get worker", "err", err)
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not get worker",
			Err:           err,
		})
		return
	}

	u := c.MustGet("TUMLiveContext").(tools.TUMLiveContext).User

	if !u.IsAdminOfSchool(worker.SchoolID) {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusForbidden,
			CustomMessage: "you are not allowed to update this worker",
		})
		return
	}

	worker.Shared = !worker.Shared
	err = r.dao.SaveWorker(worker)
	if err != nil {
		logger.Error("can not update worker", "err", err)
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not update worker",
			Err:           err,
		})
		return
	}
}
