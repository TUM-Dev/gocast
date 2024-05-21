package api

import (
	"github.com/TUM-Dev/gocast/tools/oauth"
	"net/http"
	"slices"
	"strconv"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/gin-gonic/gin"
)

func configNotificationsRouter(r *gin.Engine, daoWrapper dao.DaoWrapper) {
	routes := notificationRoutes{daoWrapper}

	notifications := r.Group("/api/notifications")
	{
		notifications.GET("/", routes.getNotifications)
		notifications.GET("/server", routes.getServerNotifications)
		notifications.POST("/", oauth.Admin, routes.createNotification)
		notifications.DELETE("/:id", oauth.Admin, routes.deleteNotification)
	}
}

type notificationRoutes struct {
	dao.DaoWrapper
}

func (r notificationRoutes) getNotifications(c *gin.Context) {
	f, _ := c.Get("TUMLiveContext")
	ctx := f.(tools.TUMLiveContext)
	targets := []model.NotificationTarget{model.TargetAll}
	if ctx.User != nil {
		targets = append(targets, model.TargetUser)
		groups := oauth.GetGroups(c)
		if slices.Contains(groups, "/admin") {
			targets = append(targets, model.TargetAdmin)
		} else if slices.Contains(groups, "/lecturer") {
			targets = append(targets, model.TargetLecturer)
		} else if slices.Contains(groups, "/student") {
			targets = append(targets, model.TargetStudent)
		}
	}
	notifications, err := r.NotificationsDao.GetNotifications(targets...)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusNotFound,
			CustomMessage: "can not get notifications",
			Err:           err,
		})
		return
	}
	c.JSON(http.StatusOK, notifications)
}

func (r notificationRoutes) getServerNotifications(c *gin.Context) {
	notifications, err := r.ServerNotificationDao.GetCurrentServerNotifications()
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not bind body",
			Err:           err,
		})
		return
	}
	c.JSON(http.StatusOK, notifications)
}

func (r notificationRoutes) createNotification(c *gin.Context) {
	var notification model.Notification
	if err := c.BindJSON(&notification); err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "can not bind body",
			Err:           err,
		})
		return
	}
	if *notification.Title == "" {
		notification.Title = nil
	}
	notification.Body = notification.SanitizedBody // reverse json binding
	if err := r.NotificationsDao.AddNotification(&notification); err != nil {
		logger.Error("Error adding notification", "err", err)
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not add notification",
			Err:           err,
		})
		return
	}
	c.JSON(http.StatusOK, notification)
}

func (r notificationRoutes) deleteNotification(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "invalid param 'id'",
			Err:           err,
		})
		return
	}
	err = r.NotificationsDao.DeleteNotification(uint(id))
	if err != nil {
		logger.Error("error deleting notification", "err", err)
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "error deleting notification",
			Err:           err,
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}
