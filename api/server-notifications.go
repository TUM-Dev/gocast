package api

import (
	"github.com/gin-gonic/gin"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools"
	"log"
	"net/http"
	"time"
)

func configServerNotificationsRoutes(engine *gin.Engine) {
	adminGroup := engine.Group("/api/serverNotification")
	adminGroup.Use(tools.Admin)
	adminGroup.POST("/:notificationId", updateServerNotification)
	adminGroup.POST("/create", createServerNotification)
}

func updateServerNotification(c *gin.Context) {
	var req notificationReq
	if err := c.ShouldBind(&req); err != nil {
		log.Printf("%v", err)
		c.AbortWithStatus(http.StatusBadRequest)
	}
	notification := model.ServerNotification{
		Text:    req.Text,
		Warn:    req.Type == "warning",
		Start:   req.From,
		Expires: req.Expires,
	}
	err := dao.UpdateServerNotification(notification, req.Id)
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
	}
	c.Redirect(http.StatusFound, "/admin/server-notifications")
}

func createServerNotification(c *gin.Context) {
	var req notificationReq
	if err := c.ShouldBind(&req); err != nil {
		log.Printf("%v", err)
		c.AbortWithStatus(http.StatusBadRequest)
	}
	notification := model.ServerNotification{
		Text:    req.Text,
		Warn:    req.Type == "warn",
		Start:   req.From,
		Expires: req.Expires,
	}
	err := dao.CreateServerNotification(notification)
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
	}
	c.Redirect(http.StatusFound, "/admin/server-notifications")
}

type notificationReq struct {
	Text    string    `form:"text"`
	From    time.Time `form:"from" time_format:"2006-01-02 15:04"`
	Expires time.Time `form:"expires" time_format:"2006-01-02 15:04"`
	Type    string    `form:"type"`
	Id      string    `form:"id"`
}
