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

func configServerNotificationsRoutes(engine *gin.Engine, daoWrapper dao.DaoWrapper) {
	routes := serverNotificationRoutes{daoWrapper}
	adminGroup := engine.Group("/api/serverNotification")
	adminGroup.Use(tools.Admin)
	adminGroup.POST("/:notificationId", routes.updateServerNotification)
	adminGroup.POST("/create", routes.createServerNotification)
}

type serverNotificationRoutes struct {
	dao.DaoWrapper
}

func (r serverNotificationRoutes) updateServerNotification(c *gin.Context) {
	var req notificationReq
	if err := c.ShouldBind(&req); err != nil {
		log.Printf("%v", err)
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "can not bind body",
			Err:           err,
		})
		return
	}
	notification := model.ServerNotification{
		Text:    req.Text,
		Warn:    req.Type == "warning",
		Start:   req.From,
		Expires: req.Expires,
	}
	err := r.ServerNotificationDao.UpdateServerNotification(c, notification, req.Id)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not update server notification",
			Err:           err,
		})
		return
	}
	c.Redirect(http.StatusFound, "/admin/server-notifications")
}

func (r serverNotificationRoutes) createServerNotification(c *gin.Context) {
	var req notificationReq
	if err := c.ShouldBind(&req); err != nil {
		log.Printf("%v", err)
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "can not bind body",
			Err:           err,
		})
		return
	}
	notification := model.ServerNotification{
		Text:    req.Text,
		Warn:    req.Type == "warn",
		Start:   req.From,
		Expires: req.Expires,
	}
	err := r.ServerNotificationDao.CreateServerNotification(c, notification)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not create server notification",
			Err:           err,
		})
		return
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
