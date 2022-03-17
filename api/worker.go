package api

import (
	"TUM-Live/dao"
	"TUM-Live/tools"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"net/http"
)

func configWorkerRouter(r *gin.Engine){
	g := r.Group("/api/workers")
	g.Use(tools.Admin)
	g.DELETE("/:id", deleteWorker)
}

func deleteWorker(c *gin.Context){
	id := c.Param("id")
	err := dao.DeleteWorker(id)
	if err != nil {
		log.WithError(err).Error("Worker deletion failed")
		c.AbortWithStatus(http.StatusInternalServerError)
	}
}
