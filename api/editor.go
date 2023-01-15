package api

import (
	"github.com/gin-gonic/gin"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/tools"
	"github.com/joschahenningsen/TUM-Live/worker/pb"
	"net/http"
)

type editorRoutes struct {
	d dao.DaoWrapper
}

func configEditorRouter(e *gin.Engine, d dao.DaoWrapper) {
	r := editorRoutes{d}
	api := e.Group("/api/editor")
	{
		api.GET("/waveform", r.getWaveform)
	}
}

func (r editorRoutes) getWaveform(c *gin.Context) {
	workers := r.d.WorkerDao.GetAliveWorkers()
	if len(workers) == 0 {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusServiceUnavailable,
			CustomMessage: "no workers available",
		})
		return
	}
	worker := workers[getWorkerWithLeastWorkload(workers)]
	clientConn, err := dialIn(worker)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadGateway, err.Error())
		return
	}
	defer endConnection(clientConn)
	client := pb.NewToWorkerClient(clientConn)
	waveform, err := client.RequestWaveform(c, &pb.WaveformRequest{
		WorkerId: worker.WorkerID,
		File:     c.Query("video"),
	})
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, err.Error())
		return
	}
	c.Data(http.StatusOK, "image/png", waveform.Waveform)
}
