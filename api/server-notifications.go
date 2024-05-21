package api

import (
	"fmt"
	"github.com/TUM-Dev/gocast/tools/oauth"
	"net/http"
	"time"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/gin-gonic/gin"
)

func configServerNotificationsRoutes(engine *gin.Engine, daoWrapper dao.DaoWrapper) {
	routes := serverNotificationRoutes{daoWrapper}
	adminGroup := engine.Group("/api/serverNotification")
	adminGroup.Use(oauth.Admin)
	adminGroup.POST("/:notificationId", routes.updateServerNotification)
	adminGroup.POST("/create", routes.createServerNotification)
}

type serverNotificationRoutes struct {
	dao.DaoWrapper
}

func (r serverNotificationRoutes) updateServerNotification(c *gin.Context) {
	var req notificationReq
	if err := c.ShouldBind(&req); err != nil {
		logger.Debug(fmt.Sprintf("%v", err))
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
	err := r.ServerNotificationDao.UpdateServerNotification(notification, req.Id)
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
		logger.Debug(fmt.Sprintf("%v", err))
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
	err := r.ServerNotificationDao.CreateServerNotification(notification)
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
