package api

import (
	"TUM-Live/dao"
	"TUM-Live/model"
	"TUM-Live/tools"
	"github.com/gin-gonic/gin"
	uuid "github.com/satori/go.uuid"
	log "github.com/sirupsen/logrus"
	"net/http"
	"strings"
)

func configWorkerRouter(r *gin.Engine) {
	api := r.Group("/api/worker")
	api.Use(tools.Admin)
	api.POST("/", createWorker)
}

type createWorkerReq struct {
	Hostname string `json:"hostname"`
}

func createWorker(c *gin.Context) {
	var req createWorkerReq
	err := c.BindJSON(&req)
	if err != nil {
		return
	}
	id := strings.ReplaceAll(uuid.NewV4().String(), "-", "")
	w := model.Worker{
		WorkerID: id,
		Host:     req.Hostname,
	}
	err = dao.CreateWorker(&w)
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		log.WithError(err).Error("can't save worker")
		return
	}
	c.JSON(http.StatusOK, gin.H{"workerID": id})
}
