package api

import (
	"TUM-Live/dao"
	"TUM-Live/model"
	"TUM-Live/tools"
	"context"
	"encoding/json"
	"fmt"
	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	"io/ioutil"
	"net/http"
	"strconv"
)

func configGinLectureHallApiRouter(router gin.IRoutes) {
	router.POST("/api/createLectureHall", createLectureHall)
	router.POST("/api/updateLecturesLectureHall", updateLecturesLectureHall)
	router.POST("/api/takeSnapshot/:lectureHallID/:presetID", takeSnapshot)
}

func takeSnapshot(c *gin.Context) {
	if user, err := tools.GetUser(c); err != nil || user.Role != model.AdminType {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}
	preset, err := dao.FindPreset(c.Param("lectureHallID"), c.Param("presetID"))
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		sentry.CaptureException(err)
	}
	tools.TakeSnapshot(preset)
	preset, err = dao.FindPreset(c.Param("lectureHallID"), c.Param("presetID"))
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		sentry.CaptureException(err)
	}
	c.JSONP(http.StatusOK, gin.H{"path": fmt.Sprintf("/public/%s", preset.Image)})
}

func updateLecturesLectureHall(c *gin.Context) {
	if user, err := tools.GetUser(c); err == nil && user.Role == model.AdminType {
		body, err := ioutil.ReadAll(c.Request.Body)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"msg": "Bad request"})
			return
		}
		var req updateLecturesLectureHallRequest

		if err = json.Unmarshal(body, &req); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"msg": "Bad request"})
			return
		}
		lecture, err := dao.GetStreamByID(context.Background(), strconv.Itoa(int(req.LectureID)))
		if err != nil {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		lectureHall, err := dao.GetLectureHallByID(req.LectureHallID)
		if err != nil {
			dao.UnsetLectureHall(lecture.Model.ID)
			return
		} else {
			lectureHall.Streams = append(lectureHall.Streams, lecture)
			dao.SaveLectureHall(lectureHall)
		}
	} else {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"msg": "Forbidden"})
	}
}

func createLectureHall(c *gin.Context) {
	if user, err := tools.GetUser(c); err == nil && user.Role == model.AdminType {
		body, err := ioutil.ReadAll(c.Request.Body)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"msg": "Bad request"})
			return
		}
		var req createLectureHallRequest
		if err = json.Unmarshal(body, &req); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"msg": "Bad request"})
			return
		}
		dao.CreateLectureHall(model.LectureHall{
			Name:   req.Name,
			CombIP: req.CombIP,
			PresIP: req.PresIP,
			CamIP:  req.CamIP,
		})
	} else {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"msg": "Forbidden"})
	}
}

type createLectureHallRequest struct {
	Name   string `json:"name"`
	CombIP string `json:"combIP"`
	PresIP string `json:"presIP"`
	CamIP  string `json:"camIP"`
}

type updateLecturesLectureHallRequest struct {
	LectureID     uint `json:"lecture"`
	LectureHallID uint `json:"lectureHall"`
}
