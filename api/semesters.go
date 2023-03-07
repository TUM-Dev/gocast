package api

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/tools/tum"
	"net/http"
)

func configSemestersRouter(router *gin.Engine, daoWrapper dao.DaoWrapper) {
	routes := semesterRoutes{daoWrapper}
	router.GET("/api/semesters", routes.getSemesters)
	router.GET("/api/semesters/current", routes.getCurrentSemester)
}

type semesterRoutes struct {
	dao.DaoWrapper
}

func (s semesterRoutes) getSemesters(c *gin.Context) {
	c.JSON(http.StatusOK, s.GetAvailableSemesters(context.Background()))
}

func (s semesterRoutes) getCurrentSemester(c *gin.Context) {
	year, term := tum.GetCurrentSemester()
	c.JSON(http.StatusOK, gin.H{
		"Year":         year,
		"TeachingTerm": term,
	})
}
