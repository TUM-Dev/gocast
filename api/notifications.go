package api

import (
	"github.com/gin-gonic/gin"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools"
	log "github.com/sirupsen/logrus"
	"net/http"
	"strconv"
)

func configNotificationsRouter(r *gin.Engine, daoWrapper dao.DaoWrapper) {
	routes := notificationRoutes{daoWrapper}
	notifications := r.Group("/api/notifications")
	notifications.GET("/", routes.getNotifications)
	notifications.POST("/", routes.createNotification)
	notifications.DELETE("/:id", routes.deleteNotification)
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
		switch ctx.User.Role {
		case model.AdminType:
			targets = append(targets, model.TargetAdmin)
		case model.LecturerType:
			targets = append(targets, model.TargetLecturer)
		case model.StudentType:
			targets = append(targets, model.TargetStudent)
		}
	}
	notifications, err := r.NotificationsDao.GetNotifications(targets...)
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	c.JSON(http.StatusOK, notifications)
}

func (r notificationRoutes) createNotification(c *gin.Context) {
	var notification model.Notification
	if err := c.BindJSON(&notification); err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	if *notification.Title == "" {
		notification.Title = nil
	}
	notification.Body = notification.SanitizedBody // reverse json binding
	if err := r.NotificationsDao.AddNotification(&notification); err != nil {
		log.Error(err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, notification)
}

func (r notificationRoutes) deleteNotification(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "id must be an integer"})
	}
	err = r.NotificationsDao.DeleteNotification(uint(id))
	if err != nil {
		log.WithError(err).Error("Error deleting notification")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}
