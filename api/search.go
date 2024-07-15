package api

import (
	"context"
	"github.com/TUM-Dev/gocast/model"
	"net/http"
	"strconv"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/gin-gonic/gin"
)

func configGinSearchRouter(router *gin.Engine, daoWrapper dao.DaoWrapper) {
	routes := searchRoutes{daoWrapper}

	searchGroup := router.Group("/api/search")
	withStream := searchGroup.Group("/stream/:streamID")
	withStream.Use(tools.InitStream(daoWrapper))
	withStream.GET("/subtitles", routes.searchSubtitles)

	searchGroup.GET("/courses", routes.searchCourses)
}

type searchRoutes struct {
	dao.DaoWrapper
}

func (r searchRoutes) searchSubtitles(c *gin.Context) {
	s := c.MustGet("TUMLiveContext").(tools.TUMLiveContext).Stream
	q := c.Query("q")
	c.JSON(http.StatusOK, tools.SearchSubtitles(q, s.ID))
}

func (r searchRoutes) searchCourses(c *gin.Context) {
	user := c.MustGet("TUMLiveContext").(tools.TUMLiveContext).User
	q := c.Query("q")
	t := c.Query("term")
	y, err := strconv.ParseInt(c.Query("year"), 10, 64)
	if err != nil {
		return
	}

	var courses []model.Course
	if user != nil {
		switch user.Role {
		case model.AdminType:
			courses = r.GetAllCoursesForSemester(int(y), t, c)
		default: // user.CoursesForSemesters includes both Administered Courses and enrolled Courses
			courses, _ = r.CoursesDao.GetPublicAndLoggedInCourses(int(y), t)
			courses = append(courses, user.CoursesForSemester(int(y), t, context.Background())...)
		}
	} else {
		courses, _ = r.GetPublicCourses(int(y), t)
	}

	distinctCourseIDs := make(map[uint]bool)
	var courseIDs []uint
	for _, course := range courses {
		value := distinctCourseIDs[course.ID]
		if !value {
			courseIDs = append(courseIDs, course.ID)
			distinctCourseIDs[course.ID] = true
		}
	}

	c.JSON(http.StatusOK, tools.SearchCourses(q, int(y), t, &courseIDs))
}
