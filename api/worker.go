package api

import (
	"TUM-Live/dao"
	"context"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"
)

func configGinWorkerRouter(r gin.IRoutes) {
	r.GET("/api/worker/getJobs/:workerID", getJob)
	r.POST("/api/worker/putVOD/:workerID", putVod)
}

func putVod(c *gin.Context) {
	_, err := dao.GetWorkerByID(context.Background(), c.Param("workerID"))
	if err != nil {
		c.JSON(http.StatusForbidden, "forbidden")
		return
	}
	jsondata, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	var req putVodData
	err = json.Unmarshal(jsondata, &req)
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	stream, _ := dao.GetStreamByID(context.Background(), strconv.Itoa(int(req.StreamId)))
	stream.Recording = true
	stream.PlaylistUrl = req.HlsUrl
	_ = dao.SaveStream(&stream)
}

func getJob(c *gin.Context) {
	_, err := dao.GetWorkerByID(context.Background(), c.Param("workerID"))
	if err != nil {
		c.JSON(http.StatusForbidden, "forbidden")
		return
	}
	job, err := dao.PickJob(context.Background())
	if err != nil {
		c.JSON(http.StatusNotFound, &jobData{})
		return
	}
	stream, _ := dao.GetStreamByID(context.Background(), strconv.Itoa(int(job.StreamID)))
	c.JSON(http.StatusOK, &jobData{
		Id:          job.ID,
		Name:        stream.Name,
		StreamStart: stream.Start,
		StreamId:    job.StreamID,
		Path:        job.FilePath,
	})
}

type jobData struct {
	Id          uint      `json:"id"`
	Name        string    `json:"name"`
	StreamId    uint      `json:"streamId"`
	StreamStart time.Time `json:"streamStart"`
	Path        string    `json:"path"`
}

type putVodData struct {
	Name     string
	Start    time.Time
	HlsUrl   string
	StreamId uint
}
