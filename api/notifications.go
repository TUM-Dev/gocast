package api

import (
	"TUM-Live/dao"
	"TUM-Live/model"
	"TUM-Live/tools"
	"github.com/gin-gonic/gin"
	"net/http"
)

func configNotificationsRouter(r *gin.Engine) {
	notifications := r.Group("/api/notifications")
	notifications.GET("/", getNotifications)
	notifications.POST("/", createNotification)
	notifications.PUT("/:id", updateNotification)
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

}

func updateNotification(c *gin.Context) {

}

func deleteNotification(c *gin.Context) {

}
