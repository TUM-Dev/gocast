package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
)

// Creating and deleting notifications moved to AdminService in apiv2 (see
// apiv2/server/notification_admin.go) with the rest of the /admin/notifications page.
// getNotifications stays here: it backs the legacy header's notification bell, which
// every not-yet-migrated page still renders. getServerNotifications stays for the same
// reason, serving the /admin/server-notifications page, which is not this page's to
// migrate.
func configNotificationsRouter(r *gin.Engine, daoWrapper dao.DaoWrapper) {
	routes := notificationRoutes{daoWrapper}

	notifications := r.Group("/api/notifications")
	{
		notifications.GET("/", routes.getNotifications)
		notifications.GET("/server", routes.getServerNotifications)
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
