package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
)

func configGinUsersRouter(router *gin.Engine, daoWrapper dao.DaoWrapper) {
	routes := usersRoutes{daoWrapper}

	admins := router.Group("/api")
	admins.Use(tools.RequirePermission(model.PermManageUsers))
	// Impersonation stays on v1: it creates a session, which the v2 API does not do.
	// The SPA's user administration calls this one.
	admins.POST("/users/impersonate", routes.impersonateUser)
}

type usersRoutes struct {
	dao.DaoWrapper
}

func (r usersRoutes) impersonateUser(c *gin.Context) {
	type req struct {
		UserID uint `json:"id"`
	}
	var request req
	err := c.Bind(&request)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "Bad Request",
			Err:           err,
		})
		return
	}
	u, err := r.UsersDao.GetUserByID(c, request.UserID)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusNotFound,
			CustomMessage: "User not found",
			Err:           err,
		})
		return
	}
	tools.StartSession(c, &tools.SessionData{Userid: u.ID})
}
