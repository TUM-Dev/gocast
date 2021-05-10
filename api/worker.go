package api

import (
	"TUM-Live/dao"
	"context"
	"encoding/json"
	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"time"
)

func configGinWorkerRouter(r gin.IRoutes) {
	r.GET("/api/worker/getJobs/:workerID", getJob)
	r.POST("/api/worker/putVOD/:workerID", putVod)
	r.POST("/api/worker/notifyLive/:workerID", notifyLive)
	r.POST("/api/worker/notifyLiveEnd/:workerID/:streamID", notifyLiveEnd)
}

func notifyLiveEnd(c *gin.Context) {
	_, err := dao.GetWorkerByID(context.Background(), c.Param("workerID"))
	if err != nil {
		c.JSON(http.StatusForbidden, "forbidden")
		return
	}
	if sid := c.Param("streamID"); sid != "" {
		err := dao.SetStreamNotLiveById(sid)
		if err != nil {
			log.Printf("Couldn't set stream not live: %v\n", err)
			sentry.CaptureException(err)
			return
		}
		return

	}
	c.JSON(http.StatusNotFound, "forbidden")
	return

}
func notifyLive(c *gin.Context) {
	_, err := dao.GetWorkerByID(context.Background(), c.Param("workerID"))
	if err != nil {
		c.JSON(http.StatusForbidden, "forbidden")
		return
	}
	body, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	var req notifyLiveRequest
	err = json.Unmarshal(body, &req)
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	stream, err := dao.GetStreamByID(context.Background(), req.StreamID)
	if err != nil {
		sentry.CaptureException(err)
		return
	}
	stream.LiveNow = true
	switch req.Version {
	case "COMB":
		stream.PlaylistUrl = req.URL
	case "PRES":
		stream.PlaylistUrlPRES = req.URL
	case "CAM":
		stream.PlaylistUrlCAM = req.URL
	}
	err = dao.SaveStream(&stream)
	if err != nil {
		sentry.CaptureException(err)
	}
}

type notifyLiveRequest struct {
	StreamID string `json:"streamID"`
	URL      string `json:"url"`     // eg. https://live.lrz.de/livetum/stream/playlist.m3u8
	Version  string `json:"version"` //eg. COMB
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
	stream.FilePath = req.FilePath
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
	course, _ := dao.GetCourseById(context.Background(), stream.CourseID)
	c.JSON(http.StatusOK, &jobData{
		Id:          job.ID,
		Name:        stream.Name,
		StreamStart: stream.Start,
		StreamId:    job.StreamID,
		Path:        job.FilePath,
		Upload:      course.VODEnabled,
	})
}

type jobData struct {
	Id          uint      `json:"id"`
	Name        string    `json:"name"`
	StreamId    uint      `json:"streamId"`
	StreamStart time.Time `json:"streamStart"`
	Path        string    `json:"path"`
	Upload      bool
}

type putVodData struct {
	Name     string
	Start    time.Time
	HlsUrl   string
	FilePath string
	StreamId uint
}
