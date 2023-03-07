package api

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/joschahenningsen/TUM-Live/dao"
	"net/http"
)

func configSemestersRouter(router *gin.Engine, daoWrapper dao.DaoWrapper) {
	routes := semesterRoutes{daoWrapper}
	router.GET("/api/semesters", routes.getSemesters)
}

type semesterRoutes struct {
	dao.DaoWrapper
}

func (s semesterRoutes) getSemesters(c *gin.Context) {
	c.JSON(http.StatusOK, s.GetAvailableSemesters(context.Background()))
}
