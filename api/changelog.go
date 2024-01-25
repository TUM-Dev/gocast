package api

import (
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/gin-gonic/gin"
	"net/http"
)

func configGinChangelogRouter(router *gin.Engine, daoWrapper dao.DaoWrapper) {
	routes := changelogRoutes{daoWrapper}
	router.GET("/api/changelog", routes.changelog)
}

type changelogRoutes struct {
	dao.DaoWrapper
}

var clError = tools.RequestError{
	Status:        http.StatusNotFound,
	CustomMessage: "No valid version string",
}

func (r changelogRoutes) changelog(c *gin.Context) {
	version := c.Param("version")
	if version == "" {
		_ = c.Error(clError)
		return
	}
	// TODO: Fetch from database and send it back
}
