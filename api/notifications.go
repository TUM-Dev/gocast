package api

import (
	"TUM-Live/dao"
	"TUM-Live/model"
	"TUM-Live/tools"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"net/http"
	"strconv"
)

func configNotificationsRouter(r *gin.Engine) {
	notifications := r.Group("/api/notifications")
	notifications.GET("/", getNotifications)
	notifications.POST("/", createNotification)
	notifications.DELETE("/:id", deleteNotification)
}

func getNotifications(c *gin.Context) {
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
	notifications, err := dao.GetNotifications(targets...)
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	c.JSON(http.StatusOK, notifications)
}

func createNotification(c *gin.Context) {
	var notification model.Notification
	if err := c.BindJSON(&notification); err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	if *notification.Title == "" {
		notification.Title = nil
	}
	notification.Body = notification.SanitizedBody // reverse json binding
	if err := dao.AddNotification(&notification); err != nil {
		log.Error(err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, notification)
}

func deleteNotification(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "id must be an integer"})
	}
	err = dao.DeleteNotification(uint(id))
	if err != nil {
		log.WithError(err).Error("Error deleting notification")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}
