package api

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/tools/tum"
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
	semesters := s.GetAvailableSemesters(context.Background())
	year, term := tum.GetCurrentSemester()
	c.JSON(http.StatusOK, gin.H{
		"Current": gin.H{
			"Year":         year,
			"TeachingTerm": term,
		},
		"Semesters": semesters,
	})
}
