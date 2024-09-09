package api

import (
	"context"
	"net/http"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/tools/tum"
	"github.com/gin-gonic/gin"
)

func configSemestersRouter(router *gin.Engine, daoWrapper dao.DaoWrapper) {
	routes := semesterRoutes{daoWrapper}
	router.GET("/api/semesters", routes.getSemesters)
}

type semesterRoutes struct {
	dao.DaoWrapper
}

func (s semesterRoutes) getSemesters(c *gin.Context) {
	includeTestSemester := c.Query("includeTestSemester")

	semesters := s.GetAvailableSemesters(context.Background(), includeTestSemester == "1")
	year, term := tum.GetCurrentSemester()
	c.JSON(http.StatusOK, gin.H{
		"Current": gin.H{
			"Year":         year,
			"TeachingTerm": term,
		},
		"Semesters": semesters,
	})
}
