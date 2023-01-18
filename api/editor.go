package api

import (
	"encoding/json"
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
		api.Use(tools.InitStream(d))
		api.Use(tools.AdminOfCourse)
		api.POST("/:courseID/:streamID", r.submitEdit)
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

func (r editorRoutes) submitEdit(c *gin.Context) {
	var req submitEditRequest
	err := json.NewDecoder(c.Request.Body).Decode(&req)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "can't decode request",
			Err:           err,
		})
	}
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
		c.Error(tools.RequestError{
			Status:        http.StatusBadGateway,
			CustomMessage: "can't connect to worker",
			Err:           err,
		})
	}
	defer endConnection(clientConn)
	client := pb.NewToWorkerClient(clientConn)
	_, err = client.RequestCut(c, &pb.CutRequest{
		WorkerId:     worker.WorkerID,
		Files:        nil,
		Segments:     nil,
		UploadResult: false,
	})
}

type submitEditRequest struct {
	Start    float64 `json:"start"`
	End      float64 `json:"end"`
	Del      bool    `json:"del"`
	Focussed *bool   `json:"focussed,omitempty"`
}
